"""Asterisk AudioSocket: one TCP connection per call, carrying 8 kHz signed-linear audio both ways.

Every message is a 3-byte header (1-byte kind, 2-byte big-endian length) and a payload. Asterisk sends the call's
UUID first, then 20 ms audio frames for as long as the call lasts. Whatever we write back is what the caller hears.
See https://docs.asterisk.org/Configuration/Channel-Drivers/AudioSocket/
"""
import asyncio
import logging
import struct
import uuid

KIND_HANGUP = 0x00
KIND_UUID = 0x01
KIND_SILENCE = 0x02
KIND_DTMF = 0x03
KIND_AUDIO = 0x10
KIND_ERROR = 0xFF

SAMPLE_RATE = 8000
FRAME_BYTES = 320  # 20 ms of 16-bit mono at 8 kHz
FRAME_SECONDS = 0.02

log = logging.getLogger('audiosocket')


async def read_message(reader):
    kind, length = struct.unpack('>BH', await reader.readexactly(3))
    payload = await reader.readexactly(length) if length else b''
    return kind, payload


async def read_uuid(reader):
    kind, payload = await read_message(reader)
    if kind != KIND_UUID or len(payload) != 16:
        raise ValueError(f'expected a UUID message first, got kind {kind:#x}')
    return str(uuid.UUID(bytes=payload))


class Call:
    """One live call. Audio from the caller lands on `audio`; `play()` queues audio to the caller, paced in real time."""

    def __init__(self, reader, writer, call_id, meta):
        self.reader, self.writer = reader, writer
        self.id = call_id
        self.meta = meta
        self.audio = asyncio.Queue(maxsize=500)  # ~10 s of backlog before we start dropping caller audio
        self.dtmf = asyncio.Queue()
        self.closed = asyncio.Event()
        self._out = bytearray()
        self._idle = asyncio.Event()
        self._idle.set()

    @property
    def caller(self):
        return self.meta.get('caller', '')

    @property
    def playing(self):
        return bool(self._out)

    def play(self, pcm):
        if pcm:
            self._out += pcm
            self._idle.clear()

    def flush(self):
        """Stop talking right now (the caller barged in)."""
        self._out.clear()
        self._idle.set()

    async def drain(self):
        await self._idle.wait()

    async def hangup(self):
        if not self.closed.is_set():
            try:
                self.writer.write(struct.pack('>BH', KIND_HANGUP, 0))
                await self.writer.drain()
            except ConnectionError:
                pass
        self.closed.set()

    async def run(self):
        """Pump audio both ways until either side hangs up."""
        tasks = [asyncio.create_task(self._receive()), asyncio.create_task(self._send())]
        try:
            await self.closed.wait()
        finally:
            for t in tasks:
                t.cancel()
            self.writer.close()

    async def _receive(self):
        try:
            while True:
                kind, payload = await read_message(self.reader)
                if kind == KIND_AUDIO:
                    if self.audio.full():
                        self.audio.get_nowait()
                    self.audio.put_nowait(payload)
                elif kind == KIND_DTMF:
                    self.dtmf.put_nowait(payload.decode('ascii', 'replace'))
                elif kind == KIND_HANGUP:
                    break
                elif kind == KIND_ERROR:
                    log.warning('%s: Asterisk reported error %s', self.id, payload.hex())
        except (asyncio.IncompleteReadError, ConnectionError):
            pass
        self.closed.set()

    async def _send(self):
        loop = asyncio.get_running_loop()
        tick = loop.time()
        try:
            while True:
                if self._out:
                    frame = bytes(self._out[:FRAME_BYTES]).ljust(FRAME_BYTES, b'\0')
                    del self._out[:FRAME_BYTES]
                    self.writer.write(struct.pack('>BH', KIND_AUDIO, len(frame)) + frame)
                    await self.writer.drain()
                    if not self._out:
                        self._idle.set()
                tick = max(tick + FRAME_SECONDS, loop.time() - FRAME_SECONDS)
                await asyncio.sleep(tick - loop.time())
        except ConnectionError:
            self.closed.set()
