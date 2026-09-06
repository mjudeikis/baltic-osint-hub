# Self-hosting (Kubernetes + Cloudflare Tunnel)

The operations runbook: how the public instance is deployed, and how to run
your own. Sections are ordered and each assumes the previous one has been
applied. If your cluster already runs cfgate and a tunnel (check with
`kubectl get crd | grep cfgate.io`), skip to [Deploy the chart](#4-deploy-the-chart)
and just add your zone.

Requirements: a Kubernetes cluster with a default StorageClass (k3s, kind with
the local-path provisioner, any managed cluster), `kubectl`, `helm` 3, and a
domain you control.

The public instance runs on **`osintbaltic.com`**, which is what the chart's
`route.hostnames` defaults to and what every command below uses. Substitute
your own domain throughout if you are deploying your own instance.

## What the chart deploys

| Object | Purpose |
|---|---|
| `Deployment` server (1 replica, `Recreate`) | API + static frontend. Runs the live AIS stream and the AIS archive poller in-process, so it must stay at one replica and cannot roll — see the comments in `values.yaml`. |
| `CronJob` collector (hourly) | Fetch, classify, cluster. `backoffLimit: 1`, killed after 65 minutes. |
| `StatefulSet` postgres + PVC | Single-node Postgres 17 (`postgres.enabled=true`). |
| `Secret` `<release>-baltic-osint-hub-postgres` | Chart-managed DB password and `DATABASE_URL`; kept on uninstall. |
| `CronJob` postgres-backup + PVC | Optional (`postgres.backup.enabled`), daily `pg_dump -Fc`. |
| `HTTPRoute` | Optional (`route.enabled`), attaches to a Gateway API gateway. |

All pods run non-root with a read-only root filesystem (Postgres excepted: it
needs `/var/run` and `/tmp`), all capabilities dropped, the `RuntimeDefault`
seccomp profile, and no service-account token mounted.

## 1. Cloudflare zone

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

Leave the zone's DNS empty — cfgate creates the records. Do **not** pre-create
an A/CNAME for the hostname you are about to route; a conflicting record makes
the sync fail.

## 2. API token for the cluster

Create a token under **My Profile → API Tokens → Create Token → Custom** with:

| Scope | Permission | Needed for |
|---|---|---|
| Account | `Cloudflare Tunnel: Edit` | creating/managing the tunnel |
| Account | `Account Settings: Read` | account lookup (optional if you set `accountId`) |
| Zone | `DNS: Edit` | writing the hostname records |

Restrict it to the one account and the zones you intend to publish. The two
Access permissions (`Access: Apps and Policies: Edit`, `Access: Service Tokens:
Edit`) are only needed if you later put Cloudflare Access in front of the
dashboard — this app is public, so skip them.

## 3. Tunnel ingress (cfgate)

[cfgate](https://github.com/cfgate/cfgate) turns Gateway API `HTTPRoute`s into
Cloudflare Tunnel ingress rules plus DNS records, so publishing a service is
one Kubernetes object and no `cloudflared` config file. Install the Gateway
API CRDs first — some distros (k3s with traefik-crd) already ship them, most
don't:

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

Check it landed — every condition should be `True`, and the gateway should
carry a `<tunnel-id>.cfargotunnel.com` address:

```sh
kubectl -n cfgate-system get cloudflaretunnel osint
kubectl -n cfgate-system get gateway cloudflare-tunnel
kubectl -n cfgate-system get cloudflaredns zones -o jsonpath='{.status.conditions[?(@.type=="ZonesResolved")].message}'
```

`CloudflareDNS` sitting at `Ready=Unknown / NoHostnamesDiscovered` before the
first `HTTPRoute` exists is normal, not a failure.

**Adding a zone to an existing install** — if the cluster already has a
`CloudflareDNS` (say from another environment), append to it instead of
creating a second one:

```sh
kubectl -n cfgate-system patch cloudflaredns <name> --type=merge \
  -p '{"spec":{"zones":[{"name":"existing.example","proxied":true},{"name":"osintbaltic.com","proxied":true}]}}'
```

`spec.zones` is a full replacement — list every zone you want kept, or the
omitted ones stop syncing.

## 4. Deploy the chart

The credentials Secret **must exist before `helm install`** — it is injected
wholesale (`envFrom`) and is no longer optional, so pods will not start
without it. That is deliberate: an optional secret turned a typo in the
secret name into a server that came up with no API keys and silently
collected nothing.

```sh
kubectl create ns osint
kubectl -n osint create secret generic baltic-osint-hub --from-env-file=.env
helm install osint deploy/helm/baltic-osint-hub -n osint \
  --set route.enabled=true --set 'route.hostnames[0]=osintbaltic.com'
```

Adding a key to `.env` and re-creating the secret is all it takes for new
credentials (then restart the server; the collector picks it up on its next
run). The chart overrides `DATABASE_URL` and `STATIC_DIR` from a copied local
`.env` unless `postgres.enabled=false`, where the secret's `DATABASE_URL` is
authoritative.

`route.enabled=true` renders the `HTTPRoute` attaching the ClusterIP Service to
`route.gateway` (default `cfgate-system/cloudflare-tunnel`). cfgate then adds
the tunnel ingress rule and the proxied `CNAME` to
`<tunnel-id>.cfargotunnel.com`. `route.hostnames` already defaults to
`osintbaltic.com`, so the `--set` above is only needed when publishing under
a different domain. Leave it `false` to expose the app some other way — the
Service is reachable in-cluster at `http://osint-baltic-osint-hub.osint.svc:8080`.

Verify:

```sh
kubectl -n osint rollout status deploy/osint-baltic-osint-hub
kubectl -n osint get httproute osint-baltic-osint-hub \
  -o jsonpath='{.status.parents[0].conditions[*].type}{"\n"}'   # Accepted ResolvedRefs
kubectl -n cfgate-system get cloudflaredns zones \
  -o jsonpath='{range .status.records[*]}{.hostname}{"\t"}{.status}{"\n"}{end}'
curl -sI https://osintbaltic.com/readyz      # 200 once the DB answers
```

A brand-new hostname can serve TLS handshake failures (SSL alert 40) for a few
minutes. The apex and one level of subdomain are covered by Universal SSL; a
*second*-level name (`osint.sub.example.com`) is outside that wildcard and only
works once Total TLS issues a per-hostname certificate.

### Database

The chart ships a single-node Postgres (`postgres.enabled=true`, 5Gi PVC,
requests 100m/256Mi, 1Gi memory limit). Its password lives in the
chart-managed Secret `<release>-baltic-osint-hub-postgres`, which also carries
the assembled `DATABASE_URL` the server and collector read. Leave
`postgres.password` empty and the chart generates a random one on first
install and re-reads the existing Secret on every upgrade, so upgrades never
rotate it. The Secret is annotated `helm.sh/resource-policy: keep`: it
survives `helm uninstall`, because the PVC does too and the data directory
only accepts the password it was initialised with.

**Upgrading an install from chart 0.1.x:** that chart hard-coded the password
`osint`. The data directory still expects it, so pass it once and the new
Secret is created with the right value:

```sh
helm upgrade osint deploy/helm/baltic-osint-hub -n osint --reuse-values \
  --set postgres.password=osint
```

Rotate afterwards if you want: `ALTER USER osint PASSWORD '...'` inside the
pod, then `helm upgrade --set postgres.password=<new>`, then restart the
server and let the collector's next run pick it up.

For an external database set `postgres.enabled=false` and put `DATABASE_URL`
in the `baltic-osint-hub` secret.

Pin the Postgres image for anything you care about — an exact patch
(`postgres.image=postgres:17.6-alpine`) or a digest — so a node re-pull cannot
move you across a minor release unannounced. Major-version upgrades are not
handled by the chart: dump, change the image and PVC, restore.

### Backups

Off by default. Enabling it adds a second PVC and a nightly CronJob running
`pg_dump -Fc`, keeping the newest `keep` files:

```sh
helm upgrade osint deploy/helm/baltic-osint-hub -n osint --reuse-values \
  --set postgres.backup.enabled=true \
  --set postgres.backup.schedule='30 3 * * *' \
  --set postgres.backup.keep=14 \
  --set postgres.backup.storage=5Gi
```

Take one now rather than waiting for 03:30:

```sh
kubectl -n osint create job backup-manual-$(date +%s) \
  --from=cronjob/osint-baltic-osint-hub-postgres-backup
```

The backup PVC is `<release>-baltic-osint-hub-postgres-backup` and is kept on
uninstall. It sits on the same cluster as the database, so copy dumps off
periodically (`kubectl cp` from a backup pod, or point your own sync at the
volume) — same-cluster backups protect against a bad migration or a fat
finger, not against losing the cluster.

**Restore.** Run a throwaway pod with both volumes, then `pg_restore` over the
existing database. Scale the server down first so nothing writes mid-restore;
the collector may also be running, so check `kubectl get jobs`.

```sh
kubectl -n osint scale deploy/osint-baltic-osint-hub --replicas=0

kubectl -n osint run pg-restore --rm -it --restart=Never --image=postgres:17-alpine \
  --overrides='{"spec":{"volumes":[{"name":"b","persistentVolumeClaim":{"claimName":"osint-baltic-osint-hub-postgres-backup"}}],
    "containers":[{"name":"pg-restore","image":"postgres:17-alpine","stdin":true,"tty":true,"command":["sh"],
    "volumeMounts":[{"name":"b","mountPath":"/backups"}],
    "env":[{"name":"PGHOST","value":"osint-baltic-osint-hub-postgres"},{"name":"PGUSER","value":"osint"},
           {"name":"PGPASSWORD","valueFrom":{"secretKeyRef":{"name":"osint-baltic-osint-hub-postgres","key":"POSTGRES_PASSWORD"}}}]}]}}'

# inside the pod:
ls -1t /backups
pg_restore --clean --if-exists --no-owner -d osint /backups/osint-<timestamp>.dump
exit

kubectl -n osint scale deploy/osint-baltic-osint-hub --replicas=1
```

`--clean --if-exists` drops and recreates objects so the restore is exact,
not a merge. Migrations are tracked in `schema_migrations` inside the dump, so
restoring an older dump onto a newer image re-applies the missing migrations
on the next start.

## 5. Rolling out a new image

CI builds on every push to `main` and pushes three tags to
`ghcr.io/mjudeikis/baltic-osint-hub`: `latest`, the full commit `:<sha>` and
`:sha-<short sha>`. Every image is scanned with Trivy before it is pushed.

**Recommended: pin the build.** It makes the running version visible in
`kubectl get pods`, makes rollback a one-liner, and switches the pull policy to
`IfNotPresent` automatically (the chart only pulls `Always` for `latest`):

```sh
helm upgrade osint deploy/helm/baltic-osint-hub -n osint --reuse-values \
  --set image.tag=$(git rev-parse HEAD)

helm -n osint history osint          # then: helm -n osint rollback osint <rev>
```

`--set image.tag=sha-$(git rev-parse --short=7 HEAD)` is the same thing with a
shorter tag; `--set image.digest=sha256:...` pins harder still.

**Alternative: ride `latest`.** With the default `image.tag=latest` a rollout
is just a restart, since Helm has nothing to change:

```sh
kubectl -n osint rollout restart deploy/osint-baltic-osint-hub
kubectl -n osint rollout status deploy/osint-baltic-osint-hub
```

Either way the server Deployment uses `Recreate`, so expect a few seconds of
downtime per rollout — the alternative is two pods streaming AIS at once. The
collector `CronJob` needs no restart; its next hourly run uses the new image.
To pick it up immediately, trigger a run by hand — see
[Ad-hoc collector run](#6-ad-hoc-collector-run).

Chart or values changed:

```sh
helm upgrade osint deploy/helm/baltic-osint-hub -n osint \
  --set route.enabled=true --set 'route.hostnames[0]=osintbaltic.com'
```

Watch what actually landed:

```sh
kubectl -n osint get pods -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.spec.containers[0].image}{"\n"}{end}'
kubectl -n osint logs -l app.kubernetes.io/component=server --tail=50
```

Migrations are embedded and applied at startup by whichever binary connects
first (`internal/store/store.go`, tracked in `schema_migrations`), so a
rollout applies them. They are forward-only: rolling back to an image older
than an applied migration is not supported — restore a backup instead.

## 6. Ad-hoc collector run

The collector only runs on its schedule (hourly by default). To force a
fetch+classify cycle now — after a deploy, after adding a source, or to pick
up a freshly pushed image without waiting — clone the `CronJob` into a one-off
`Job`:

```sh
kubectl -n osint create job collector-manual-$(date +%s) \
  --from=cronjob/osint-baltic-osint-hub-collector

kubectl -n osint get jobs --sort-by=.metadata.creationTimestamp
kubectl -n osint logs -f job/<job-name>
```

The Job copies the CronJob's pod template as it exists *at creation time*, so
it picks up the current `image.tag` and the current `baltic-osint-hub` secret.
It also inherits `activeDeadlineSeconds: 3900`, so a wedged run is killed after
65 minutes (the collector's own `RUN_TIMEOUT_MINUTES`, 50 by default, normally
fires first), and `backoffLimit: 1`, so a failure is retried once, not six
times. The name must be unique; the timestamp suffix handles that, whereas a
plain `collector-manual` fails the second time with `AlreadyExists`.

Two consequences of the CronJob `ownerReference` kubectl attaches:

- the manual Job counts as an active job for `concurrencyPolicy: Forbid`, so a
  scheduled run that comes due while it is still going is **skipped**, not
  queued;
- it is pruned by the same `successfulJobsHistoryLimit: 3`, so finished manual
  jobs clean themselves up. Delete one early by name if you want it gone
  sooner (`kubectl -n osint delete job <job-name>`).

Running one alongside a scheduled run is otherwise harmless: the collector
dedupes by URL and normalized-title hash and enforces per-source fetch
intervals internally, so an extra run largely no-ops instead of re-fetching.
It does spend OpenAI credit on whatever is genuinely new, bounded by
`MAX_ENRICH_PER_RUN`.

## Health endpoints

| Path | Used by | Meaning |
|---|---|---|
| `/healthz` | liveness probe | process is up; never touches the database |
| `/readyz` | readiness probe | pings Postgres; `503` until it answers, so the pod leaves the Service during a DB outage instead of serving errors |

## Versioning

- **Image**: every push to `main` is a release candidate, tagged `latest`,
  `<sha>` and `sha-<short>`. There are no semantic image versions; pin by sha.
- **Chart**: `Chart.yaml` `version` is bumped on any change under
  `deploy/helm/`; `appVersion` records the application release the defaults
  were written for. On each push to `main` the chart is also pushed as an OCI
  artifact to `oci://ghcr.io/mjudeikis/charts/baltic-osint-hub`, so
  `helm install osint oci://ghcr.io/mjudeikis/charts/baltic-osint-hub --version 0.2.0`
  works without a checkout.
- **Database**: migrations are forward-only and applied on start.
