# Concepts

## One binary, many stacks

There is exactly one `zook` binary installed on the server. It manages any
number of application stacks. The per-stack files (compose config, secrets,
release state) are *not* per-app installations of Zook — they are the
configuration and state for apps that the single tool manages.

The analogy is `git`: one `/usr/local/bin/git` binary manages every repository
on the machine. Each repository has its own `.git/` directory. Zook works the
same way: one binary, each stack has its own `.zook/` directory.

```
                 ONE /usr/local/bin/zook
                            │
                          Engine
                            │
                    Runtime interface
                            │
                ┌───────────┴───────────┐
                ▼                        ▼
          DockerRuntime (V1)      NativeRuntime (later)
                │
          docker compose
                │
          Docker Engine
                │
         ┌──────┼──────┐
         ▼      ▼      ▼
       core   tenant  postgres
               -api
```

## What is a stack?

A stack is any directory under the stacks root that contains a `compose.yaml`.
Zook discovers stacks by scanning the filesystem — no central registry, no
configuration file required in V1. If the directory has a `compose.yaml`, it is
a stack.

Stack names are directory names. A stack at `/opt/stacks/saas-staging/` has the
name `saas-staging`.

## What is a release?

A release is a single, immutable image tag applied to all services in a stack at
deploy time. Zook injects the version as an environment variable when invoking
`docker compose`, so `compose.yaml` references it as `${VERSION}`:

```yaml
services:
  api:
    image: ghcr.io/example/api:${VERSION}
```

Zook never writes to `.env`. The authoritative record of what is currently
deployed is `state.json`, not any environment file.

## What Zook is not

- Not a daemon. There is no `zook serve` in V1 — every command is a one-shot
  invocation.
- Not a reverse proxy controller. Zook does not modify Nginx, Caddy, or any
  other router.
- Not a database tool. Rollback reverts the *application version*; it never
  touches database data.
- Not a replacement for Docker Compose. Compose remains the runtime and
  orchestrator; Zook is the release controller layered on top.
- Not Kubernetes, Nomad, or any scheduler. One server, one binary, straightforward
  deploys.
