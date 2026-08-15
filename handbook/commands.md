# Command Reference

All commands share this structure:

```
zook <command> [args]
```

Environment variables that affect every command:

| Variable | Default | Effect |
|----------|---------|--------|
| `ZOOK_STACKS_ROOT` | `/opt/stacks` | Root directory scanned for stacks |
| `ZOOK_TIMEOUT` | `60` | Seconds to wait for health on deploy/rollback |

`zook.yaml` overrides: a stack's `health_timeout` and `rollback_on_fail` fields
in `zook.yaml` take precedence over the global defaults for that stack. These
overrides apply to both the docker and native runtimes.

---

## `zook deploy <stack> <version>`

Deploy `version` to `stack`. Runs preflight, pulls images, brings the stack up
with health-gating, and auto-rolls back on failure.

**Steps:**

1. **Preflight** — checks that every long-running service in `compose.yaml` has
   a healthcheck. Refuses with a clear error if any are missing; does not touch
   the running state.
2. **Pull** — `VERSION=<version> docker compose pull`
3. **Up** — `VERSION=<version> docker compose up -d --wait --wait-timeout <ZOOK_TIMEOUT>`
4. If `up` succeeds: commits `state.json` with `current=<version>`,
   `previous=<old current>`, `result=success`.
5. If `up` fails and a previous version exists: auto-rollback to `previous`
   (health-gated). If rollback succeeds, records `result=rolled_back` for the
   failed version. If rollback also fails, halts and requires manual
   intervention.

**Successful deploy:**

```
$ zook deploy saas-staging v0.1.4
deployed saas-staging v0.1.4
```

**Auto-rollback transcript (deploy fails, rollback succeeds):**

```
$ zook deploy saas-staging v0.1.5
deploy of v0.1.5 failed, rolled back to v0.1.4: exit status 1
```

The stack is back on `v0.1.4`. Check the log for the root cause:

```
$ zook logs saas-staging
== 20260813T173500Z-v0.1.5.log ==
=== deploy saas-staging v0.1.5 @ 2026-08-13T17:35:00Z ===
...
deploy of v0.1.5 failed: exit status 1
rolling back to v0.1.4
...
```

**Preflight refusal:**

```
$ zook deploy saas-staging v0.1.4
refusing deploy: services missing a healthcheck: worker
```

---

## `zook rollback <stack>`

Restore the stack to its previous version. This is the manual form of the same
health-gated path that auto-rollback uses.

- Reads `previous` from `state.json`.
- Runs `docker compose up -d --wait` for the previous version.
- If health passes, records a new success entry in history.
- If health fails, records a failed entry and exits non-zero.

```
$ zook rollback saas-staging
rolled back saas-staging
```

Rollback requires a `previous` version. If none exists (the stack has never had
a successful deploy), the command exits with an error.

---

## `zook status [stack]`

Show the current version and health of one stack, or all discovered stacks when
no argument is given.

**All stacks:**

```
$ zook status
STACK           VERSION   RUNTIME   STATUS
saas-staging    v0.1.4    docker    healthy
rue             v1.4.1    native    healthy
```

**Single stack:**

```
$ zook status saas-staging
STACK           VERSION   RUNTIME   STATUS
saas-staging    v0.1.4    docker    healthy
```

The `RUNTIME` column shows `docker` or `native` as resolved from `zook.yaml`
(default `docker` when no `zook.yaml` is present).

Health is checked live — via `docker compose ps` for docker stacks, and via the
configured `health.url` or `health.command` probe for native stacks. It
reflects actual runtime state at the time of the command, not what `state.json`
says. Possible values: `healthy`, `unhealthy`, `unknown`.

---

## `zook releases <stack>`

Show the full release history for a stack, newest entry first.

```
$ zook releases saas-staging
v0.1.4    2026-08-13T17:30:00Z    success
v0.1.3    2026-08-12T14:10:00Z    success
v0.1.2    2026-08-11T09:00:00Z    rolled_back
```

Each line is `<version>  <timestamp>  <result>`. Results: `success`,
`rolled_back`, `failed`.

---

## `zook list`

List all stacks discovered under the stacks root.

```
$ zook list
STACK           RUNTIME
saas-staging    docker
rue             native
another-app     docker
```

The `RUNTIME` column shows `docker` or `native` as resolved from `zook.yaml`
(default `docker` when no `zook.yaml` is present).

A stack is any directory under `ZOOK_STACKS_ROOT` that contains a `compose.yaml`
or a `zook.yaml`.

---

## `zook logs <stack>`

Print the most recent deploy or rollback log for the stack.

```
$ zook logs saas-staging
== 20260813T173000Z-v0.1.4.log ==
=== deploy saas-staging v0.1.4 @ 2026-08-13T17:30:00Z ===
[1/2] Pulling from ghcr.io/example/api
...
```

Logs are stored at `.zook/logs/<timestamp>-<version>.log` inside the stack
directory. The most recent file (by name sort) is shown. To see older logs,
read them directly from the filesystem.
