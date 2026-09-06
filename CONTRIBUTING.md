# Contributing

Small project, small process. Open an issue first for anything that changes
what the dashboard claims (a new source, a posture rule, a threshold) — those
decisions are documented in `docs/methodology.md` and the PR should update it.
Bug fixes and hardening can go straight to a PR.

## Run it

```sh
make dev            # Postgres (docker compose, :5433) + frontend build + server on :8080
make dev-web        # Vite hot reload, proxies /api to :8080
make collect        # one collector cycle against the local DB
```

Copy `.env.example` to `.env` first. `OPENAI_API_KEY` is optional for
development: without it the collector fetches but does not classify.

## Test

```sh
make test           # go test ./... + frontend vitest; needs no credentials
make test-db        # SQL suites against a throwaway osint_test DB (they TRUNCATE)
make test-live      # + tests that hit real upstream feeds (LIVE_FEEDS=1)
OPENAI_API_KEY=sk-... go test ./internal/cluster/ -run Calibration -v
```

Never point `TEST_DATABASE_URL` at a database you care about. The calibration
test is the one that fixes `CLUSTER_THRESHOLD`: re-run it after changing the
embedding model, the dimension count, or the threshold, and paste the numbers
into the PR. It exists because the threshold was originally set by reasoning
to a value that, when measured, merged nothing at all.

CI runs the same things plus `govulncheck`, a Trivy scan of the image, and
`helm lint`; a PR must be green.

## Lint and format

```sh
make fmt            # gofmt + goimports
make lint           # golangci-lint (.golangci.yml) + eslint + tsc
make helm-lint      # chart lint + render in its main configurations
```

## Pull requests

- One change per PR. A source addition, a UI change and a chart bump are
  three PRs.
- If you touch `deploy/helm/`, bump `Chart.yaml` `version`.
- If you change an env variable, update the table in `README.md` and
  `.env.example`.
- Comments explain *why*, not what; the codebase keeps its history of
  mistakes in comments on purpose (see the threshold story above). Keep that
  style.

## Commit messages

Imperative, lower-case subject under ~70 characters, scoped when it helps:
`collector: throttle reddit fetches`, `chart: move postgres password into a
Secret`, `docs: split self-hosting out of README`. The body says why, and
names any number that was measured rather than chosen.
