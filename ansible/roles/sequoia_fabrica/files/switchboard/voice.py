"""Ears and mouths: Cartesia for streaming speech-to-text (Ink) and text-to-speech (Sonic); Piper locally as a fallback."""
import asyncio
import audioop  # deprecated but present on 3.12; plenty for one resample per utterance
import json
import logging
import os
import urllib.parse

import aiohttp

from audiosocket import SAMPLE_RATE

log = logging.getLogger('voice')


class CartesiaSTT:
    """A live transcription stream with Cartesia's own turn detection. Push caller audio in with `send`.

    `events` yields {'type': 'partial', 'text'} while the caller talks (cumulative for the turn),
    {'type': 'turn', 'text'} when they've finished a turn, and {'type': 'closed'} at the end.
    """

    URL = 'wss://api.cartesia.ai/stt/turns/websocket'

    def __init__(self, session, api_key, model='ink-2', version='2026-03-01'):
        self.session, self.api_key, self.model, self.version = session, api_key, model, version
        self.events = asyncio.Queue()
        self.ws = None
        self._reader = None

    async def __aenter__(self):
        params = {'model': self.model, 'encoding': 'pcm_s16le', 'sample_rate': str(SAMPLE_RATE)}
        self.ws = await self.session.ws_connect(
            f'{self.URL}?{urllib.parse.urlencode(params)}',
            headers={'X-API-Key': self.api_key, 'Cartesia-Version': self.version},
            heartbeat=10,
        )
        self._reader = asyncio.create_task(self._read())
        return self

    async def __aexit__(self, *exc):
        self._reader.cancel()
        await self.ws.close()  # closing the socket is how a Cartesia STT session ends

    async def send(self, pcm):
        await self.ws.send_bytes(pcm)

    async def _read(self):
        async for msg in self.ws:
            if msg.type != aiohttp.WSMsgType.TEXT:
                continue
            data = json.loads(msg.data)
            kind, text = data.get('type'), (data.get('transcript') or '').strip()
            if kind == 'turn.update' and text:
                self.events.put_nowait({'type': 'partial', 'text': text})
            elif kind == 'turn.end':
                self.events.put_nowait({'type': 'turn', 'text': text})
            elif kind == 'error':
                log.error('Cartesia STT error: %s', data)
        self.events.put_nowait({'type': 'closed'})


class CartesiaTTS:
    """Cartesia Sonic, streamed straight out as 8 kHz PCM."""

    URL = 'https://api.cartesia.ai/tts/bytes'

    def __init__(self, session, api_key, voice, model='sonic-3.6', version='2026-08-14'):
        self.session, self.api_key, self.voice, self.model, self.version = session, api_key, voice, model, version

    async def stream(self, text):
        body = {
            'model_id': self.model,
            'transcript': text,
            'voice': {'mode': 'id', 'id': self.voice},
            'output_format': {'container': 'raw', 'encoding': 'pcm_s16le', 'sample_rate': SAMPLE_RATE},
            'language': 'en',
        }
        headers = {'Authorization': f'Bearer {self.api_key}', 'Cartesia-Version': self.version}
        async with self.session.post(self.URL, json=body, headers=headers) as resp:
            resp.raise_for_status()
            carry = b''
            async for chunk in resp.content.iter_any():
                chunk = carry + chunk
                cut = len(chunk) & ~1  # keep whole 16-bit samples
                carry = chunk[cut:]
                if cut:
                    yield chunk[:cut]


class PiperTTS:
    """Piper running on nursery's CPU: free, offline, a bit robotic. Good for gags and as a fallback."""

    def __init__(self, model_path):
        from piper import PiperVoice
        self.voice = PiperVoice.load(model_path)

    def _synth(self, text):
        out, rate = [], None
        for chunk in self.voice.synthesize(text):
            out.append(chunk.audio_int16_bytes)
            rate = chunk.sample_rate
        pcm = b''.join(out)
        if rate and rate != SAMPLE_RATE:
            pcm, _ = audioop.ratecv(pcm, 2, 1, rate, SAMPLE_RATE, None)
        return pcm

    async def stream(self, text):
        yield await asyncio.to_thread(self._synth, text)


def make_stt(session):
    return CartesiaSTT(session, os.environ['CARTESIA_API_KEY'],
                       model=os.environ.get('CARTESIA_STT_MODEL', 'ink-2'),
                       version=os.environ.get('CARTESIA_STT_VERSION', '2026-03-01'))


def make_tts(session):
    key, voice = os.environ.get('CARTESIA_API_KEY'), os.environ.get('CARTESIA_VOICE')
    if key and voice:
        return CartesiaTTS(session, key, voice,
                           model=os.environ.get('CARTESIA_MODEL', 'sonic-3.6'),
                           version=os.environ.get('CARTESIA_VERSION', '2026-08-14'))
    return local_tts()


_piper = None


def local_tts():
    global _piper
    if _piper is None:
        _piper = PiperTTS(os.environ.get('PIPER_VOICE', '/voices/en_US-lessac-medium.onnx'))
    return _piper


async def say(call, tts, text):
    """Speak a whole line and wait until the caller has heard it."""
    async for pcm in tts.stream(text):
        call.play(pcm)
    await call.drain()
