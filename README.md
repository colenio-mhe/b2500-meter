# B2500 Meter Go

An emulator for the Shelly Pro 3EM power meter, designed to work with the Marstek B2500 battery (and similar systems). It allows the battery to "see" power readings from various sources (like Tasmota devices) and adjust its output accordingly using the "Auto" mode.

## Features

- **Shelly Pro 3EM Emulation**: Responds to UDP status requests on ports 1010 and 2220.
- **Multiple Providers**: Aggregate readings from multiple power meters (Tasmota, MQTT, Serial, Mock).
- **Synchronous Blocking (No-Cache Policy)**: To prevent control loop oscillations, this emulator **never** sends old or cached data.
    - **Pull-Providers (Tasmota)**: Blocks requests until the `throttle` interval has passed.
    - **Push-Providers (MQTT/Serial)**: Sends fresh value or drops package.
- **Error Resilience (Configurable)**: Can optionally return 0W if a fetch fails using the `zero_fallback` option to stabilize the battery's control loop.
- **High Performance**: Optimized JSON parsing using `gjson` and zero-allocation byte-level processing for Serial data.
- **Structured Logging**: Configurable log levels (`debug`, `info`, `warn`, `error`) using Go's modern `slog` package.
- **Dockerized**: Ready to run in a lightweight container.

## Quick Start

The easiest way to run the emulator is using Docker.

1.  **Create a `config.yaml`** with your power sources:

    ```yaml
    log_level: info
    zero_fallback: true  # Recommended for stable regulation during sensor glitches
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
      - type: serial
        port_name: /dev/ttyUSB0
        baud_rate: 9600
        payload: SML
        label: Power
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
- `log_level`: Verbosity of the logs. Set to `debug` to see raw power fetches.
- `device`: The type of device to emulate (currently only `shellypro3em` supported).
- `device_id`: The source ID reported in JSON-RPC responses.
- `zero_fallback`: (Default: `false`)
    - If `true`: Returns 0W to the battery if a fetch fails. This signals "zero deviation," causing the battery to **hold its current output** stable.
    - If `false`: The emulator remains silent (UDP timeout) on errors. This may cause the battery to stop regulation or go to a safety state after a few seconds.

#### Common Provider Options
- `throttle`: (Optional) Minimal interval (in seconds) between fetches for pull-based providers (Tasmota). `GetPower` will block until this interval has passed to ensure fresh data and avoid overwhelming the source. **Set to `2.0` for Tasmota.** MQTT and Serial providers are push-based and do not use this setting as they naturally wait for incoming data.

#### Provider Options (Tasmota)
- `ip`: IP address of the Tasmota device.
- `user`/`password`: (Optional) For HTTP authentication.
- `status`: (Default: `StatusSNS`) JSON key for status.
- `payload`: (Default: `SML`) JSON key for the sensor payload.
- `label`: (Default: `Power`) JSON key for the power value (when `calculate` is `false`).
- `calculate`: If `true`, calculates net power using `label_in` and `label_out`.
- `label_in`: JSON key for imported power (required if `calculate: true`).
- `label_out`: JSON key for exported power (required if `calculate: true`).
- `json_path`: (Optional) [GJSON path](https://github.com/tidwall/gjson) to extract the power value. Overrides `status`, `payload`, and `label`.
- `json_path_in`: (Optional) GJSON path for imported power. Overrides `status`, `payload`, and `label_in`.
- `json_path_out`: (Optional) GJSON path for exported power. Overrides `status`, `payload`, and `label_out`.

#### Provider Options (Mock)
- `power`: Static power value in Watts.

#### Provider Options (MQTT)
- `broker`: Hostname or IP of the MQTT broker.
- `port`: Port of the MQTT broker (usually `1883`).
- `topic`: MQTT topic to subscribe to.
- `user`/`password`: (Optional) For MQTT authentication.
- `json_path`: (Optional) [GJSON path](https://github.com/tidwall/gjson) to extract the power value from a JSON payload. If omitted, the raw payload is parsed as a float.

#### Provider Options (Serial)
- `port_name`: The path to the USB/Serial port (e.g., `/dev/ttyUSB0`).
- `baud_rate`: (Default: `9600`) Baud rate for the serial connection.
- `payload`: (Default: `SML`) JSON key for the sensor payload.
- `label`: (Default: `Power`) JSON key for the power value.

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

This project was inspired by [tomquist/b2500-meter](https://github.com/tomquist/b2500-meter). This Go implementation focuses on lower latency, native concurrency, and a strict "No-Cache" policy for improved regulation stability.

## How it works

The Marstek B2500 battery expects a Shelly Pro 3EM on the local network. This emulator listens for the battery's UDP broadcast requests and responds with formatted JSON-RPC messages.

The emulator handles the specific rounding and "decimal point enforcement" (e.g., adding 0.001 to integers) required for the battery firmware. It uses a **Synchronous Blocking Strategy**: by waiting for a fresh measurement before responding to the battery's request, it synchronizes the battery's internal PI-regulator with the actual update rate of your power meter. This eliminates "sawtooth" patterns and minimizes grid leakage.