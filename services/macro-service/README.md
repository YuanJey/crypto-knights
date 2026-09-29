# Macro Service

Accepts and exposes macro Agent reports. The service enforces the
`macro-risk.v1` envelope and rejects any position posture marked executable or
not requiring human approval.

## Run

```bash
go run ./cmd/server
```

Configuration:

| Variable | Default |
| --- | --- |
| `HTTP_ADDR` | `:8081` |

## API

- `POST /v1/reports` stores an immutable report by `report_id`.
- `GET /v1/reports/{id}` returns a report.
- `GET /v1/reports/latest?asset=ETH` returns the newest matching report.
- `GET /v1/signals/latest?asset=ETH&horizon=1d` filters its signals.
- `GET /healthz` reports process health.

The embedded contract is
`internal/report/macro-risk-report.schema.json`. The root validation target
checks that it remains byte-identical to the Agent contract in `.trae`.
Persistence is currently in memory.
