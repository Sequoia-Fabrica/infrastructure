#!/usr/bin/env python3
"""Watch the front roll-up gate through Frigate's front_door camera and publish its state to Home Assistant.

The gate's slats are a dense field of horizontal edges; the glass doors behind it are mostly vertical edges.
We split the gate opening into horizontal bands and measure, per band, what share of the gradient energy is
horizontal ("slatness"). Counting slat bands down from the top finds the gate's bottom edge, which gives
closed / partially open / open plus an approximate open percentage. Classic CV only: no model, no AVX.

Results go to MQTT with Home Assistant discovery, so HA creates the entities on its own.

Usage:
  gate_watcher.py                 run forever (config from environment)
  gate_watcher.py --image X.jpg   classify local images and exit (for calibration)
"""
import argparse
import json
import logging
import os
import statistics
import sys
import time
import urllib.request

import cv2
import numpy as np

log = logging.getLogger('gate_watcher')

FRIGATE_URL = os.environ.get('FRIGATE_URL', 'http://127.0.0.1:5000')
CAMERA = os.environ.get('CAMERA', 'front_door')
INTERVAL = float(os.environ.get('INTERVAL', '30'))
CONFIRM = int(os.environ.get('CONFIRM', '3'))  # consecutive identical readings before a state change
MQTT_HOST = os.environ.get('MQTT_HOST', '192.168.88.254')
MQTT_PORT = int(os.environ.get('MQTT_PORT', '1883'))
MQTT_USER = os.environ.get('MQTT_USER', 'mqtt')
MQTT_PASSWORD = os.environ.get('MQTT_PASSWORD', '')
TOPIC = 'seqfab/front_gate'

# Calibrated 2026-10-17 on front_door frames (both the 704x480 detect stream and the 2592x1944 record stream).
# Normalized to the frame: the gate opening from just above the fully-raised gate edge down to the threshold,
# clear of the laser cutter at the right.
X1, X2 = 0.58, 0.82
Y_TOP, Y_FLOOR = 0.10, 0.62
BANDS = 26
STRIPS = 3            # vertical strips; the median edge ignores a person blocking one of them
SLAT = 0.62           # band slatness above this is gate (slats ~0.75-0.90, doors/street ~0.2-0.6)
RUN_OPEN = 10         # fully raised: only the rolled-up edge, ~10 bands
RUN_CLOSED = 23       # at or past this many bands the gate is down
MIN_EDGE_ENERGY = 2.0  # mean gradient below this means a black/frozen frame, not a reading


def slat_run(profile):
    """Bands of gate counted down from the top, tolerating a single non-slat band."""
    run, gap = 0, 0
    for i, v in enumerate(profile):
        if v > SLAT:
            run, gap = i + 1, 0
        else:
            gap += 1
            if gap > 1:
                break
    return run


def classify(img_bgr):
    h, w = img_bgr.shape[:2]
    g = cv2.cvtColor(img_bgr[int(Y_TOP * h):int(Y_FLOOR * h), int(X1 * w):int(X2 * w)], cv2.COLOR_BGR2GRAY)
    g = cv2.resize(g, (STRIPS * 66, BANDS * 12), interpolation=cv2.INTER_AREA)
    g = cv2.GaussianBlur(g, (3, 3), 0).astype(np.float32)
    gx = np.abs(cv2.Sobel(g, cv2.CV_32F, 1, 0, ksize=3))
    gy = np.abs(cv2.Sobel(g, cv2.CV_32F, 0, 1, ksize=3))
    if float((gx + gy).mean()) < MIN_EDGE_ENERGY:
        return None
    runs = []
    for s in np.array_split(np.arange(g.shape[1]), STRIPS):
        bx = gx[:, s].reshape(BANDS, -1).sum(1)
        by = gy[:, s].reshape(BANDS, -1).sum(1)
        runs.append(slat_run(by / (bx + by + 1e-6)))
    run = statistics.median(runs)
    open_pct = int(round(min(max((RUN_CLOSED - run) / (RUN_CLOSED - RUN_OPEN), 0.0), 1.0) * 100 / 5) * 5)
    state = 'closed' if run >= RUN_CLOSED else 'open' if run <= RUN_OPEN + 2 else 'partially_open'
    return {'state': state, 'open_percent': open_pct, 'bands': run, 'strips': runs}


