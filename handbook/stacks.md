# Working with Stacks

## What makes a valid stack directory

A stack is any directory under the stacks root (`ZOOK_STACKS_ROOT`, default
`/opt/stacks`) that contains a `compose.yaml`. That is the only requirement for
discovery. No registration step, no Zook-specific config file needed in V1.

Minimum viable stack directory:

```
/opt/stacks/my-app/
├── compose.yaml    # must exist; must define healthcheck: for every long-running service
└── .env            # optional; Zook never writes to this
```

## Compose file requirements

Every long-running service must have a `healthcheck:` block (or a `HEALTHCHECK`
in its Dockerfile). See [deploy-contract.md](deploy-contract.md) for why.

Use `${VERSION}` to reference the version that Zook injects:

```yaml
services:
  api:
    image: ghcr.io/example/my-app:${VERSION}
    healthcheck:
      test: ["CMD", "/app/healthcheck"]
      interval: 5s
      timeout: 5s
      retries: 10

  postgres:
    image: postgres:16-alpine
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      interval: 5s
      timeout: 5s
      retries: 10
```

## Playing with Zook locally

You do not need `/opt/stacks` to experiment. Point `ZOOK_STACKS_ROOT` at any
scratch directory.

```bash
# Create a scratch stacks root
mkdir -p /tmp/my-stacks/hello-app

# Write a minimal compose.yaml with a healthcheck
cat > /tmp/my-stacks/hello-app/compose.yaml <<'EOF'
services:
  web:
    image: nginx:${VERSION}-alpine
    ports:
      - "8080:80"
    healthcheck:
      test: ["CMD", "wget", "-q", "-O", "-", "http://localhost"]
      interval: 5s
      timeout: 5s
      retries: 5
EOF

# Use the scratch root for all commands
export ZOOK_STACKS_ROOT=/tmp/my-stacks

# Discover stacks
zook list
# hello-app

# Deploy a version
zook deploy hello-app 1.27

# Check status
zook status
# STACK        VERSION   STATUS
# hello-app    1.27      healthy

# View releases
zook releases hello-app
# 1.27   2026-08-13T18:00:00Z   success

# View the deploy log
zook logs hello-app
```

## State and logs

After the first deploy, `hello-app` will have a `.zook/` directory:

```
/tmp/my-stacks/hello-app/
├── compose.yaml
└── .zook/
    ├── state.json
    └── logs/
        └── 20260813T180000Z-1.27.log
```

To start fresh, delete `.zook/`:

```bash
rm -rf /tmp/my-stacks/hello-app/.zook
```

## Gitignore recommendation

If your stack directory is version-controlled, add `.zook/` to `.gitignore`:

```
# .gitignore
.zook/
```

`state.json` and logs are runtime artifacts — they belong on the server, not in
source control.

## Adjusting the health timeout

If your images take longer than 60 seconds to become healthy (e.g. a cold JVM
start), increase the timeout:

```bash
ZOOK_TIMEOUT=120 zook deploy my-app v2.0.0
```

Or export it for a whole session:

```bash
export ZOOK_TIMEOUT=120
zook deploy my-app v2.0.0
```
