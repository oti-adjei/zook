# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What Zook is

Zook is a tiny single-binary deployment controller for Docker Compose stacks and native systemd binaries. It manages the release lifecycle — deploy, health-gate, rollback — sitting alongside Docker/Dockge, not replacing them. Pure stdlib + `gopkg.in/yaml.v3`. Go 1.23.

## Commands

```bash
make build     # CGO_ENABLED=0 go build -o bin/zook .
make test      # go test ./...
make install   # build + install to /usr/local/bin/zook

go test ./internal/core/...                     # one package
go test -run TestEngine_Deploy ./internal/core  # one test
```

## Runtime env vars (used by the CLI)

- `ZOOK_STACKS_ROOT` — where stacks live (default `/opt/stacks`).
- `ZOOK_TIMEOUT` — health-wait timeout in **seconds** (default 60). `zook.yaml`'s `health_timeout` overrides per stack.

## Architecture

The core seam is `runtime.Runtime` (`internal/runtime/runtime.go`): `Preflight`, `Pull`, `Up`, `Health`. The engine is runtime-agnostic; two backends implement the interface.

Flow: `main.go` → `cli.Run` (dispatch + exit codes) → `config.FindStack`/`DiscoverStacks` → `core.Engine` → a `runtime.Runtime`.

- **`internal/core`** — the release engine and persisted state. `Engine.Deploy` runs preflight → pull → up-with-health-wait, and on failure auto-rolls-back to `State.Current` (health-gated). If the rollback *also* fails it halts for manual intervention. `rollback_on_fail: false` disables this. State lives per-stack in `<stackDir>/.zook/state.json` (`Current`/`Previous`/`History`), written atomically (temp + rename). Deploy logs go to `<stackDir>/.zook/logs/<ts>-<version>.log`.

- **`internal/config`** — stack discovery and `zook.yaml` parsing. A directory under the stacks root is a stack if it has a compose file (`compose.yaml`/`.yml`/`docker-compose.*`) **or** a `zook.yaml`. `Runtime` resolves to `docker` (default) or `native`. `FindStack` rejects path-traversal names. `zook.yaml` uses `KnownFields(true)` — unknown keys are errors.

- **`internal/runtime/docker`** — shells out to `docker compose -f <file>`, passing the target version as a `VERSION` env var (compose files reference `${VERSION}`). `Up` uses `up -d --wait --wait-timeout`. Preflight refuses to deploy any active service lacking a healthcheck.

- **`internal/runtime/native`** — deploys prebuilt binaries under systemd. Deploy = ensure `releases/<version>/` exists (fetch artifact if configured), flip the `current` symlink atomically, `systemctl restart <unit>`, then poll health. **Zook never builds binaries or writes unit files.** Native stacks *require* a `zook.yaml` with `binary`, `systemd_unit`, and a `health` block.

- **`internal/health`** — HTTP-GET or command prober with `Poll` (retry-until-timeout). This is the health engine the native runtime needs; docker relies on compose's own `--wait`.

- **`internal/exec`** — all external commands run through the `Runner` interface (`OSRunner` in prod, `FakeRunner` in tests). Never call `os/exec` directly in runtime code — inject a `Runner` so it stays unit-testable.

## Conventions

- Runtime code takes an `exec.Runner` and (for native) a `health.Doer` so tests script commands/HTTP without touching the system. Follow this when adding backends.
- `cli.Run` returns process exit codes directly: `0` ok, `1` runtime error, `2` usage error.
- `${VERSION}` is the template token expanded in artifact URLs (native) and passed as `VERSION` env (docker).

## Docs

The `handbook/` dir is the source of truth for concepts, the deploy contract, command reference, architecture, layout, and the native runtime (`handbook/native.md`, `handbook/native-smoke.md`).

## Git

Commit with the noreply email per global instructions:
`git -c user.email='52512684+oti-adjei@users.noreply.github.com' commit -m "..."`
