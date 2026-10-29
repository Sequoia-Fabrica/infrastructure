#!/usr/bin/env python3
"""switchboard: the brains behind the red phones.

Asterisk handles the phones and the Twilio line; whenever the dialplan wants something clever it hands the call over:

  1. CURL(http://127.0.0.1:9093/calls/<uuid>?app=<app>&caller=<num>)  registers what the call is for
  2. AudioSocket(<uuid>,127.0.0.1:9092)                                  streams the call's audio here

and the matching app in apps.py runs the call until it hangs up or returns, when the dialplan carries on.

Usage:
  switchboard.py                      run forever (config from environment)
  switchboard.py --say "text" OUT.raw synthesize with the configured voice to 8 kHz s16le and exit (for testing)
"""
import argparse
import asyncio
import logging
import os
import sys
import time

import aiohttp
import anthropic
from aiohttp import web

import apps
import audiosocket
from knowledge import Calendars, HomeAssistant, Wiki
from voice import make_tts

log = logging.getLogger('switchboard')

AUDIO_PORT = int(os.environ.get('AUDIO_PORT', '9092'))
API_PORT = int(os.environ.get('API_PORT', '9093'))
LISTEN = os.environ.get('LISTEN', '127.0.0.1')
PENDING_SECONDS = 30


class Context:
    """Everything an app might need, shared across calls."""

    def __init__(self, session):
        env = os.environ.get
        self.session = session
        self.tts = make_tts(session)
        self.cartesia_key = env('CARTESIA_API_KEY')
        self.effort = env('CLAUDE_EFFORT', 'low')
        if env('ANTHROPIC_API_KEY'):
            self.claude = anthropic.AsyncAnthropic()
            self.model = env('CLAUDE_MODEL', 'claude-opus-5-5')
            # if a safety classifier declines, let Anthropic re-run the turn on a suitable model instead of failing
            self.claude_extras = {'betas': ['server-side-fallback-2026-07-01'], 'extra_body': {'fallbacks': 'default'}}
        elif env('OPENROUTER_API_KEY'):
            # OpenRouter speaks the Anthropic Messages API, so the same SDK works pointed at it
            self.claude = anthropic.AsyncAnthropic(base_url='https://openrouter.ai/api', api_key=None,
                                                   auth_token=env('OPENROUTER_API_KEY'))
            self.model = env('CLAUDE_MODEL', 'anthropic/claude-opus-5.5')
            self.claude_extras = {}
        else:
            self.claude, self.model, self.claude_extras = None, None, {}
        self.ha = HomeAssistant(session, env('HA_URL', 'http://192.168.88.254:8123'), env('HA_TOKEN')) \
            if env('HA_TOKEN') else None
        self.wiki = Wiki(session, env('WIKI_API', 'https://wiki.sequoiafabrica.org/w/api.php'))
        self.calendars = Calendars(session, env('CALENDARS', '').split())
        # STATUS_ENTITIES="sensor.foo=Inside temperature;binary_sensor.bar=Front gate"
        self.status_entities = [tuple(pair.split('=', 1)) for pair in env('STATUS_ENTITIES', '').split(';') if '=' in pair]
        # what outside callers may read; red phones see everything
        self.remote_entities = {e.strip() for e in env('REMOTE_ENTITIES', '').split(',') if e.strip()}


class Switchboard:
    def __init__(self, ctx):
        self.ctx = ctx
        self.pending = {}  # uuid -> (registered at, meta)

    async def register(self, request):
        app = request.query.get('app', 'operator')
        if app not in apps.APPS:
            return web.Response(text='unknown app', status=404)
        now = time.monotonic()
        self.pending = {k: v for k, v in self.pending.items() if now - v[0] < PENDING_SECONDS}
        meta = dict(request.query)
        meta['internal'] = request.query.get('internal') == '1'
        self.pending[request.match_info['uuid']] = (now, meta)
        return web.Response(text='ok')

    async def healthz(self, request):
        return web.Response(text='ok')

    async def handle(self, reader, writer):
        try:
            call_id = await asyncio.wait_for(audiosocket.read_uuid(reader), 5)
        except (ValueError, asyncio.TimeoutError, asyncio.IncompleteReadError) as e:
            log.warning('bad AudioSocket connection: %s', e)
            writer.close()
            return
        _, meta = self.pending.pop(call_id, (0, {'app': 'operator'}))
        call = audiosocket.Call(reader, writer, call_id, meta)
        app = apps.APPS[meta.get('app', 'operator')]
        log.info('%s: %s from %r', call_id, meta.get('app', 'operator'), call.caller)
        io = asyncio.create_task(call.run())
        task = asyncio.create_task(app(call, self.ctx))
        try:
            await asyncio.wait([task, io], return_when=asyncio.FIRST_COMPLETED)
            if task.done() and task.exception():
                log.error('%s: app crashed', call_id, exc_info=task.exception())
        finally:
            if task.done() and not call.closed.is_set():
                await call.drain()  # let the app's last words finish before handing the call back to the dialplan
            task.cancel()
            call.closed.set()
            await io
            log.info('%s: done', call_id)


async def serve():
    async with aiohttp.ClientSession() as session:
        ctx = Context(session)
        board = Switchboard(ctx)
        api = web.Application()
        api.add_routes([web.get('/calls/{uuid}', board.register), web.get('/healthz', board.healthz)])
        runner = web.AppRunner(api)
        await runner.setup()
        await web.TCPSite(runner, LISTEN, API_PORT).start()
        server = await asyncio.start_server(board.handle, LISTEN, AUDIO_PORT)
        log.info('listening: audio %s:%d, api %s:%d; tts=%s claude=%s stt=%s ha=%s calendars=%d',
                 LISTEN, AUDIO_PORT, LISTEN, API_PORT, type(ctx.tts).__name__, ctx.model,
                 bool(ctx.cartesia_key), bool(ctx.ha), len(ctx.calendars.urls))
        async with server:
            await server.serve_forever()


async def say_to_file(text, path):
    async with aiohttp.ClientSession() as session:
        with open(path, 'wb') as f:
            async for pcm in make_tts(session).stream(text):
                f.write(pcm)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--say', nargs=2, metavar=('TEXT', 'OUT'))
    args = parser.parse_args()
    logging.basicConfig(level=logging.INFO, format='%(asctime)s %(name)s %(levelname)s %(message)s')
    if args.say:
        asyncio.run(say_to_file(*args.say))
        return
    asyncio.run(serve())


if __name__ == '__main__':
    sys.exit(main())
