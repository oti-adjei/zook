# Install

## Get the binary

Build from source and install to your PATH:

```bash
git clone https://github.com/oti-adjei/zook.git
cd zook
make build                                    # produces bin/zook
sudo install -m 0755 bin/zook /usr/local/bin/zook
```

Verify the install:

```bash
zook version
```

## Set up your stacks root

zook expects stacks to live under a stacks root directory. The default is `/opt/stacks`. You can override it with the `ZOOK_STACKS_ROOT` environment variable:

```bash
export ZOOK_STACKS_ROOT=/opt/stacks   # default — set in /etc/environment or your shell profile
sudo mkdir -p /opt/stacks
```

Each stack gets its own subdirectory inside the root. zook writes release history and logs there alongside your stack definition — no external state.

## Deploy a Docker Compose stack

Lay out a stack directory under your stacks root:

```
/opt/stacks/
└── saas-staging/
    └── docker-compose.yml
```

Then deploy a specific version:

```bash
zook deploy saas-staging v1.4.2
```

zook will pull the new image, run health checks, and commit the release if everything passes. If health checks fail, it rolls back automatically.

## Deploy a native (systemd) app

For native binaries, set up the stack directory and a systemd unit. See the [native runtime docs](/docs/native/) for the full layout contract.

```bash
zook deploy my-api v2.0.1
```

The same deploy command works for both runtimes. zook detects which type of stack it's dealing with from the stack directory layout.

## Check what's running

```bash
zook status
```

Prints every stack, its current version, and whether health checks are passing.

## Roll back if needed

```bash
zook rollback saas-staging
```

Returns to the last known-good version. If the rollback itself fails health checks, zook halts and leaves the stack in a degraded state — it won't guess.

## Next steps

- [Commands reference](/docs/commands/) — full flag and argument docs for every command
- [Stack layout](/docs/stacks/) — how to structure a Docker Compose stack directory
- [Native runtime](/docs/native/) — running native binaries under zook + systemd
- [Concepts](/docs/concepts/) — health gates, release history, the deploy contract
