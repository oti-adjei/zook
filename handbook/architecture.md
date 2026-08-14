# Architecture

## Package boundaries

```
zook/
├── main.go            # entry point: calls cli.Run
├── internal/
│   ├── cli/           # command parsing, human-readable output
│   │   ├── cli.go     # command dispatch (deploy/rollback/status/releases/list/logs)
│   │   └── status.go  # status table + logs display
│   ├── core/          # release logic, state — runtime-agnostic
│   │   ├── engine.go  # Deploy + Rollback orchestration
│   │   └── state.go   # State, HistoryEntry, LoadState, SaveState
│   ├── runtime/
│   │   ├── runtime.go # Runtime interface
│   │   └── docker/    # DockerRuntime: shells out to `docker compose`
│   ├── config/        # stack discovery, FindStack, DiscoverStacks
│   └── exec/          # Runner interface + OSRunner (thin shell-out wrapper)
```

## The `Runtime` interface

The `Runtime` interface is the central seam in Zook's design. Everything above
it is runtime-agnostic; everything below it is implementation-specific.

```go
type Runtime interface {
    Preflight(ctx, stack, log) (missingHealthchecks []string, err error)
    Pull(ctx, stack, version, log) error
    Up(ctx, stack, version, timeout, log) error
    Health(ctx, stack) (healthy bool, err error)
}
```

`DockerRuntime` (V1) implements this interface by shelling out to the `docker`
CLI. It calls:

- `docker compose config --services` + `docker compose config --format json` for preflight
- `docker compose pull` for pull
- `docker compose up -d --wait --wait-timeout <secs>` for up
- `docker compose ps --format json` for health queries

A later `NativeRuntime` (V2) will implement the same interface using systemd
units and release directories — no changes required in `core`, `cli`, or
`config`.

## The `core` engine

`core.Engine` owns the release logic. It is the reusable value of the tool.

`Engine.Deploy` sequence:
1. Load `state.json` (missing file = zero state, not an error).
2. Open a log file for this operation.
3. Call `rt.Preflight`. If any services are missing healthchecks, refuse and
   return without touching the running state.
4. Call `rt.Pull`.
5. Call `rt.Up`. If it succeeds, call `state.recordSuccess` and save.
6. If `rt.Up` fails and a `previous` version exists, call `rt.Up` again with
   `previous`. If that succeeds, call `state.recordRolledBack` and return an
   error describing what happened. If the rollback also fails, call
   `state.recordFailed`, save, and return an error demanding manual intervention.

`Engine.Rollback` is a simplified path: load state, verify `previous` exists,
call `rt.Up`, record the outcome.

## The CLI as a thin transport

The `cli` package is one transport over the engine. It:
- Reads `ZOOK_STACKS_ROOT` and `ZOOK_TIMEOUT` from the environment.
- Resolves the stack directory using `config.FindStack`.
- Calls `Engine.Deploy` or `Engine.Rollback`.
- Formats output for a human terminal.

A future `zook serve` daemon would wrap the same `core.Engine` with an HTTP
handler — zero rework to the engine, state management, or `Runtime` interface.

## Atomic state writes

`SaveState` writes `state.json` by creating a temp file in the same `.zook/`
directory and then renaming it over the target path. On POSIX systems, rename is
atomic: a crash mid-write leaves the old `state.json` intact.

## Exec abstraction

The `exec.Runner` interface wraps the actual shell-out. `OSRunner` uses
`os/exec`. In tests, a fake runner is injected so that no real Docker daemon is
required. This keeps the test suite fast and hermetic.
