# Native Runtime

The native runtime deploys prebuilt binaries managed by systemd. It uses the
same `zook deploy` / `zook rollback` interface as the docker runtime, with the
same health-gated, fail-closed guarantees — but it drives `systemctl` and
release directories instead of `docker compose`.

## What the native runtime does (and does not do)

**Does:**
- Stage a versioned artifact into `releases/<version>/` (by fetching a URL, or
  verifying a pre-staged directory).
- Flip the `current` symlink atomically to `releases/<version>/`.
- Run `systemctl restart <unit>`.
- Poll a health probe (HTTP or command) until the app passes or `health_timeout`
  elapses.
- Auto-roll back (flip symlink back, restart, re-probe) on failure.

**Does not:**
- Build from source. Artifacts must be prebuilt and immutable.
- Generate or install systemd unit files. The operator installs the unit once;
  zook only restarts it.
- Supervise processes directly. systemd owns the lifecycle, boot persistence,
  restart-on-crash, and log capture.

## Operator responsibility

Before the first `zook deploy`, install a systemd unit that points at the
stable `current` path:

```ini
[Unit]
Description=Rue API

[Service]
ExecStart=/opt/stacks/rue/current/rue-api
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

`/opt/stacks/rue/current` is the symlink zook manages. The unit never needs
to change; zook flips the symlink before each restart.

Enable the unit once:

```bash
systemctl enable rue-api
```

Subsequent deploys are driven entirely by `zook deploy rue <version>`.

## zook.yaml for native stacks

Every native stack requires a `zook.yaml` in the stack directory. Zook parses
this file strictly — unknown keys cause a parse error, so typos are caught at
preflight rather than silently ignored.

### Full field reference

```yaml
# Selects the runtime. "docker" is the default when this file is absent or the
# field is omitted.
runtime: native

# Path of the binary within releases/<version>/. Required for native.
binary: rue-api

# systemd unit name that zook restarts. Required for native.
systemd_unit: rue-api

# URL template for the release artifact. ${VERSION} is substituted at deploy
# time. Optional — omit when the artifact is pre-staged by CI.
artifact: https://ci.example.com/rue-${VERSION}.tar.gz

# Health probe. Required for native; exactly one of url or command must be set.
health:
  # HTTP mode: GET the URL; healthy when the response status equals
  # expected_status (default 200).
  url: http://localhost:8080/healthz
  expected_status: 200

  # Command mode (alternative to url): run the command; healthy when it exits 0.
  # command: ["rue-api", "health"]

# How long to wait for health before declaring the deploy failed.
# Accepts Go duration strings (e.g. 30s, 2m). Default: 60s. Applies to both
# runtimes.
health_timeout: 60s

# Whether to auto-roll back when a deploy fails health. Default: true. Applies
# to both runtimes.
rollback_on_fail: true
```

### Validation rules

| Condition | Result |
|-----------|--------|
| `runtime: native` without `binary` | Preflight refuses |
| `runtime: native` without `systemd_unit` | Preflight refuses |
| `runtime: native` without a `health` block | Preflight refuses |
| Both `url` and `command` set in `health` | Preflight refuses |
| Neither `url` nor `command` set | Preflight refuses |
| Unknown key in `zook.yaml` | Parse error on load |
| Invalid `health_timeout` duration string | Parse error on load |

### Worked example

Stack: `rue` at `/opt/stacks/rue/`.

```yaml
# /opt/stacks/rue/zook.yaml
runtime: native
binary: rue-api
systemd_unit: rue-api
artifact: https://releases.example.com/rue/rue-${VERSION}.tar.gz
health:
  url: http://localhost:8080/healthz
  expected_status: 200
