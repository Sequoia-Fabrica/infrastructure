# Canary — Sequoia Fabrica CO2 sensor

ESPHome firmware for the hackerspace CO2 monitor. Shows CO2 / temperature / humidity on
the OLED and reports to Home Assistant via MQTT discovery (device **Canary**).

## Hardware

| Part | Detail |
|---|---|
| MCU | LOLIN/WEMOS S2 Mini (ESP32-S2FN4R2, 4MB flash) |
| Sensor | Sensirion SCD30 (CO2, temp, RH) — I2C 0x61 |
| Display | SSD1306 128x32 OLED — I2C 0x3C |
| I2C | SDA = GPIO12, SCL = GPIO11, 50kHz (SCD30 needs slow clock / clock stretching) |

## Home Assistant

- Broker: Mosquitto on the HA box (192.168.88.254:1883)
- Discovery prefix `homeassistant/`, state under `canary/…`, availability `canary/status`
- Entities: `sensor.canary_co2`, `sensor.canary_temperature`, `sensor.canary_humidity`, plus diagnostics
- Web UI: http://canary.local/

## Building / flashing

```sh
cp secrets.example.yaml secrets.yaml   # fill in real values
esphome run canary.yaml --device canary.local   # OTA over WiFi
```

USB flashing (first time / recovery): hold **BOOT (GPIO0)**, tap **RST**, release BOOT, then
`esphome run canary.yaml --device /dev/cu.usbmodem01`.

If the WiFi changes, the board opens a `canary-setup` access point with a captive portal.

## History

The board originally ran CircuitPython 8.1 with a `code.py` that never reached MQTT:
2024-era `adafruit_minimqtt` requires a `socket_pool` (and CircuitPython 9+), so `connect()`
always threw and a bare `except:` hid it. It also used a non-discovery topic (`/feeds/co2`) and
published CO2 only. A full flash backup of the original firmware lives in `dump/` (not committed).
