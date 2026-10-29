"""Things you can reach on the red phones. Each app is an async function (call, ctx) that owns the call until it returns.

To add a gag: write a function here, add it to APPS, and give it an extension in extensions.conf.
"""
import asyncio
import collections
import datetime
import json
import logging
import os
import re

import anthropic

from knowledge import TZ
from voice import make_stt, say

log = logging.getLogger('apps')

PERSONA = open(os.path.join(os.path.dirname(__file__), 'persona.md')).read()
MAX_CALL_SECONDS = 15 * 60
IDLE_SECONDS = 40
SENTENCE_END = re.compile(r'(?<=[.!?…])\s+')

TOOLS = [
    {
        'name': 'home_assistant',
        'description': "Read (never change) the space's live sensors and devices from Home Assistant: temperature, "
                       'CO2, air quality, the front roll-up gate, and on red-phone calls also doors, motion and the '
                       "smart plugs on the 3D printers and laser. Returns matching entities with their current state. "
                       'Leave both fields empty to list everything you can see.',
        'input_schema': {
            'type': 'object',
            'properties': {
                'query': {'type': 'string', 'description': 'Words that must all appear in the entity id or name, '
                                                           "e.g. 'gate', 'co2', 'printer'."},
                'domain': {'type': 'string', 'description': "Optional Home Assistant domain, e.g. 'sensor', "
                                                            "'binary_sensor', 'switch'."},
            },
            'required': [],
        },
    },
    {
        'name': 'wiki_search',
        'description': "Search the Sequoia Fabrica wiki (policies, equipment, membership, groves, how-tos, history). "
                       'Returns page titles and snippets; read a page with wiki_page for detail.',
        'input_schema': {
            'type': 'object',
            'properties': {'query': {'type': 'string'}},
            'required': ['query'],
        },
    },
    {
        'name': 'wiki_page',
        'description': 'Read one wiki page by its exact title.',
        'input_schema': {
            'type': 'object',
            'properties': {'title': {'type': 'string'}},
            'required': ['title'],
        },
    },
    {
        'name': 'upcoming_events',
        'description': "Classes, workshops, open hours and community events coming up, from the space's Bookwhen "
                       'and Google calendars.',
        'input_schema': {
            'type': 'object',
            'properties': {'days': {'type': 'integer', 'description': 'How far ahead to look, default 14.'}},
            'required': [],
        },
    },
    {
        'name': 'end_call',
        'description': 'Hang up after your current sentence. Use once the caller has said goodbye or is clearly done.',
        'input_schema': {'type': 'object', 'properties': {}, 'required': []},
    },
]


async def run_tool(ctx, name, args, scope=None):
    """`scope`: the Home Assistant entities this call may read; None means all of them (a red phone)."""
    if name == 'home_assistant':
        if not ctx.ha:
            return 'Home Assistant is not connected yet.'
        hits = await ctx.ha.search(args.get('query', ''), args.get('domain', ''), allowed=scope)
        return '\n'.join(hits[:60]) or 'No matching entities.'
    if name == 'wiki_search':
        hits = await ctx.wiki.search(args['query'])
        return json.dumps(hits) if hits else 'Nothing on the wiki matches that.'
    if name == 'wiki_page':
        return await ctx.wiki.page(args['title']) or 'No page with that title.'
    if name == 'upcoming_events':
        events = await ctx.calendars.upcoming(int(args.get('days') or 14))
        return '\n'.join(events) or 'Nothing scheduled in that window.'
    return f'Unknown tool {name}.'


