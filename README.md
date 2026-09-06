# Baltic OSINT Hub

**Live: [osintbaltic.com](https://osintbaltic.com)**

Public dashboard tracking open-source intelligence on hybrid threats against
Lithuania, Latvia, Estonia, and Poland: sabotage, GPS jamming, cyberattacks,
disinformation, airspace and border incidents, espionage, and military
activity. Every count is a count of *events*, not articles; every reading says
what it was derived from; adversary media is ingested but marked and never
allowed to move the gauge. The reasoning behind those rules is in
[docs/methodology.md](docs/methodology.md).

## Architecture

```
news RSS, GDELT, Google News, Telegram, Reddit, Bluesky ─┐
signal layers: gpsjam, FIRMS, OpenSky, CERT.PL,          ├─► collector (CronJob, hourly) ──► Postgres ──► server ──► dashboard + API
               OpenSanctions, Sentinel-1 SAR             ┘         │                            ▲
                                                                   └─ OpenAI gpt-5-mini:        │  live AIS (aisstream.io) and the
                                                                      relevant? category,       │  AIS archive (Digitraffic) run
                                                                      countries, severity, tone,│  inside the server process
                                                                      EN summary → embed → cluster
```

- **Collector** (`cmd/collector`) fetches roughly 80 sources listed in
  `internal/sources/registry.go`: national press in English and in the
  native languages (LRT, ERR, LSM, Delfi, Postimees, 15min, TVN24,
  Rzeczpospolita, …) plus Finnish and Swedish press for the Baltic Sea half;
  institutional feeds — the Lithuanian and Latvian MoD and the Estonian
  Defence Forces are read in their **native languages** (both ministries
  dropped their English feeds in mid-2026; the classifier translates), plus
  EUvsDisinfo, CERT.PL and EU press; think tanks (CEPA, Jamestown, ICDS,
  Warsaw Institute, OSW); GDELT; **targeted Google News queries** in
  EN/LT/LV/ET/PL/RU, two of which exist specifically to surface arrests and
  reinforcement so the feed is not threat-only; exiled Russian press; public
  Telegram channels (news, Belarus military-movement trackers, and Russian
  MoD/propaganda channels monitored as adversary messaging, not reporting);
  regional subreddits and Bluesky keyword searches. It dedupes,
  keyword-prefilters, classifies new items through OpenAI, then embeds and
  clusters reports of the same incident into one event.
- **Signal layers** (`internal/layers`) are machine measurements shown as map
  overlays, never passed through the LLM: GPS jamming (gpsjam), thermal
  anomalies (NASA FIRMS), air activity (OpenSky), sea activity and AIS gaps in
  the cable corridors (aisstream.io + Digitraffic), sanctioned vessels
  (OpenSanctions), CERT.PL phishing-list rate, and Sentinel-1 SAR change
  detection over a watchlist of sites.
- **Server** (`cmd/server`) serves the API and the built frontend, and hosts
  the two persistent AIS loops — which is why it runs as a single replica.
- **Frontend** (`web/`) — React + Recharts + MapLibre: per-country threat
  board, daily trend, situation map with togglable layers, SAR panel,
  filterable feed.

## Quickstart (local development)

```sh
docker compose up -d          # Postgres on :5433
cp .env.example .env          # then fill in OPENAI_API_KEY etc.
go run ./cmd/collector        # one fetch+classify cycle
cd web && npm install && npm run build && cd ..
go run ./cmd/server           # http://localhost:8080
```

Both binaries read `.env` from the working directory (real environment
variables take precedence); `.env` is git- and docker-ignored. For frontend
iteration run `npm run dev` in `web/` (proxies `/api` to :8080).

`make help` lists the same steps as targets: `make dev`, `make dev-web`,
`make collect`, `make test`, `make test-db`, `make lint`, `make helm-lint`.
See [CONTRIBUTING.md](CONTRIBUTING.md) for the test suites and PR flow.

### Configuration

Every variable read by `internal/config/config.go`:

| Variable | Default | Purpose |
|---|---|---|
| `DATABASE_URL` | — (required) | Postgres connection string |
| `OPENAI_API_KEY` | — | enables LLM classification and clustering embeddings; empty = fetch-only |
| `OPENAI_BASE_URL` | `https://api.openai.com/v1` | override for compatible gateways |
| `ENRICH_MODEL` | `gpt-5-mini` | classification model |
| `MAX_ENRICH_PER_RUN` | `300` | cost guard: items classified per collector run |
| `MAX_CLUSTER_PER_RUN` | `1000` | incidents embedded and clustered per run |
| `CLUSTER_THRESHOLD` | `0.70` | cosine similarity at which two reports are one event; higher merges less |
| `RUN_TIMEOUT_MINUTES` | `50` | cap on one collector run; the chart's Job deadline (65 min) sits above it |
| `FIRMS_MAP_KEY` | — | NASA FIRMS thermal layer ([free key](https://firms.modaps.eosdis.nasa.gov/api/map_key/)) |
| `OPENSKY_CLIENT_ID` / `OPENSKY_CLIENT_SECRET` | — | OpenSky OAuth2; anonymous fallback with tight rate limits |
| `AISSTREAM_API_KEY` | — | live sea-activity watch, runs in the server ([free key](https://aisstream.io)) |
| `AIS_ARCHIVE_MINUTES` | `15` | how often the server archives Digitraffic AIS positions (no key) |
| `COPERNICUS_CLIENT_ID` / `COPERNICUS_CLIENT_SECRET` | — | Sentinel-1 SAR change detection ([free OAuth client](https://dataspace.copernicus.eu)) |
| `LISTEN_ADDR` | `:8080` | server bind address |
| `STATIC_DIR` | — | built frontend dir; empty disables static serving |

Test-only: `TEST_DATABASE_URL` enables the SQL suites (they `TRUNCATE`);
`LIVE_FEEDS=1` additionally runs the tests that hit real upstream feeds.

## Public API

Read-only, unauthenticated, CORS-enabled (`Access-Control-Allow-Origin: *`),
`Cache-Control: max-age=60`. Rate-limited per client IP at **10 requests/s
with a burst of 30**; over that you get `429`.

| Endpoint | Returns |
|---|---|
| `GET /api/incidents` | classified events. Filters: `category`, `country`, `tone`, `severity` (minimum), `since`/`until` (RFC 3339), `days`, `day=YYYY-MM-DD`, `limit`, `offset`. Default window **90 days** when no `since`/`days`/`day` is given; `limit` defaults to 100 and is clamped to 500. |
| `GET /api/incidents.csv`, `.geojson` | same filters, same rows. GeoJSON only includes located events. |
| `GET /api/stats/summary`, `/api/stats/timeline`, `/api/stats/posture`, `/api/stats/posture/history` | counts, daily buckets, the regional posture reading and its history |
| `GET /api/history/{YYYY-MM-DD}` | what the dashboard showed on that day |
| `GET /api/sources` | every source with its last successful run |
| `GET /api/meta` | categories, countries, and the full posture ladder |
| `GET /api/layers/*` | signal layers (jamming, thermal, air, sea, AIS archive, sanctions, CERT.PL, SAR) |
| `GET /healthz`, `GET /readyz` | liveness (process up) and readiness (database answers; `503` otherwise) |

Classification is retried up to **3 attempts** per item; an item that fails
all three is marked errored and left out rather than guessed. The posture
ladder behind `/api/stats/posture` is published at `/api/meta` and explained
in [docs/methodology.md](docs/methodology.md#regional-posture).

## Documentation

- [docs/methodology.md](docs/methodology.md) — how an item becomes a reading:
  sources, cadence, credibility, tone, posture, what the dashboard refuses to
  claim.
- [docs/self-hosting.md](docs/self-hosting.md) — Kubernetes deployment,
  Cloudflare Tunnel, rollouts, backups and restore.
- [docs/README.md](docs/README.md) — index of the rest (SAR detection,
  watchlist, the 2021–22 precedent).
- [CONTRIBUTING.md](CONTRIBUTING.md), [SECURITY.md](SECURITY.md).

## Disclaimer

Classification is automated and may contain errors. The dashboard aggregates
*publicly reported* events and links every item to its original source; it is
not an official threat assessment.
