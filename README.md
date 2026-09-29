# Crypto Knights

Crypto Knights is organized as independent services. Each service owns its Go
module, internal packages, API, tests, configuration, and container image.
There are no shared internal Go packages between services.

## Services

| Service | Directory | Default port | Responsibility |
| --- | --- | --- | --- |
| Macro | `services/macro-service` | `8081` | Accept and expose validated `macro-risk.v1` Agent reports |
| Options | `services/options-service` | `8082` | Ingest option trades and aggregate large-premium direction |
| Trigger | `services/trigger-service` | `8083` | Track price conditions and dispatch eligible entry triggers |
| Execution | `services/execution-service` | `8084` | Enforce execution policy and place idempotent paper orders |

The frontend dashboard is intentionally outside this Go backend phase.

## Local Development

Requirements:

- Go 1.23 or newer
- Docker Compose, only when running containers

Run all tests:

```bash
make test
```

Run all services locally with Go:

```bash
make dev
```

Wait for the four health checks, then use ports `8081` through `8084`. Press
`Ctrl+C` once to stop every service. Logs are written to `.run/logs`.

Run with Docker Compose:

```bash
make up
```

Stop Docker services with `make down`.

The execution service starts in `paper` mode. It refuses unknown exchange
modes; no live exchange adapter or credentials are included.

## Example Trigger

Create a rule that activates when ETH reaches `2600`, tracks the minimum for 60
seconds, and buys after a 10-point rebound:

```bash
curl -X POST http://localhost:8083/v1/rules \
  -H 'Content-Type: application/json' \
  -d '{
    "id": "eth-long-2600",
    "symbol": "ETHUSDT",
    "direction": "long",
    "activation_price": "2600",
    "retracement": "10",
    "window_seconds": 60,
    "quantity": "0.5"
  }'
```

Feed timestamped ticks:

```bash
curl -X POST http://localhost:8083/v1/ticks \
  -H 'Content-Type: application/json' \
  -d '{"symbol":"ETHUSDT","price":"2600","observed_at":"2026-09-29T12:00:00Z"}'

curl -X POST http://localhost:8083/v1/ticks \
  -H 'Content-Type: application/json' \
  -d '{"symbol":"ETHUSDT","price":"2588","observed_at":"2026-09-29T12:00:20Z"}'

curl -X POST http://localhost:8083/v1/ticks \
  -H 'Content-Type: application/json' \
  -d '{"symbol":"ETHUSDT","price":"2598","observed_at":"2026-09-29T12:00:40Z"}'
```

The final tick dispatches one idempotent paper market order. The trigger
service does not infer prices itself; production market-data adapters must call
its tick endpoint with exchange timestamps.

## Current Storage

This baseline uses in-memory repositories, so data does not survive restart.
The domain and HTTP boundaries are separated so PostgreSQL/Redis-backed
repositories and durable outbox delivery can be added without merging service
modules.
