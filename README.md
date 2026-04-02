# B2500 Meter Go

An emulator for the Shelly Pro 3EM power meter, designed to work with the Marstek B2500 battery (and similar systems). It allows the battery to "see" power readings from various sources (like Tasmota devices) and adjust its output accordingly using the "Auto" mode.

## Features

- **Shelly Pro 3EM Emulation**: Responds to UDP status requests on ports 1010 and 2220.
- **Multiple Providers**: Aggregate readings from multiple power meters (Tasmota, MQTT, Mock).
- **Non-Blocking Throttling**: Limits data fetch frequency using efficient caching to ensure low-latency UDP responses.
- **Structured Logging**: Configurable log levels (`debug`, `info`, `warn`, `error`) using Go's modern `slog` package.
- **Dockerized**: Ready to run in a lightweight container.

## Quick Start

The easiest way to run the emulator is using Docker.

1.  **Create a `config.yaml`** with your power sources:

    ```yaml
    providers:
      - type: tasmota
        ip: 192.168.178.6
        status: StatusSNS
        payload: SML
        label: Power
      - type: mqtt
        broker: 192.168.178.10
        port: 1883
        topic: tele/my_sensor/SENSOR
        json_path: ENERGY.Power
    ```

2.  **Run the container**:

    ```bash
    docker run -d \
      --name b2500-meter \
      -p 1010:1010/udp \
      -p 2220:2220/udp \
      -v $(pwd)/config.yaml:/app/config.yaml \
      ghcr.io/colenio-mhe/b2500-meter-go:latest
    ```

The Marstek battery will now find the emulated Shelly on your network. Make sure the battery is in **"Auto"** mode.

## Configuration Options

The `config.yaml` file supports the following options:

#### Global Options
- `log_level`: Verbosity of the logs. Set to `debug` to see raw power fetches and connection details.
- `device`: The type of device to emulate (currently only `shellypro3em` is supported).
- `device_id`: The source ID reported in JSON-RPC responses.

#### Provider Options (Tasmota)
- `ip`: IP address of the Tasmota device.
- `user`/`password`: (Optional) For HTTP authentication.
- `status`: JSON key for status (usually `StatusSNS`).
- `payload`: JSON key for the sensor payload (e.g., `SML`, `ENERGY`).
- `label`: JSON key for the power value (when `calculate` is `false`).
- `calculate`: If `true`, calculates net power using `label_in` and `label_out`.
- `label_in`: JSON key for imported power (required if `calculate: true`).
- `label_out`: JSON key for exported power (required if `calculate: true`).
- `throttle`: Minimum interval (in seconds) between fetches from the device. Calls within this interval return the last cached value instantly.

#### Provider Options (Mock)
- `power`: Static power value in Watts.

#### Provider Options (MQTT)
- `broker`: Hostname or IP of the MQTT broker.
- `port`: Port of the MQTT broker (usually `1883`).
- `topic`: MQTT topic to subscribe to.
- `user`/`password`: (Optional) For MQTT authentication.
- `json_path`: (Optional) [GJSON path](https://github.com/tidwall/gjson) to extract the power value from a JSON payload. If omitted, the raw payload is parsed as a float.

## Alternative Installation

### Build and Run Locally

If you prefer to run the binary directly on your host machine:

```bash
go run cmd/b2500-meter/main.go --config config.yaml
```

### Build Docker Image Locally

```bash
docker build -t b2500-meter-go .
docker run -d \
  --name b2500-meter \
  -p 1010:1010/udp \
  -p 2220:2220/udp \
  -v $(pwd)/config.yaml:/app/config.yaml \
  b2500-meter-go
```

## Credits

This project was inspired by [tomquist/b2500-meter](https://github.com/tomquist/b2500-meter), which provides a similar emulator written in Python.

## How it works

The Marstek B2500 battery expects a Shelly Pro 3EM on the local network to provide real-time power consumption data. This emulator listens for the battery's UDP broadcast requests on the standard Shelly ports and responds with formatted JSON-RPC messages.

The emulator handles the specific rounding and "decimal point enforcement" (e.g., adding 0.001 to integers) required for the battery to correctly parse the power values.

When you configure multiple providers, the emulator sums the power values from all of them before reporting the total to the battery.
