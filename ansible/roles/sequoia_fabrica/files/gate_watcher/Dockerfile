FROM python:3.12-slim
# numpy/opencv pinned to the versions Frigate runs on nursery: its QEMU vCPU has no AVX
RUN pip install --no-cache-dir numpy==1.26.4 opencv-python-headless==4.11.0.86 paho-mqtt==2.1.0
COPY gate_watcher.py /usr/local/bin/gate_watcher.py
USER nobody
ENTRYPOINT ["python3", "-u", "/usr/local/bin/gate_watcher.py"]
