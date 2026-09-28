# Serve (HTTP API)

`zook serve` runs a long-running daemon exposing the same release engine the
CLI drives over HTTP: deploy and rollback endpoints, stack status, and a
GitHub webhook receiver that deploys on push. The CLI and the daemon are two
transports over one engine — a deploy triggered by a webhook produces the
same logs, the same `state.json` transitions, and the same auto-rollback as
`zook deploy` run by hand.

```
                 ┌────────────┐
   zook deploy ─►│   cli      │──┐            ┌─────────────────┐
                 └────────────┘  ├───────────►│   core.Engine   │
                 ┌────────────┐  │            │  (unchanged)    │
   POST /deploy ►│  server    │──┘            └────────┬────────┘
   GH webhook ──►│  (daemon)  │                        │
                 └────────────┘                        ▼
                                              runtime.Runtime
                                              (docker | native)
```

## Running the daemon

```bash
ZOOK_API_TOKEN=... ZOOK_WEBHOOK_SECRET=... zook serve
# zook serve listening on 127.0.0.1:8484 (stacks root /opt/stacks)
```

| Variable | Default | Effect |
|----------|---------|--------|
| `ZOOK_ADDR` | `127.0.0.1:8484` | Listen address |
| `ZOOK_API_TOKEN` | *(unset)* | Bearer token for `POST /deploy` and `POST /rollback`. **Unset = those endpoints return 503** — fail closed, never open |
| `ZOOK_WEBHOOK_SECRET` | *(unset)* | GitHub webhook HMAC secret. **Unset = every delivery is rejected** |
| `ZOOK_STACKS_ROOT` | `/opt/stacks` | Same root the CLI uses |
| `ZOOK_TIMEOUT` | `60` | Same default health-wait the CLI uses |

The daemon binds to loopback by default. To expose it, put a TLS-terminating
proxy (caddy, nginx) in front; zook does not terminate TLS itself.

### systemd unit for the daemon

Per zook's rule, the operator owns unit files. A reasonable one:

```ini
[Unit]
Description=zook serve
After=network-online.target

[Service]
Environment=ZOOK_API_TOKEN=changeme
Environment=ZOOK_WEBHOOK_SECRET=changeme
ExecStart=/usr/local/bin/zook serve
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

## HTTP API

| Method + path | Auth | Behavior |
|---|---|---|
| `GET /healthz` | none | Liveness: `{"status":"ok"}` |
| `GET /stacks` | none | Discovered stacks: name, runtime, current/previous version, last result |
| `GET /stacks/{name}` | none | One stack's full `state.json` view; 404 if unknown |
| `POST /deploy` | bearer | `{"stack":"web","version":"v2"}` → `202 {"id":…}` |
| `POST /rollback` | bearer | `{"stack":"web"}` → `202 {"id":…}` |
| `GET /jobs/{id}` | none | Job status: `queued` → `running` → `done`/`failed` (with `error`) |
| `POST /hooks/github` | HMAC | See below |

Deploys are **asynchronous**: health-gated deploys can take minutes, and
webhook callers must be answered in seconds. Mutating endpoints enqueue a job
and return `202` with its id immediately; poll `GET /jobs/{id}` for the
outcome. The durable record of a deploy is still the stack's `state.json` and
deploy log — job records are in-memory and vanish on restart.

**One deploy per stack at a time.** The engine assumes a single writer; the
daemon enforces it. A request for a stack with a deploy in flight returns
`409`.

```bash
# deploy
curl -s -X POST localhost:8484/deploy \
  -H 'Authorization: Bearer '"$ZOOK_API_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"stack":"web","version":"v2"}'
# → {"id":"a1b2c3d4e5f6a7b8","status":"queued"}

# poll
curl -s localhost:8484/jobs/a1b2c3d4e5f6a7b8
# → {"id":"a1b2c3d4e5f6a7b8","kind":"deploy","stack":"web","version":"v2","status":"done",...}

# status
curl -s localhost:8484/stacks/web
```

## GitHub webhooks

Configure a webhook in the application repo (Settings → Webhooks):

- **Payload URL** — `https://your-host/hooks/github`
- **Content type** — `application/json`
- **Secret** — the same value as `ZOOK_WEBHOOK_SECRET`

Every delivery is HMAC-verified (`X-Hub-Signature-256`) before anything else
happens; a bad signature is a `401` and never triggers a deploy.

`ping` events get `200`; non-`push` events are ignored with `200`.

### Wiring a stack: `deploy_on.github`

Add a `deploy_on` block to the stack's `zook.yaml` (works for both runtimes):

```yaml
deploy_on:
  github:
    repo: oti-adjei/rue     # must equal the payload's repository.full_name
    branch: main            # deploy every push to main…
```

or, to deploy tags:

```yaml
deploy_on:
  github:
    repo: oti-adjei/rue
    tag: "v*"               # …or every tag matching this glob
```

Rules:

- `repo` matches **exactly**; `branch` matches **exactly**; `tag` is a **glob**
  (`path.Match` syntax).
- Exactly one of `branch` / `tag` per stack — enforced at config parse.
- A tag push deploys the **tag name** (`refs/tags/v1.2.3` → version `v1.2.3`).
  A branch push deploys the **full commit SHA** of the pushed commit. Versions
  are opaque strings to zook: docker stacks pass them as the `VERSION` env
  var; native stacks expand them in `artifact` URLs (`${VERSION}`), so CI that
  publishes artifacts keyed by SHA or tag works unchanged.
- A push that matches no stack's `deploy_on` returns `200 {"status":"ignored"}`.
