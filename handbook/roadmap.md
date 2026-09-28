# Roadmap

## V1 — shipped

Single-binary Docker Compose deployment controller. Deploy, health-gate,
auto-rollback, manual rollback, status, release history, list, logs.
No daemon, no config file required. See the rest of this handbook.

## V2 — native/systemd runtime + zook.yaml — shipped

**Native runtime.** `NativeRuntime` implements the `Runtime` interface, managing
application versions using systemd units and release directories (symlink-swap
pattern). Enables deploying prebuilt binaries with the same `deploy`/`rollback`
UX as docker stacks.

**`zook.yaml`.** A per-stack config file that selects the runtime and carries
overrides:

- `runtime: native` — switch the stack to the native backend.
- `health_timeout: 120s` — override the global `ZOOK_TIMEOUT` per stack.
- `rollback_on_fail: false` — disable auto-rollback for stacks where manual
  control is preferred.

`zook.yaml` is optional for docker stacks; absent, V2 behaves identically to V1.
For native stacks it is required. Existing docker stacks need no changes.

Also shipped in V2: an HTTP/command health prober used by the native runtime;
an HTTP artifact fetcher (`.tar.gz` or single binary); and a `RUNTIME` column
in `zook list` and `zook status`.

## V3 — `zook serve` (webhooks + HTTP API) — shipped

**Long-running daemon** that exposes an HTTP API and webhook endpoint:

- `POST /deploy` — trigger a deploy from a GitHub Actions workflow or any
  CI system.
- GitHub webhook receiver — HMAC-verified push events deploy a stack via
  per-stack `deploy_on.github` config (`branch:` exact or `tag:` glob) in
  `zook.yaml`.
- Status API — `GET /stacks`, `GET /stacks/{name}`, `GET /jobs/{id}`.

The CLI became one transport; the HTTP handler another. The `core.Engine`
and `Runtime` interface are unchanged. Deploys serialize per stack (409 on
conflict); mutating endpoints require a bearer token and fail closed when
unset. See [Serve (HTTP API)](/docs/serve/).

## Later

- **Multi-server** — coordinate deploys across a fleet of machines.
- **Backup orchestration** — snapshot volumes or databases before a deploy,
  wired into the preflight step.
- **Dashboard** — a lightweight web UI for viewing stack status, release
  history, and logs without SSH access.

---

These are directional; exact scope and sequencing will shift as the tool is
used. V1 is the foundation everything else builds on.
