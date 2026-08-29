# Baltic OSINT Hub

**Live: [osintbaltic.com](https://osintbaltic.com)**

Public dashboard tracking open-source intelligence on hybrid threats against
Lithuania, Latvia, Estonia, and Poland: sabotage, GPS jamming, cyberattacks,
disinformation, airspace and border incidents, espionage, and military activity.

## How it works

```
RSS + GDELT + Telegram + Reddit + Bluesky ──► collector (CronJob, hourly) ──► Postgres ──► server ──► dashboard
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
- **Event clustering** (`internal/cluster`) then groups reports of the *same*
  incident. The title hash only catches verbatim syndication; clustering
  embeds each item's English summary and merges reports within ±72 hours that
  share a category and a country. **Every count on the dashboard is a count of
  events, not articles** — without this, one story carried by six outlets moved
  the posture reading six times. Corroboration falls out of it: an event's
  confidence is set from how many *independent* outlets carried it, with
  state-controlled sources counted as evidence but never as corroboration.
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
  - **AIS archive** — Finnish Digitraffic positions inside the cable corridors,
    polled every 15 min (CC BY 4.0, no key). aisstream is realtime-only, so
    this is the only track history there is. Coverage is the Finnish network:
    good in the Gulf of Finland, partial in the central Baltic, **none in
    NordBalt** — an empty NordBalt corridor means no data, not a quiet one;
  - **Sanctioned vessels** — OpenSanctions maritime, refreshed daily, joined to
    sea events by MMSI so a loitering vessel can be named as shadow-fleet or
    sanctioned rather than just "a vessel" (CC BY-NC);
  - **CERT.PL warning list** — daily count of domains added to Poland's
    phishing blocklist as a cyber-activity rate. Counts only, never the
    domains; Poland only, since no equivalent open feed exists for LT/LV/EE;
  - **Satellite change detection** — Sentinel-1 SAR over monitored sites
    (Kaliningrad garrisons, Belarusian air bases and training grounds, rail
    and border chokepoints; see `internal/layers/aoi.go`). Runs at most daily.
- **Server** (`cmd/server`) exposes `/api/incidents`, `/api/stats/timeline`,
  `/api/stats/summary`, `/api/stats/posture`, `/api/sources`, `/api/meta`,
  `/api/layers/*`, `/api/stats/posture/history`, `/api/history/{YYYY-MM-DD}`,
  plus `/api/incidents.csv` and `/api/incidents.geojson`, and
  serves the built frontend. Responses carry `Cache-Control: max-age=60` and
  `Access-Control-Allow-Origin: *` — the API is read-only and public, so it is
  usable from anyone else's page.
- **Frontend** (`web/`) — React + Recharts + MapLibre: per-country threat board,
  stacked daily trend, situation map with togglable signal layers, satellite
  change-detection panel, filterable feed.

### Cadence

The CronJob runs **hourly** (`collector.schedule`), but every source and layer
enforces its own minimum interval internally, keyed off its last *successful*
run. Changing the schedule therefore shifts worst-case staleness without
multiplying calls to rate-limited upstreams; a failing source still retries on
the next invocation.

| Source / layer | Minimum interval |
|---|---|
| News RSS, Telegram, GDELT, Bluesky | 30 min |
| Reddit (serialised, 10 s apart) | 1 h |
| Think tanks, CERT, MoD feeds | 6 h |
| OpenSky air snapshots | 30 min |
| FIRMS thermal | 2 h |
| gpsjam | 6 h (stores per day) |
| Sentinel-1 SAR | 20 h |

LLM spend tracks item volume, not schedule: deduplication means each item is
classified exactly once regardless of how often the collector runs.

### Source credibility

Russian and Belarusian state outlets (TASS, RIA, Interfax, Zvezda, BelTA,
Rybar, Baltnews, the Russian MoD channel) are ingested **deliberately** — the
narrative aimed at the region is itself intelligence. But they are never
presented as reporting: every item carries a credibility class
(`institutional` / `independent` / `state-controlled`), state-controlled items
are visibly marked in the feed, and they are **excluded from the posture
calculation** so an adversary cannot move the dashboard's own threat gauge.
Exiled Russian outlets (Meduza, Astra, The Insider, Mediazona) are classified
independent, not state-controlled — the distinction is the point of the field.

### Tone and regional posture

Every classified item carries a **tone** as well as a severity — favourable,
neutral or adverse *for the security of the region*, judged independently of
how consequential it is. A NATO reinforcement is severity 3-4 and favourable; a
successful arson attack is severity 4 and adverse; an arrest of a saboteur is
favourable even though its subject is sabotage.

That feeds a single **regional posture** reading (Calm → Watchful → Elevated →
High → Severe, ascending; deliberately not DEFCON numbering, which counts
down). Levels 4 and 5 are set by absolute adverse severity and cannot be
softened by good news, while a week with more favourable than adverse
developments steps the middle of the scale down one. The banner always shows
the counts it was derived from, so the reading is auditable rather than a vibe.

The banner also answers **"is this week unusual?"** against the median of the
trailing twelve weeks, and the dashboard links each country's official civil
preparedness guidance (LT72, 72 stundas, kriis.ee, RCB). Reporting a threat
without telling the reader what to do with it is how a monitor becomes a
source of anxiety rather than readiness.

This exists because a threat-only feed reads as uniformly dire even in an
ordinary week — the first live reading was 29 favourable developments against
6 adverse ones, which the old view rendered as four countries in the red.

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

### Tests

`go test ./...` runs everything that needs no credentials. Two suites are
opt-in because they need a real database or a real API key:

```sh
# SQL against a THROWAWAY database — these TRUNCATE, so never point them at a
# database you care about, least of all production.
createdb -h localhost -p 5433 -U osint osint_test
TEST_DATABASE_URL=postgres://osint:osint@localhost:5433/osint_test go test ./...

# Clustering calibration and the end-to-end pass (a fraction of a cent).
OPENAI_API_KEY=sk-... go test ./internal/cluster/ -run Calibration -v
```

The calibration test is the one that fixes `CLUSTER_THRESHOLD` — re-run it
after changing the embedding model, the dimension count, or the threshold.
It exists because the threshold was originally set by reasoning to a value
that, when finally measured, merged nothing at all.

## Configuration (env)

| Variable | Default | Purpose |
|---|---|---|
| `DATABASE_URL` | — (required) | Postgres connection string |
| `OPENAI_API_KEY` | — | enables LLM classification |
| `OPENAI_BASE_URL` | `https://api.openai.com/v1` | override for compatible gateways |
| `ENRICH_MODEL` | `gpt-5-mini` | classification model |
| `MAX_ENRICH_PER_RUN` | `300` | cost guard per collector run |
| `MAX_CLUSTER_PER_RUN` | `1000` | incidents embedded and clustered per run |
| `CLUSTER_THRESHOLD` | `0.70` | cosine similarity at which two reports are one event; higher merges less |
| `FIRMS_MAP_KEY` | — | NASA FIRMS thermal layer ([free key](https://firms.modaps.eosdis.nasa.gov/api/map_key/)) |
| `OPENSKY_CLIENT_ID` / `OPENSKY_CLIENT_SECRET` | — | OpenSky OAuth2; anonymous fallback |
| `AISSTREAM_API_KEY` | — | sea-activity watch ([free key](https://aisstream.io)) |
| `AIS_ARCHIVE_MINUTES` | `15` | how often the server archives AIS positions from Digitraffic (no key) |
| `COPERNICUS_CLIENT_ID` / `COPERNICUS_CLIENT_SECRET` | — | Sentinel-1 SAR change detection ([free OAuth client](https://dataspace.copernicus.eu)) |
| `LISTEN_ADDR` | `:8080` | server bind address |
| `STATIC_DIR` | — | built frontend dir |

## Self-hosting (k8s + Cloudflare Tunnel)

Sections are ordered and each assumes the previous one has been applied. Nothing
below assumes an existing cluster add-on — if your cluster already runs cfgate
and a tunnel (check with `kubectl get crd | grep cfgate.io`), skip to
[Deploy the chart](#4-deploy-the-chart) and just add your zone.

Requirements: a Kubernetes cluster with a default StorageClass (k3s, kind with
the local-path provisioner, any managed cluster), `kubectl`, `helm` 3, and a
domain you control.

The public instance runs on **`osintbaltic.com`**, which is what the chart's
`route.hostnames` defaults to and what every command below uses. Substitute your
own domain throughout if you are deploying your own instance.

### 1. Cloudflare zone

A *zone* is a domain Cloudflare serves DNS for. You need one before anything
else; the tunnel controller writes records into it.

1. Register the domain (any registrar; Cloudflare Registrar is fine).
2. Add it to Cloudflare — **Dashboard → Add a domain**, pick the Free plan.
3. Cloudflare assigns two nameservers (e.g. `xxx.ns.cloudflare.com`). Set them
   at your registrar, replacing whatever is there.
4. Wait for the zone to flip to **Active** (minutes to a few hours). Verify:

```sh
dig +short NS osintbaltic.com          # must return the *.ns.cloudflare.com pair
```

The same thing over the API, if you'd rather not click:

```sh
export CF_API_TOKEN=<token with Account: Zone: Edit>
export CF_ACCOUNT_ID=<your account id>

curl -sX POST https://api.cloudflare.com/client/v4/zones \
  -H "Authorization: Bearer $CF_API_TOKEN" -H 'Content-Type: application/json' \
  -d "{\"name\":\"osintbaltic.com\",\"account\":{\"id\":\"$CF_ACCOUNT_ID\"},\"type\":\"full\"}" \
  | jq '.result.name_servers'            # then set these at the registrar
```

Leave the zone's DNS empty — cfgate creates the records. Do **not** pre-create an
A/CNAME for the hostname you are about to route; a conflicting record makes the
sync fail.

### 2. API token for the cluster

Create a token under **My Profile → API Tokens → Create Token → Custom** with:

| Scope | Permission | Needed for |
|---|---|---|
| Account | `Cloudflare Tunnel: Edit` | creating/managing the tunnel |
| Account | `Account Settings: Read` | account lookup (optional if you set `accountId`) |
| Zone | `DNS: Edit` | writing the hostname records |

Restrict it to the one account and the zones you intend to publish. Only the two
Access permissions (`Access: Apps and Policies: Edit`, `Access: Service Tokens:
Edit`) are extra, and only if you later put Cloudflare Access in front of the
dashboard — this app is public, so skip them.

### 3. Tunnel ingress (cfgate)

[cfgate](https://github.com/cfgate/cfgate) turns Gateway API `HTTPRoute`s into
Cloudflare Tunnel ingress rules plus DNS records, so publishing a service is one
Kubernetes object and no `cloudflared` config file. Install the Gateway API CRDs
first — some distros (k3s with traefik-crd) already ship them, most don't:

```sh
kubectl get crd gateways.gateway.networking.k8s.io >/dev/null 2>&1 || \
  kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.5.1/standard-install.yaml

kubectl apply -f https://github.com/cfgate/cfgate/releases/latest/download/install.yaml
kubectl -n cfgate-system rollout status deploy/cfgate-controller-manager

kubectl -n cfgate-system create secret generic cloudflare-credentials \
  --from-literal=CLOUDFLARE_API_TOKEN="$CF_API_TOKEN"
```

Then the tunnel, the gateway it backs, and the zone list. `CloudflareTunnel`
creates the tunnel in Cloudflare if it doesn't exist — there is nothing to
pre-provision in the dashboard:

```sh
cat <<EOF | kubectl apply -f -
apiVersion: cfgate.io/v1alpha1
kind: CloudflareTunnel
metadata:
  name: osint
  namespace: cfgate-system
spec:
  tunnel:
    name: osint
  cloudflare:
    accountId: "$CF_ACCOUNT_ID"
    secretRef:
      name: cloudflare-credentials
  cloudflared:
    replicas: 2
---
apiVersion: gateway.networking.k8s.io/v1
kind: GatewayClass
metadata:
  name: cfgate
spec:
  controllerName: cfgate.io/cloudflare-tunnel-controller
---
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: cloudflare-tunnel
  namespace: cfgate-system
  annotations:
    cfgate.io/tunnel-ref: cfgate-system/osint
spec:
  gatewayClassName: cfgate
  listeners:
    - name: http
      protocol: HTTP
      port: 80
      allowedRoutes:
        namespaces:
          from: All        # lets the osint namespace attach its route
---
apiVersion: cfgate.io/v1alpha1
kind: CloudflareDNS
metadata:
  name: zones
  namespace: cfgate-system
spec:
  tunnelRef:
    name: osint
    namespace: cfgate-system
  zones:
    - name: osintbaltic.com
      proxied: true
  defaults:
    proxied: true
  source:
    gatewayRoutes:
      enabled: true        # discover hostnames from HTTPRoutes
EOF
```

Check it landed — every condition should be `True`, and the gateway should carry
a `<tunnel-id>.cfargotunnel.com` address:

```sh
kubectl -n cfgate-system get cloudflaretunnel osint
kubectl -n cfgate-system get gateway cloudflare-tunnel
kubectl -n cfgate-system get cloudflaredns zones -o jsonpath='{.status.conditions[?(@.type=="ZonesResolved")].message}'
```

`CloudflareDNS` sitting at `Ready=Unknown / NoHostnamesDiscovered` before the
first `HTTPRoute` exists is normal, not a failure.

**Adding a zone to an existing install** — if the cluster already has a
`CloudflareDNS` (say from another environment), append to it instead of creating
a second one:

```sh
kubectl -n cfgate-system patch cloudflaredns <name> --type=merge \
  -p '{"spec":{"zones":[{"name":"existing.example","proxied":true},{"name":"osintbaltic.com","proxied":true}]}}'
```

`spec.zones` is a full replacement — list every zone you want kept, or the
omitted ones stop syncing.

### 4. Deploy the chart

```sh
kubectl create ns osint
kubectl -n osint create secret generic baltic-osint-hub --from-env-file=.env
helm install osint deploy/helm/baltic-osint-hub -n osint \
  --set route.enabled=true --set 'route.hostnames[0]=osintbaltic.com'
```

The secret is injected wholesale (`envFrom`), so adding a key to `.env` and
re-creating the secret is all it takes for new credentials. The chart
overrides `DATABASE_URL` and `STATIC_DIR` from a copied local `.env` (unless
`postgres.enabled=false`, where the secret's `DATABASE_URL` is used).

`route.enabled=true` renders the `HTTPRoute` attaching the ClusterIP Service to
`route.gateway` (default `cfgate-system/cloudflare-tunnel`). cfgate then adds the
tunnel ingress rule and the proxied `CNAME` to `<tunnel-id>.cfargotunnel.com`.
`route.hostnames` already defaults to `osintbaltic.com`, so the `--set` above is
only needed when publishing under a different domain.
Leave it `false` to expose the app some other way — the Service is reachable
in-cluster at `http://osint-baltic-osint-hub.osint.svc:8080`.

The chart ships a single-node Postgres (`postgres.enabled=true`, 5Gi PVC). For an
external database set `postgres.enabled=false` and add `DATABASE_URL` to the
secret.

Verify:

```sh
kubectl -n osint rollout status deploy/osint-baltic-osint-hub
kubectl -n osint get httproute osint-baltic-osint-hub \
  -o jsonpath='{.status.parents[0].conditions[*].type}{"\n"}'   # Accepted ResolvedRefs
kubectl -n cfgate-system get cloudflaredns zones \
  -o jsonpath='{range .status.records[*]}{.hostname}{"\t"}{.status}{"\n"}{end}'
curl -sI https://osintbaltic.com/healthz
```

A brand-new hostname can serve TLS handshake failures (SSL alert 40) for a few
minutes. The apex and one level of subdomain are covered by Universal SSL; a
*second*-level name (`osint.sub.example.com`) is outside that wildcard and only
works once Total TLS issues a per-hostname certificate.

### 5. Rolling out a new image

CI builds on every push to `main` and pushes two tags to
`ghcr.io/mjudeikis/baltic-osint-hub`: `latest` and the commit `:<sha>`.

The chart defaults to `image.tag=latest` with `pullPolicy: Always`, so a rollout
is a restart — Helm has nothing to change:

```sh
kubectl -n osint rollout restart deploy/osint-baltic-osint-hub
kubectl -n osint rollout status deploy/osint-baltic-osint-hub
```

The collector `CronJob` needs no restart; its next run (hourly) pulls `latest`
by itself. To pick the new image up immediately, trigger a run by hand — see
[Ad-hoc collector run](#6-ad-hoc-collector-run).

Pin an exact build instead — preferable for anything you care about, since it
makes the running version visible and rollback a one-liner:

```sh
helm upgrade osint deploy/helm/baltic-osint-hub -n osint --reuse-values \
  --set image.tag=$(git rev-parse HEAD)
```

Chart or values changed:

```sh
helm upgrade osint deploy/helm/baltic-osint-hub -n osint \
  --set route.enabled=true --set 'route.hostnames[0]=osintbaltic.com'

helm -n osint history osint          # then: helm -n osint rollback osint <rev>
```

Watch what actually landed:

```sh
kubectl -n osint get pods -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.spec.containers[0].image}{"\n"}{end}'
kubectl -n osint logs -l app.kubernetes.io/component=server --tail=50
```

Migrations are embedded and applied at startup by whichever binary connects
first (`internal/store/store.go`, tracked in `schema_migrations`), so a rollout
applies them. They are forward-only: rolling back to an image older than an
applied migration is not supported.

### 6. Ad-hoc collector run

The collector only runs on its schedule (hourly by default). To force a
fetch+classify cycle now — after a deploy, after adding a source, or to pick up
a freshly pushed image without waiting — clone the `CronJob` into a one-off
`Job`:

```sh
kubectl create job <new-job-name> --from=cronjob/<existing-cronjob-name> -n <namespace>
```

For this chart, with release `osint` in namespace `osint`:

```sh
kubectl -n osint create job collector-manual-$(date +%s) \
  --from=cronjob/osint-baltic-osint-hub-collector

kubectl -n osint get jobs --sort-by=.metadata.creationTimestamp
kubectl -n osint logs -f job/<job-name>
```

The Job copies the CronJob's pod template as it exists *at creation time*, so it
picks up the current `image.tag`, the current `baltic-osint-hub` secret, and
`pullPolicy: Always` — i.e. it pulls the newest `latest`. It also inherits
`activeDeadlineSeconds: 1500`, so a wedged run is killed after 25 minutes. The
name must be unique; the timestamp suffix handles that, whereas a plain
`collector-manual` fails the second time with `AlreadyExists`.

Two consequences of the CronJob `ownerReference` kubectl attaches:

- the manual Job counts as an active job for `concurrencyPolicy: Forbid`, so a
  scheduled run that comes due while it is still going is **skipped**, not
  queued;
- it is pruned by the same `successfulJobsHistoryLimit: 3`, so finished manual
  jobs clean themselves up. Delete one early by name if you want it gone sooner
  (`kubectl -n osint delete job <job-name>`).

Running one alongside a scheduled run is otherwise harmless: the collector
dedupes by URL and normalized-title hash and enforces per-source fetch intervals
internally, so an extra run largely no-ops instead of re-fetching. It does spend
OpenAI credit on whatever is genuinely new, bounded by `MAX_ENRICH_PER_RUN`.

## Disclaimer

Classification is automated and may contain errors. The dashboard aggregates
*publicly reported* events and links every item to its original source; it is
not an official threat assessment.
