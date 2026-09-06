# Security

## Reporting

Email **mangirdas@judeikis.lt** with "baltic-osint-hub security" in the
subject, or use GitHub's private vulnerability reporting on this repository.
Please do not open a public issue for anything exploitable. You will get an
acknowledgement within a few days; this is a one-person project, so fixes land
as fast as one person can land them.

## Scope

What the public instance exposes:

- a **read-only, unauthenticated HTTP API** (`/api/*`) and a static frontend.
  There are no accounts, sessions, uploads or write endpoints; every response
  is derived from publicly reported material and public open-data feeds;
- per-IP rate limiting (10 req/s, burst 30) as the only abuse control.

Things that are in scope and worth reporting:

- any way to write to or alter the database, or to make the server perform a
  request to an arbitrary host;
- injection through fetched content: the collector ingests untrusted RSS,
  Telegram previews and social posts and feeds them to an LLM and the
  frontend, so prompt injection that alters classifications, or stored XSS
  through a source title or summary, both count;
- credential exposure — the Helm chart, CI workflow and image are in this
  repo;
- denial of service that the rate limiter does not contain.

Out of scope:

- classification *errors* — wrong severity, wrong country, a merged pair that
  should not have merged. Those are data-quality issues; open a normal issue;
- the content of third-party sources the dashboard links to;
- volumetric attacks against the Cloudflare edge.

## Supported versions

Only `main` and the latest image on `ghcr.io/mjudeikis/baltic-osint-hub`.
Dependabot keeps Go, npm, Actions and base-image versions current, and CI
runs `govulncheck` and a Trivy image scan on every push.