class Operator:
    """Sequoia Tel, the voice of the space: Cartesia ears, Claude brain, Cartesia (or Piper) mouth. Callers can talk over it."""

    def __init__(self, call, ctx):
        self.call, self.ctx = call, ctx
        self.messages = []
        self.reply = None       # the task currently answering, if any
        self.hang_up = False
        self.said = collections.deque(maxlen=20)  # (when, sentence) recently spoken, to spot our own echo
        # Red phones read all of Home Assistant; anything over Twilio (strangers, and trusted cells with a remote dial
        # tone alike) only the public sensors. Asterisk decides which from the channel, not the caller ID. Nobody
        # can switch anything: the switchboard only ever reads states.
        self.ha_scope = None if call.meta.get('internal') else self.ctx.remote_entities

    async def run(self):
        if not (self.ctx.claude and self.ctx.cartesia_key):
            await say(self.call, self.ctx.tts, "Sequoia Tel here, but I'm not quite awake yet: nobody has given me my "
                                                'ears and brain. Try again soon.')
            return
        async with make_stt(self.ctx.session) as stt:
            pump = asyncio.create_task(self._pump(stt))
            try:
                self._answer(None)  # greet
                await asyncio.wait_for(self._listen(stt), MAX_CALL_SECONDS)
            except asyncio.TimeoutError:
                await say(self.call, self.ctx.tts, "We've been talking a while and I have a shop to run. Bye!")
            finally:
                pump.cancel()
                if self.reply:
                    self.reply.cancel()

    async def _pump(self, stt):
        while True:
            await stt.send(await self.call.audio.get())

    async def _listen(self, stt):
        while not self.call.closed.is_set():
            try:
                ev = await asyncio.wait_for(stt.events.get(), IDLE_SECONDS)
            except asyncio.TimeoutError:
                if self.call.playing or (self.reply and not self.reply.done()):
                    continue
                await say(self.call, self.ctx.tts, "I'll let you go. Bye!")
                return
            if ev['type'] == 'closed':
                return
            if ev['type'] in ('partial', 'turn') and self._echo(ev['text']):
                if ev['type'] == 'turn':
                    log.info('%s: (ignored own echo) %s', self.call.id, ev['text'])
                continue
            if ev['type'] == 'partial':
                # Two or more words while we're talking means the caller is talking over us: stop and listen.
                if len(ev['text'].split()) >= 2 and self.reply and not self.reply.done():
                    self._interrupt()
            elif ev['type'] == 'turn' and ev['text']:
                log.info('%s: > %s', self.call.id, ev['text'])
                if self.reply and not self.reply.done():
                    self._interrupt()
                self._answer(ev['text'])
            if self.hang_up and self.reply and self.reply.done():
                return

    def _echo(self, heard):
        """Is this just our own voice coming back down the line? Most of its words were in what we said lately."""
        words = re.findall(r"[a-z']+", heard.lower())
        if len(words) < 2:
            return False
        now = asyncio.get_running_loop().time()
        recent = set(re.findall(r"[a-z']+", ' '.join(t for when, t in self.said if now - when < 10).lower()))
        return sum(w in recent for w in words) / len(words) >= 0.8

    def _interrupt(self):
        self.reply.cancel()
        self.call.flush()

    def _answer(self, text):
        now = datetime.datetime.now(TZ)
        if text is None:
            who = f'line {self.call.caller}' if self.call.meta.get('internal') else 'an outside caller'
            text = (f'[Call connected from {who} at {now:%A %-I:%M %p}. Greet them briefly; they have not '
                    'said anything yet.]')
        else:
            text = f'[{now:%-I:%M %p}] {text}'
        self.reply = asyncio.create_task(self._respond(text))

    async def _respond(self, text):
        # History only grows: an interrupted turn is recorded as what the caller actually heard.
        turn = [{'role': 'user', 'content': text}]
        spoken = []
        sentences = asyncio.Queue()
        speaker = asyncio.create_task(self._speak(sentences, spoken))
        try:
            while True:
                buf = ''
                async with self.ctx.claude.beta.messages.stream(
                    model=self.ctx.model,
                    max_tokens=8000,
                    system=PERSONA,
                    tools=TOOLS,
                    messages=self.messages + turn,
                    output_config={'effort': self.ctx.effort},
                    cache_control={'type': 'ephemeral'},
                    **self.ctx.claude_extras,
                ) as stream:
                    async for event in stream:
                        if event.type == 'text':
                            buf += event.text
                            *done, buf = SENTENCE_END.split(buf)
                            for s in done:
                                sentences.put_nowait(s)
                    msg = await stream.get_final_message()
                if buf.strip():
                    sentences.put_nowait(buf)
                if msg.stop_reason == 'refusal':
                    sentences.put_nowait("Hm, that's not something I can help with.")
                    turn.append({'role': 'assistant', 'content': "Hm, that's not something I can help with."})
                    break
                turn.append({'role': 'assistant', 'content': msg.content})
                calls = [b for b in msg.content if b.type == 'tool_use']
                if msg.stop_reason != 'tool_use' or not calls:
                    break
                results = await asyncio.gather(*(self._tool(b) for b in calls))
                turn.append({'role': 'user', 'content': results})
                if self.hang_up:
                    break  # goodbye is already said; don't ask for another word
            sentences.put_nowait(None)
            await speaker
            await self.call.drain()
            self.messages += turn
            if self.hang_up:
                await self.call.hangup()
        except asyncio.CancelledError:
            speaker.cancel()
            heard = ' '.join(spoken)
            self.messages += [turn[0], {'role': 'assistant', 'content': (heard + ' —') if heard else '(interrupted)'}]
            self.hang_up = False
            raise
        except anthropic.APIError as e:
            log.error('%s: Claude call failed: %s', self.call.id, e)
            speaker.cancel()
            self.call.flush()
            await say(self.call, self.ctx.tts, 'Sorry, I lost my train of thought. Say that again?')

    async def _tool(self, block):
        if block.name == 'end_call':
            self.hang_up = True
            result = 'Hanging up after you finish speaking.'
        else:
            try:
                result = await run_tool(self.ctx, block.name, block.input, self.ha_scope)
            except Exception as e:  # a flaky sensor or wiki shouldn't drop the call
                log.warning('%s: tool %s failed: %s', self.call.id, block.name, e)
                return {'type': 'tool_result', 'tool_use_id': block.id, 'content': f'Failed: {e}', 'is_error': True}
        log.info('%s: %s(%s)', self.call.id, block.name, json.dumps(block.input))
        return {'type': 'tool_result', 'tool_use_id': block.id, 'content': result}

    async def _speak(self, sentences, spoken):
        while (s := await sentences.get()) is not None:
            s = s.strip()
            if not s:
                continue
            log.info('%s: < %s', self.call.id, s)
            async for pcm in self.ctx.tts.stream(s):
                self.call.play(pcm)
            spoken.append(s)
            self.said.append((asyncio.get_running_loop().time(), s))


async def operator(call, ctx):
    await Operator(call, ctx).run()


async def status(call, ctx):
    """411: the space reads out its vitals. No AI involved; a template over Home Assistant entities."""
    now = datetime.datetime.now(TZ)
    lines = [f"Sequoia Tel here with the shop report. It's {now:%-I:%M %p} on {now:%A}."]
    if ctx.ha:
        for entity_id, label in ctx.status_entities:
            if not call.meta.get('internal') and entity_id not in ctx.remote_entities:
                continue
            s = await ctx.ha.get(entity_id)
            if s and s['state'] not in ('unavailable', 'unknown'):
                unit = s.get('attributes', {}).get('unit_of_measurement', '')
                unit = {'°F': 'degrees', '°C': 'degrees celsius', 'ppm': 'parts per million', '%': 'percent'}.get(unit, unit)
                lines.append(f'{label}: {s["state"].replace("_", " ")} {unit}.'.replace(' .', '.'))
    else:
        lines.append('My sensors are not connected yet.')
    lines.append("That's the report. Keep making things.")
    await say(call, ctx.tts, ' '.join(lines))


APPS = {
    'operator': operator,
    'status': status,
}
