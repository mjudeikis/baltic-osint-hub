# Baltic OSINT Hub

Public dashboard tracking open-source intelligence on hybrid threats against
Lithuania, Latvia, Estonia, and Poland: sabotage, GPS jamming, cyberattacks,
disinformation, airspace and border incidents, espionage, and military activity.

## How it works

```
RSS + GDELT + Telegram + Reddit + Bluesky ──► collector (CronJob, 30 min) ──► Postgres ──► server ──► dashboard
                          │
                          └─ OpenAI (gpt-5-mini) classifies each new item:
                             relevant? category, countries, severity 1–5,
                             English summary, location
```

- **Collector** (`cmd/collector`) fetches sources listed in
  `internal/sources/registry.go`:
  - news/institutional RSS (LRT, ERR, LSM, Notes from Poland, EUvsDisinfo,
    CERT.PL, CEPA, Jamestown, ICDS, Warsaw Institute, Lithuanian and Latvian
    MoD, Estonian Defence Forces) and GDELT;
  - public **Telegram** channels via the t.me/s preview: news/monitoring
    (meduzalive, astrapress, nexta_live), Belarus military-movement tracking
    (MotolkoHelp, belzhd_live — railway echelon sightings), and Russian
    military/propaganda channels monitored as primary sources of adversary
    messaging, not as trusted reporting (rybar, mod_russia, milinfolive);
  - **Reddit** regional subreddits via RSS (r/BalticStates, r/lithuania,
    r/latvia, r/Eesti, r/poland);
  - **Bluesky** keyword searches via the open AppView API.
  It dedupes by URL and normalized-title hash, keyword-prefilters (EN/LT/LV/ET/
  PL/RU), then batches new items through the OpenAI API for classification.
- **Signal layers** are machine measurements shown as map overlays, separate
  from the classified news feed:
  - **GPS jamming** — gpsjam.org daily H3 cells (share of aircraft reporting
    degraded navigation), no key needed;
  - **Thermal (FIRMS)** — NASA VIIRS thermal anomalies filtered to strips along
    the RU/BY borders (needs a free `FIRMS_MAP_KEY`);
  - **Air activity** — OpenSky snapshots of the border boxes; flags watchlist
    callsigns (RFF/RSD), emergency squawks, and RU/BY-registered aircraft
    (anonymous works; an OpenSky OAuth2 client raises the rate limits);
  - **Sea activity** — aisstream.io live AIS in the Baltic cable corridors;
    flags loitering (<1 kn for 30+ min) and AIS gaps (1h+ dark inside a
    corridor). Runs as a persistent stream inside the server (needs a free
    `AISSTREAM_API_KEY`).
- **Server** (`cmd/server`) exposes `/api/incidents`, `/api/stats/timeline`,
  `/api/stats/summary`, `/api/sources`, `/api/meta` and serves the built
  frontend. Responses carry `Cache-Control: max-age=300` for CDN caching.
- **Frontend** (`web/`) — React + Recharts + MapLibre: per-country threat board,
  stacked daily trend, incident map (severity-colored), filterable feed.

## Local development

```sh
docker compose up -d          # Postgres on :5433
cp .env.example .env          # then fill in OPENAI_API_KEY etc.
go run ./cmd/collector        # one fetch+classify cycle
cd web && npm install && npm run build && cd ..
go run ./cmd/server           # http://localhost:8080
```

Both binaries read `.env` from the working directory (real environment
variables take precedence); `.env` is git- and docker-ignored.

For frontend iteration run `npm run dev` in `web/` (proxies /api to :8080).

## Configuration (env)

| Variable | Default | Purpose |
|---|---|---|
| `DATABASE_URL` | — (required) | Postgres connection string |
| `OPENAI_API_KEY` | — | enables LLM classification |
| `OPENAI_BASE_URL` | `https://api.openai.com/v1` | override for compatible gateways |
| `ENRICH_MODEL` | `gpt-5-mini` | classification model |
| `MAX_ENRICH_PER_RUN` | `300` | cost guard per collector run |
| `ACLED_EMAIL` / `ACLED_PASSWORD` | — | myACLED credentials ([register free](https://acleddata.com/user/register)); fetcher lands in phase 2 |
| `FIRMS_MAP_KEY` | — | NASA FIRMS thermal layer ([free key](https://firms.modaps.eosdis.nasa.gov/api/map_key/)) |
| `OPENSKY_CLIENT_ID` / `OPENSKY_CLIENT_SECRET` | — | OpenSky OAuth2; anonymous fallback |
| `AISSTREAM_API_KEY` | — | sea-activity watch ([free key](https://aisstream.io)) |
| `LISTEN_ADDR` | `:8080` | server bind address |
| `STATIC_DIR` | — | built frontend dir |

## Deploy (k8s + Cloudflare Tunnel)

```sh
kubectl create ns osint
kubectl -n osint create secret generic baltic-osint-hub --from-env-file=.env
helm install osint deploy/helm/baltic-osint-hub -n osint
```

The secret is injected wholesale (`envFrom`), so adding a key to `.env` and
re-creating the secret is all it takes for new credentials. The chart
overrides `DATABASE_URL` and `STATIC_DIR` from a copied local `.env` (unless
`postgres.enabled=false`, where the secret's `DATABASE_URL` is used).

The chart ships a single-node Postgres (`postgres.enabled=true`, 5Gi PVC);
point your Cloudflare tunnel at `http://osint-baltic-osint-hub.osint.svc:8080`.
For an external database set `postgres.enabled=false` and add `DATABASE_URL`
to the secret. Images build to `ghcr.io/mjudeikis/baltic-osint-hub` via GitHub
Actions on push to `main`.

## Disclaimer

Classification is automated and may contain errors. The dashboard aggregates
*publicly reported* events and links every item to its original source; it is
not an official threat assessment.
