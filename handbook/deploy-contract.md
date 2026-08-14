# Deploy Contract

These are the invariants that every Zook-managed stack must satisfy. The whole
auto-rollback design rests on them. Violate one and the guarantees break.

## 1. Immutable image tags — never `latest`

Every service image must be tagged with an immutable version string — a semver
tag (`v1.2.3`), a git SHA, or any other identifier that never moves.

**Do not use `latest` or any other mutable tag.** Rollback works by bringing
the `previous` version back up. If `previous` pointed at `latest` and the
registry has moved `latest` forward, rollback pulls the new image, not the old
one. There is no safe rollback without immutable tags.

Good:
```yaml
image: ghcr.io/example/api:v1.2.3
image: ghcr.io/example/api:a3f8c92
```

Not acceptable:
```yaml
image: ghcr.io/example/api:latest
image: ghcr.io/example/api:stable
```

## 2. Healthchecks required for every long-running service

Every long-running service in the stack must define a Docker `healthcheck:` in
`compose.yaml` or a `HEALTHCHECK` in its Dockerfile. One-shot containers
(migrations, seed scripts) are exempt.

Why this matters: `docker compose up -d --wait` only waits on services that
*have* a healthcheck. A service without one is reported as healthy the instant
its container starts — a false green. Zook's preflight check refuses to deploy
any stack where a long-running service lacks a healthcheck, so the false-green
case never reaches production.

Decision matrix:

| Healthcheck | Outcome |
|-------------|---------|
| Present + passes | SUCCESS — deploy committed |
| Present + fails within timeout | ROLLBACK — previous version restored |
| Missing | REFUSED at preflight — running state is not touched |

Third-party images (databases, caches) often lack a built-in healthcheck; add
one in `compose.yaml`:

```yaml
postgres:
  image: postgres:16-alpine
  healthcheck:
    test: ["CMD-SHELL", "pg_isready -U postgres"]
    interval: 5s
    timeout: 5s
    retries: 10
```

Use only commands that exist inside the image. Do not assume `curl` or `wget`
are present in minimal images.

## 3. Backward-compatible migrations

Schema migrations must follow an expand/contract pattern: the new schema must
remain compatible with the *previous* application version. If a deploy fails
after a migration has run, auto-rollback brings the previous application code
back — but it cannot un-run the migration. The previous application must still
work against the new schema.

This is a people-and-process constraint, not one Zook enforces technically.
Documenting it here makes it explicit for everyone operating a stack.

## 4. Fail-closed

Zook never marks a deploy successful unless it can confirm the new version is
healthy. If health confirmation is impossible — due to a missing healthcheck,
a timeout, or a container crash — Zook does not count it as a success. It
either rolls back (if a previous version exists) or halts for manual
intervention (if there is nothing to roll back to).

Rollback is itself health-gated. If restoring the previous version also fails
health, Zook stops and prints:

```
DEPLOYMENT FAILED / ROLLBACK FAILED — manual intervention required.
```

No further automatic recovery is attempted in V1.

## 5. VERSION injected as environment variable

Zook sets `VERSION` in the environment when calling `docker compose`, never by
writing to `.env`. Use `${VERSION}` in `compose.yaml` to reference it:

```yaml
services:
  api:
    image: ghcr.io/example/api:${VERSION}
```

Zook never modifies `.env`. Secrets and static configuration stay in `.env`,
managed separately from the release lifecycle.
