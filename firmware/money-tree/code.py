"""Read and publish the money-tree soil-moisture percentage."""

import os
import time

import alarm
import analogio
import board
import socketpool
import wifi

from payload import moisture_payload

import adafruit_minimqtt.adafruit_minimqtt as MQTT


def required_setting(name):
    """Return a required settings.toml value or raise a useful error."""
    value = os.getenv(name)
    if value is None or value == "":
        raise RuntimeError("Missing required setting: " + name)
    return value


def boolean_setting(name, default=False):
    """Read a true/false setting while tolerating string or bool values."""
    value = os.getenv(name)
    if value is None:
        return default
    if isinstance(value, bool):
        return value
    normalized = str(value).strip().lower()
    if normalized in ("true", "1", "yes", "on"):
        return True
    if normalized in ("false", "0", "no", "off"):
        return False
    raise RuntimeError(name + " must be true or false")


WIFI_SSID = required_setting("CIRCUITPY_WIFI_SSID")
WIFI_PASSWORD = required_setting("CIRCUITPY_WIFI_PASSWORD")
MQTT_BROKER = required_setting("MQTT_BROKER")
MQTT_PORT = int(os.getenv("MQTT_PORT", 1883))
MQTT_TOPIC = os.getenv("MQTT_TOPIC", "plants/money-tree/moisture")
DRY_VALUE = int(required_setting("DRY_VALUE"))
WET_VALUE = int(required_setting("WET_VALUE"))
SAMPLE_COUNT = int(os.getenv("SAMPLE_COUNT", 20))
CONTINUOUS_MODE = boolean_setting("CONTINUOUS_MODE", False)
PUBLISH_INTERVAL_SECONDS = int(os.getenv("PUBLISH_INTERVAL_SECONDS", 30))

if not 0 <= DRY_VALUE <= 65535:
    raise RuntimeError("DRY_VALUE must be between 0 and 65535")
if not 0 <= WET_VALUE <= 65535:
    raise RuntimeError("WET_VALUE must be between 0 and 65535")
if DRY_VALUE == WET_VALUE:
    raise RuntimeError("DRY_VALUE and WET_VALUE must be different")
if SAMPLE_COUNT < 1:
    raise RuntimeError("SAMPLE_COUNT must be at least 1")
if PUBLISH_INTERVAL_SECONDS < 1:
    raise RuntimeError("PUBLISH_INTERVAL_SECONDS must be at least 1")

mac_suffix = "".join("%02x" % byte for byte in wifi.radio.mac_address[-3:])
client_id = "money-tree-" + mac_suffix


def read_moisture():
    """Return raw sample statistics and a calibrated percentage."""
    # A0 uses ADC2 on this board. Disable Wi-Fi so it cannot contend with the
    # ADC while the measurement is being taken.
    wifi.radio.enabled = False
    time.sleep(0.25)

    sensor = analogio.AnalogIn(board.A0)
    samples = []
    try:
        for _ in range(SAMPLE_COUNT):
            samples.append(sensor.value)
            time.sleep(0.1)
    finally:
        sensor.deinit()

    average = sum(samples) // len(samples)
    percentage = round((average - DRY_VALUE) * 100 / (WET_VALUE - DRY_VALUE))
    percentage = max(0, min(100, percentage))
    return min(samples), average, max(samples), percentage


def shutdown(message):
    """Turn off Wi-Fi and deep sleep until a reset or power-cycle."""
    print(message)
    wifi.radio.enabled = False
    time.sleep(0.25)
    alarm.exit_and_deep_sleep_until_alarms()


def publish_moisture(raw, moisture):
    """Attempt one bounded Wi-Fi/MQTT publication and report success."""
    mqtt = None
    try:
        wifi.radio.enabled = True
        if not wifi.radio.connected:
            print("Connecting to Wi-Fi:", WIFI_SSID)
            wifi.radio.connect(WIFI_SSID, WIFI_PASSWORD)
            print("Wi-Fi connected; address:", wifi.radio.ipv4_address)

        mqtt = MQTT.MQTT(
            broker=MQTT_BROKER,
            port=MQTT_PORT,
            client_id=client_id,
            socket_pool=socketpool.SocketPool(wifi.radio),
            keep_alive=30,
            connect_retries=3,
        )

        print("Connecting to MQTT:", MQTT_BROKER, MQTT_PORT)
        mqtt.connect()
        payload = moisture_payload(raw, moisture)
        mqtt.publish(MQTT_TOPIC, payload, qos=1, retain=True)
        print("Published retained moisture:", payload)
        mqtt.disconnect()
        return True
    except Exception as error:  # CircuitPython prints the concrete error below.
        print("Connection or publish failed:", repr(error))
        if mqtt is not None:
            try:
                mqtt.disconnect()
            except Exception:
                pass
        return False


print("Money-tree moisture publisher")
print("Topic:", MQTT_TOPIC)
print("Mode:", "continuous" if CONTINUOUS_MODE else "one-shot battery")

while True:
    try:
        minimum, raw_value, maximum, moisture = read_moisture()
        print("Raw min:", minimum)
        print("Raw average:", raw_value)
        print("Raw max:", maximum)
        print("Moisture:", moisture, "%")
        published = publish_moisture(raw_value, moisture)
    except Exception as error:  # Keep continuous mode recoverable from sensor errors.
        print("Measurement failed:", repr(error))
        published = False

    if not CONTINUOUS_MODE:
        if published:
            shutdown("Measurement complete; sleeping until reset.")
        shutdown("Measurement or network attempt failed; sleeping until reset.")

    wifi.radio.enabled = False
    print("Next measurement in", PUBLISH_INTERVAL_SECONDS, "seconds.")
    time.sleep(PUBLISH_INTERVAL_SECONDS)
