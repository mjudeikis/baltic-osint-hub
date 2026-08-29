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
    `AISSTREAM_API_KEY`);
  - **Satellite change detection** — Sentinel-1 SAR over monitored sites
    (Kaliningrad garrisons, Belarusian air bases and training grounds, rail
    and border chokepoints; see `internal/layers/aoi.go`). Runs at most daily.
- **Server** (`cmd/server`) exposes `/api/incidents`, `/api/stats/timeline`,
  `/api/stats/summary`, `/api/sources`, `/api/meta`, `/api/layers/*` and serves
  the built frontend. Responses carry `Cache-Control: max-age=300` for CDN caching.
- **Frontend** (`web/`) — React + Recharts + MapLibre: per-country threat board,
  stacked daily trend, situation map with togglable signal layers, satellite
  change-detection panel, filterable feed.

### How SAR change detection works

The Copernicus **Statistical API** does the raster work server-side: an
evalscript converts VV backscatter to dB and marks pixels above −5 dB, and the
service returns the *bright-pixel fraction* per 6-day interval — a proxy for
metallic scatterers (vehicles, aircraft, rolling stock). Nothing decodes
imagery locally.

Two choices keep false positives down: only **descending** passes are used, so
incidence angle stays comparable (mixing orbit directions is the classic way to
manufacture anomalies), and the verdict uses a **median + MAD** baseline over
180 days rather than mean/stdev, so occasional wild passes don't mask real
change. A site is flagged when the newest pass is ≥3σ *and* ≥1 percentage point
above its own baseline, with ≥8 prior observations. Only rises are flagged.

This finds *"something changed here, go look"* — it is not object detection.
Weather, farm machinery and construction move the same number, so every AOI
card deep-links to the Copernicus Browser for human verification.

The first run backfills 180 days of history, so baselines are usable
immediately rather than after weeks of accumulation. Observed medians line up
with what the sites are: Brest rail yard ~10% bright pixels (rolling stock),
Baranavichy air base ~7%, rural border crossings ~0.9% (fields and forest).

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
| `COPERNICUS_CLIENT_ID` / `COPERNICUS_CLIENT_SECRET` | — | Sentinel-1 SAR change detection ([free OAuth client](https://dataspace.copernicus.eu)) |
| `LISTEN_ADDR` | `:8080` | server bind address |
| `STATIC_DIR` | — | built frontend dir |

## Deploy (k8s + Cloudflare Tunnel)

```sh
kubectl create ns osint
kubectl -n osint create secret generic baltic-osint-hub --from-env-file=.env
helm install osint deploy/helm/baltic-osint-hub -n osint \
  --set route.enabled=true --set route.hostnames[0]=just-bob-club.xyz
```

The secret is injected wholesale (`envFrom`), so adding a key to `.env` and
re-creating the secret is all it takes for new credentials. The chart
overrides `DATABASE_URL` and `STATIC_DIR` from a copied local `.env` (unless
`postgres.enabled=false`, where the secret's `DATABASE_URL` is used).

`route.enabled=true` renders a Gateway API `HTTPRoute` attaching the ClusterIP
Service to a Cloudflare Tunnel gateway (`route.gateway`, default
`cfgate-system/cloudflare-tunnel`). The [cfgate](https://github.com/inherent-design/cfgate)
controller then adds the tunnel ingress rule and the proxied `CNAME` to
`<tunnel-id>.cfargotunnel.com` — no per-host tunnel config by hand. The
hostname's zone must be listed in the cluster's `CloudflareDNS` resource, e.g.:

```sh
kubectl -n cfgate-system patch cloudflaredns faros-sh --type=merge \
  -p '{"spec":{"zones":[{"name":"faros.sh","proxied":true},{"name":"just-bob-club.xyz","proxied":true}]}}'
```

Leave `route.enabled=false` to expose it some other way — the Service is
reachable at `http://osint-baltic-osint-hub.osint.svc:8080`.

The chart ships a single-node Postgres (`postgres.enabled=true`, 5Gi PVC). For
an external database set `postgres.enabled=false` and add `DATABASE_URL`
to the secret. Images build to `ghcr.io/mjudeikis/baltic-osint-hub` via GitHub
Actions on push to `main`.

## Disclaimer

Classification is automated and may contain errors. The dashboard aggregates
*publicly reported* events and links every item to its original source; it is
not an official threat assessment.
