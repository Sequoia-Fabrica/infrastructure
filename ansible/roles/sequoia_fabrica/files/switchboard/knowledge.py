"""What the space knows about itself: Home Assistant (live state), the wiki (documents) and its calendars (events)."""
import asyncio
import datetime
import html
import logging
import re
import time
import zoneinfo

import aiohttp
import icalendar
import recurring_ical_events

log = logging.getLogger('knowledge')
TZ = zoneinfo.ZoneInfo('America/Los_Angeles')


class HomeAssistant:
    """Read-only. Nothing a caller says can switch anything on in the shop."""

    def __init__(self, session, url, token):
        self.session, self.url, self.token = session, url.rstrip('/'), token
        self._states, self._fetched = [], 0.0

    async def states(self):
        if time.monotonic() - self._fetched > 10:
            async with self.session.get(f'{self.url}/api/states', headers={'Authorization': f'Bearer {self.token}'},
                                        timeout=aiohttp.ClientTimeout(total=5)) as resp:
                resp.raise_for_status()
                self._states, self._fetched = await resp.json(), time.monotonic()
        return self._states

    @staticmethod
    def describe(s):
        attrs = s.get('attributes', {})
        unit = attrs.get('unit_of_measurement', '')
        name = attrs.get('friendly_name', s['entity_id'])
        return f"{s['entity_id']} ({name}): {s['state']}{' ' + unit if unit else ''}"

    async def search(self, query='', domain='', allowed=None):
        """Entities matching every word of `query`; `allowed` (a set of entity ids) narrows what may be seen at all."""
        words = [w for w in re.split(r'\W+', query.lower()) if w]
        hits = []
        for s in await self.states():
            if allowed is not None and s['entity_id'] not in allowed:
                continue
            if domain and not s['entity_id'].startswith(domain + '.'):
                continue
            if s['state'] in ('unavailable', 'unknown'):
                continue
            hay = (s['entity_id'] + ' ' + s.get('attributes', {}).get('friendly_name', '')).lower()
            if all(w in hay for w in words):
                hits.append(self.describe(s))
        return hits

    async def get(self, entity_id):
        for s in await self.states():
            if s['entity_id'] == entity_id:
                return s
        return None


class Wiki:
    def __init__(self, session, api_url):
        self.session, self.api = session, api_url

    async def _query(self, **params):
        params.update(format='json', formatversion='2')
        async with self.session.get(self.api, params=params, timeout=aiohttp.ClientTimeout(total=8)) as resp:
            resp.raise_for_status()
            return await resp.json()

    async def search(self, query, limit=5):
        data = await self._query(action='query', list='search', srsearch=query, srlimit=str(limit))
        return [{'title': r['title'], 'snippet': html.unescape(re.sub(r'<[^>]+>', '', r['snippet']))}
                for r in data['query']['search']]

    async def titles(self):
        data = await self._query(action='query', list='allpages', aplimit='500')
        return [p['title'] for p in data['query']['allpages']]

    async def page(self, title, max_chars=20000):
        data = await self._query(action='query', prop='revisions', rvprop='content', rvslots='main',
                                 titles=title, redirects='1')
        page = data['query']['pages'][0]
        if page.get('missing'):
            return None
        text = page['revisions'][0]['slots']['main']['content']
        text = re.sub(r'\[\[File:[^\]]*\]\]', '', text)
        text = re.sub(r'\[\[(?:[^|\]]*\|)?([^\]]*)\]\]', r'\1', text)  # [[Page|label]] -> label
        text = re.sub(r'\[https?://\S+ ([^\]]*)\]', r'\1', text)        # [url label] -> label
        text = re.sub(r'\n{3,}', '\n\n', text)
        return text[:max_chars]


class Calendars:
    """Upcoming events from the public iCal feeds the website already uses (Bookwhen classes, the Google calendar)."""

    def __init__(self, session, urls):
        self.session, self.urls = session, urls
        self._cache, self._fetched = [], 0.0

    async def _fetch(self, url):
        async with self.session.get(url, timeout=aiohttp.ClientTimeout(total=10)) as resp:
            resp.raise_for_status()
            return icalendar.Calendar.from_ical(await resp.read())

    async def calendars(self):
        if time.monotonic() - self._fetched > 15 * 60:
            results = await asyncio.gather(*(self._fetch(u) for u in self.urls), return_exceptions=True)
            for url, r in zip(self.urls, results):
                if isinstance(r, Exception):
                    log.warning('calendar %s: %s', url, r)
            self._cache, self._fetched = [r for r in results if not isinstance(r, Exception)], time.monotonic()
        return self._cache

    async def upcoming(self, days=14):
        start = datetime.datetime.now(TZ)
        end = start + datetime.timedelta(days=days)
        seen, events = set(), []
        for cal in await self.calendars():
            for ev in recurring_ical_events.of(cal).between(start, end):
                when = ev.get('DTSTART').dt
                if not isinstance(when, datetime.datetime):  # all-day
                    when = datetime.datetime.combine(when, datetime.time(), TZ)
                when = when.astimezone(TZ)
                title = str(ev.get('SUMMARY', 'Untitled')).strip()
                if (title, when) not in seen:  # an event can be on both calendars
                    seen.add((title, when))
                    events.append((when, title))
        return [f'{when:%A %B %-d, %-I:%M %p}: {title}' for when, title in sorted(events)]
