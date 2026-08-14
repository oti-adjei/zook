# On-Disk Layout

## Stacks root

All stacks live under a single directory, the *stacks root*. The default is
`/opt/stacks`. Override with the `ZOOK_STACKS_ROOT` environment variable.

```
/opt/stacks/                      # stacks root
├── saas-staging/
│   ├── compose.yaml              # orchestration + healthcheck: blocks
│   ├── .env                      # secrets/config — Zook NEVER writes this
│   └── .zook/
│       ├── state.json            # release state (gitignore this)
│       └── logs/
│           └── 20260813T173000Z-v0.1.4.log
├── rue/
│   └── ...
└── another-app/
    └── ...
```

Zook discovers stacks by looking for directories that contain a `compose.yaml`.
No registration step required.

## The `.zook/` directory

Each stack directory gets a `.zook/` subdirectory created by Zook on first use.
It holds:

- `state.json` — the current release state (see below).
- `logs/` — one log file per deploy or rollback operation.

**Add `.zook/` to your stack's `.gitignore`.** This is runtime state, not
source configuration.

## `state.json` schema

```json
{
  "current": "v0.1.4",
  "previous": "v0.1.3",
  "history": [
    { "version": "v0.1.4", "timestamp": "2026-08-13T17:30:00Z", "result": "success" },
    { "version": "v0.1.3", "timestamp": "2026-08-12T14:10:00Z", "result": "success" }
  ]
}
```

| Field | Description |
|-------|-------------|
| `current` | The version currently deployed and healthy. |
| `previous` | The version before the last successful deploy; the rollback target. |
| `history` | All deploy and rollback attempts, oldest first. |

`result` is one of:

| Value | Meaning |
|-------|---------|
| `success` | Deploy or rollback completed; services passed health. |
| `rolled_back` | A deploy was attempted with this version but it was auto-rolled back. |
| `failed` | A deploy or rollback attempt failed and could not be recovered automatically. |

`state.json` is written atomically using a temp-file-then-rename pattern so a
crash mid-write cannot corrupt it.

## Deploy logs

Every deploy or rollback writes a log file:

```
.zook/logs/<timestamp>-<version>.log
```

The timestamp is UTC in compact ISO-8601 format (`20260813T173000Z`). The log
captures the full output of pull, up, health-wait, and any rollback steps,
plus the final result. Use `zook logs <stack>` to view the most recent log.

## Environment variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `ZOOK_STACKS_ROOT` | `/opt/stacks` | Directory that Zook scans for stacks. |
| `ZOOK_TIMEOUT` | `60` | Seconds to wait for `docker compose up --wait` to report healthy. |
