FROM python:3.12-slim
RUN pip install --no-cache-dir anthropic==2.3.0 aiohttp==3.14.5 piper-tts==1.8.0 icalendar==7.3.0 recurring-ical-events==3.8.2
# Piper's offline voice: the fallback mouth when no Cartesia voice is configured, and the robot for gags
ADD https://huggingface.co/rhasspy/piper-voices/resolve/main/en/en_US/lessac/medium/en_US-lessac-medium.onnx /voices/
ADD https://huggingface.co/rhasspy/piper-voices/resolve/main/en/en_US/lessac/medium/en_US-lessac-medium.onnx.json /voices/
COPY *.py persona.md /app/
RUN chmod -R a+rX /voices /app
WORKDIR /app
USER nobody
ENTRYPOINT ["python3", "-u", "/app/switchboard.py"]