def fetch_frame():
    url = f'{FRIGATE_URL}/api/{CAMERA}/latest.jpg?h=480'
    data = urllib.request.urlopen(url, timeout=15).read()
    img = cv2.imdecode(np.frombuffer(data, np.uint8), cv2.IMREAD_COLOR)
    if img is None:
        raise ValueError(f'undecodable frame from {url}')
    return img


def discovery(client):
    device = {'identifiers': ['seqfab_front_gate_camera'], 'name': 'Front Gate (camera)',
              'manufacturer': 'Sequoia Fabrica', 'model': 'gate_watcher (Frigate front_door)'}
    common = {'state_topic': f'{TOPIC}/state', 'availability_topic': f'{TOPIC}/availability',
              'json_attributes_topic': f'{TOPIC}/state', 'device': device}
    entities = {
        'binary_sensor/front_gate': {
            'name': 'Front gate', 'unique_id': 'seqfab_front_gate', 'default_entity_id': 'binary_sensor.front_gate',
            'device_class': 'garage_door', 'value_template': "{{ 'OFF' if value_json.state == 'closed' else 'ON' }}"},
        'sensor/front_gate_position': {
            'name': 'Front gate position', 'unique_id': 'seqfab_front_gate_position',
            'default_entity_id': 'sensor.front_gate_position', 'device_class': 'enum',
            'options': ['open', 'partially_open', 'closed'], 'value_template': '{{ value_json.state }}'},
        'sensor/front_gate_open_percent': {
            'name': 'Front gate open', 'unique_id': 'seqfab_front_gate_open_percent',
            'default_entity_id': 'sensor.front_gate_open', 'unit_of_measurement': '%',
            'state_class': 'measurement', 'icon': 'mdi:garage-open-variant',
            'value_template': '{{ value_json.open_percent }}'},
    }
    for path, cfg in entities.items():
        client.publish(f'homeassistant/{path}/config', json.dumps({**common, **cfg}), qos=1, retain=True)


def run_forever():
    import paho.mqtt.client as mqtt

    client = mqtt.Client(mqtt.CallbackAPIVersion.VERSION2, client_id='seqfab_gate_watcher')
    client.username_pw_set(MQTT_USER, MQTT_PASSWORD)
    client.will_set(f'{TOPIC}/availability', 'offline', qos=1, retain=True)

    def on_connect(c, userdata, flags, reason_code, properties):
        log.info('MQTT connected: %s', reason_code)
        discovery(c)
        c.publish(f'{TOPIC}/availability', 'online', qos=1, retain=True)

    client.on_connect = on_connect
    client.connect_async(MQTT_HOST, MQTT_PORT)
    client.loop_start()

    published, candidate, streak, failures = None, None, 0, 0
    while True:
        started = time.monotonic()
        try:
            reading = classify(fetch_frame())
            failures = 0
        except Exception as e:  # Frigate restarting, camera down, network blip
            reading = None
            failures += 1
            log.warning('frame fetch failed (%d): %s', failures, e)
            if failures == 5:
                client.publish(f'{TOPIC}/availability', 'offline', qos=1, retain=True)
        if reading is not None:
            if failures == 0 and published is not None:
                client.publish(f'{TOPIC}/availability', 'online', qos=1, retain=True)
            streak = streak + 1 if candidate and reading['state'] == candidate['state'] else 1
            candidate = reading
            if published is None or (streak >= CONFIRM and (
                    reading['state'] != published['state'] or abs(reading['open_percent'] - published['open_percent']) >= 15)):
                payload = {**reading, 'updated': int(time.time())}
                client.publish(f'{TOPIC}/state', json.dumps(payload), qos=1, retain=True)
                log.info('published %s', payload)
                published = reading
        time.sleep(max(0.0, INTERVAL - (time.monotonic() - started)))


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument('--image', nargs='+', help='classify these image files and exit')
    args = ap.parse_args()
    logging.basicConfig(level=logging.INFO, format='%(asctime)s %(levelname)s %(message)s')
    if args.image:
        for path in args.image:
            img = cv2.imread(path)
            print(path, 'unreadable' if img is None else classify(img))
        return
    if not MQTT_PASSWORD:
        sys.exit('MQTT_PASSWORD is not set')
    run_forever()


if __name__ == '__main__':
    main()
