# Macro Service

Collects free macro and geopolitical news feeds, and accepts macro Agent
reports. The service keeps discovery separate from verification, enforces the
`macro-risk.v1` report contract, and rejects any position posture marked
executable or not requiring human approval.

## Run

```bash
go run ./cmd/server
```

Configuration:

| Variable | Default |
| --- | --- |
| `HTTP_ADDR` | `:8081` |
| `NEWS_SOURCES_FILE` | Embedded `internal/news/default-sources.json` |
| `NEWS_MAX_ITEMS` | `5000` |
| `NEWS_RETENTION_HOURS` | `72` |
| `NEWS_USER_AGENT` | Project URL-based user agent |

## API

- `GET /v1/news?limit=100&tier=A&source_id=federal-reserve&since=...` lists
  collected news.
- `GET /v1/news/sources` returns source configuration, freshness, and errors.
- `POST /v1/news/refresh` refreshes sources whose polling interval has elapsed.
- `POST /v1/reports` stores an immutable report by `report_id`.
- `GET /v1/reports/{id}` returns a report.
- `GET /v1/reports/latest?asset=ETH` returns the newest matching report.
- `GET /v1/signals/latest?asset=ETH&horizon=1d` filters its signals.
- `GET /healthz` reports process health.

## Default News Sources

| Tier | Sources | Use |
| --- | --- | --- |
| `A` | Federal Reserve, ECB, BEA, SEC, UN News | Primary or authoritative evidence |
| `B` | BBC World, Al Jazeera | Independent corroboration |
| `D` | Google News search, GDELT | Discovery only |

GDELT is polled every 15 minutes and never more frequently than five minutes.
Other feeds have source-specific intervals between 30 seconds and five
minutes. Failures are isolated per source and exposed through
`GET /v1/news/sources`.

To replace the source matrix, point `NEWS_SOURCES_FILE` at a JSON file with the
same structure as `internal/news/default-sources.json`. Aggregator entries must
remain discovery-tier unless the underlying publisher is independently
verified.

X and Telegram are not automated because their free programmatic access is not
stable enough for this service. They may be used manually to discover an event,
but not as sole confirmation.

The embedded contract is
`internal/report/macro-risk-report.schema.json`. The root validation target
checks that it remains byte-identical to the Agent contract in `.trae`.
News and report persistence are currently in memory.