health_timeout: 90s
rollback_on_fail: true
```

Deploy:

```
$ zook deploy rue v1.4.2
fetching https://releases.example.com/rue/rue-v1.4.2.tar.gz
current -> releases/v1.4.2
deployed rue v1.4.2
```

## Artifact model

Native stacks use one of two artifact workflows. zook never builds from source;
all artifacts are prebuilt and treated as immutable.

### URL-fetch (recommended)

Set `artifact:` in `zook.yaml` to a URL template containing `${VERSION}`.
On `zook deploy <stack> <version>`, zook substitutes the version and downloads
the artifact into `releases/<version>/`. If `releases/<version>/` already
exists, the download is skipped (idempotent re-deploys).

Two archive shapes are supported:
- **`.tar.gz` / `.tgz`**: extracted directly into `releases/<version>/`.
- **Single binary URL** (no `.tar.gz` suffix): saved as
  `releases/<version>/<basename>` and made executable.

The binary named by `binary:` in `zook.yaml` must be present in the extracted
directory after fetch; preflight verifies this.

### Pre-staged

Omit `artifact:` from `zook.yaml`. Before running `zook deploy`, stage the
release directory manually (via CI, rsync, scp, or similar):

```bash
rsync -a ./dist/rue-v1.4.2/ server:/opt/stacks/rue/releases/v1.4.2/
```

On deploy, Pull verifies that `releases/<version>/` exists and that the binary
is present. If neither `artifact:` is set nor the directory is pre-staged,
preflight refuses before touching the running stack.

## On-disk layout

```
/opt/stacks/rue/
├── zook.yaml                    # runtime: native + config
├── releases/
│   ├── v1.4.2/
│   │   └── rue-api              # immutable — fetched or pre-staged
│   └── v1.4.1/
│       └── rue-api
├── current -> releases/v1.4.2   # symlink; systemd's unit points at this
└── .zook/
    ├── state.json               # release state (unchanged from docker layout)
    └── logs/
        └── 20260814T120000Z-v1.4.2.log
```

`current` is a relative symlink (`releases/v1.4.2`). zook flips it using an
atomic write-to-temp-then-rename, so a crash mid-flip never leaves a broken
symlink. Prior release directories are not deleted — rollback works because
`releases/<previous-version>/` still exists on disk.

## Deploy flow

```
zook deploy rue v1.4.2

  Preflight: validate zook.yaml (binary, systemd_unit, health block present)
             confirm artifact resolvable (URL configured OR releases/v1.4.2 staged)
    │  missing → REFUSE — no changes made
    ▼
  Pull: if artifact URL and releases/v1.4.2 absent → download + extract
        else verify releases/v1.4.2/ exists and binary is present
    ▼
  Up: flip current → releases/v1.4.2  (atomic)
      systemctl restart rue-api
      poll health (url or command) until healthy or health_timeout
    │
    ├─ healthy → record success, write state.json
    │
    └─ unhealthy → auto-rollback:
                     flip current → releases/v1.4.1  (previous)
                     systemctl restart rue-api
                     poll health
                       ok   → record rolled_back, exit non-zero
                       fail → record failed, HALT (manual intervention required)
```

`rollback_on_fail: false` skips the auto-rollback step and halts immediately
after the failed health poll, leaving state marked `failed`. Use this for
stacks where you prefer to intervene manually rather than have zook attempt a
restore.

`zook rollback rue` takes the same restore path on demand: flips `current` to
`previous`, restarts the unit, and health-gates the result.

## Error handling summary

| Situation | Outcome |
|-----------|---------|
| Preflight validation fails | Refused — no disk changes, exit non-zero |
| Artifact not found (no URL, not staged) | Refused at Pull, exit non-zero |
| `systemctl restart` fails | Up returns error → auto-rollback |
| Health poll times out | Up returns error → auto-rollback |
| Auto-rollback succeeds | `rolled_back` recorded, exit non-zero |
| Auto-rollback also fails | HALT — `failed` recorded, manual intervention message |
| `rollback_on_fail: false` | No auto-rollback — `failed` recorded, exit non-zero |
