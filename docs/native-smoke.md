# Native smoke procedure

**Platform:** Linux with systemd only.  
**Why not macOS:** This host has no `systemctl`. The native runtime calls
`systemctl restart <unit>` during every deploy; running this procedure on
macOS will fail at that step. All steps below must be executed on a Linux
machine with a working systemd instance (a VM, a cloud instance, or a
container with systemd enabled is fine).

---

## Prerequisites

- `zook` binary installed and on `$PATH` (or referenced by full path).
- `sudo` / root access (needed for `/etc/systemd/system/` and `/opt/stacks/`).
- A port free on localhost for the health check — this example uses `8080`.

---

## Step 1 — Stage a working v1 release

Create a minimal HTTP binary that serves `GET /healthz → 200` and stays running.
The simplest option is a tiny shell script backed by `nc` or Python, but a real
binary works identically.

### Option A: Python wrapper (no build required)

```bash
sudo mkdir -p /opt/stacks/demo/releases/v1

sudo tee /opt/stacks/demo/releases/v1/hello > /dev/null <<'EOF'
#!/usr/bin/env python3
import http.server, socketserver

class H(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path == "/healthz":
            self.send_response(200)
            self.end_headers()
            self.wfile.write(b"ok")
        else:
            self.send_response(404)
            self.end_headers()
    def log_message(self, *_): pass

with socketserver.TCPServer(("", 8080), H) as srv:
    srv.serve_forever()
EOF

sudo chmod +x /opt/stacks/demo/releases/v1/hello
```

### Option B: Pre-built Go binary

Build a binary that listens on `:8080` and returns 200 for `/healthz`, copy it
to `/opt/stacks/demo/releases/v1/hello`, and `chmod +x` it.

---

## Step 2 — Install the systemd unit

The unit points at the `current` symlink (which zook manages). Write it once;
zook never modifies it.

```bash
sudo tee /etc/systemd/system/demo.service > /dev/null <<'EOF'
[Unit]
Description=Demo service (zook native smoke)

[Service]
ExecStart=/opt/stacks/demo/current/hello
Restart=on-failure

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable demo
```

Do **not** start the unit yet — zook will restart it during the first deploy,
which also creates the `current` symlink.

---

## Step 3 — Write zook.yaml

```bash
sudo tee /opt/stacks/demo/zook.yaml > /dev/null <<'EOF'
runtime: native
binary: hello
systemd_unit: demo
health:
  url: http://localhost:8080/healthz
  expected_status: 200
health_timeout: 30s
rollback_on_fail: true
EOF
```

---

## Step 4 — Deploy v1 (happy path)

```bash
ZOOK_STACKS_ROOT=/opt/stacks zook deploy demo v1
```

Expected behaviour:

1. Preflight validates `zook.yaml` (binary, systemd_unit, health block present).
2. Pull verifies `releases/v1/hello` exists (pre-staged; no `artifact:` URL).
3. `current` symlink is flipped: `/opt/stacks/demo/current → releases/v1`.
4. `systemctl restart demo` is issued.
5. Health probe polls `http://localhost:8080/healthz` until it returns 200.
6. Deploy recorded as successful.

Verify the symlink and health:

```bash
readlink /opt/stacks/demo/current
# → releases/v1

curl -s http://localhost:8080/healthz
# → ok

ZOOK_STACKS_ROOT=/opt/stacks zook status demo
# STACK   RUNTIME  VERSION  STATUS
# demo    native   v1       healthy
```

---

## Step 5 — Stage a broken v2 and observe auto-rollback

Stage a v2 binary that exits immediately (fails health):

```bash
sudo mkdir -p /opt/stacks/demo/releases/v2
sudo tee /opt/stacks/demo/releases/v2/hello > /dev/null <<'EOF'
#!/bin/sh
exit 1
EOF
sudo chmod +x /opt/stacks/demo/releases/v2/hello
```

Deploy v2:

```bash
ZOOK_STACKS_ROOT=/opt/stacks zook deploy demo v2
```

Expected behaviour:

1. Preflight and Pull succeed (directory and binary are present).
2. `current` is flipped to `releases/v2`.
3. `systemctl restart demo` is issued — the process crashes immediately.
4. Health probe polls `http://localhost:8080/healthz`; all requests fail.
5. After `health_timeout` elapses, deploy is declared failed.
6. Auto-rollback triggers (`rollback_on_fail: true`):
   - `current` is flipped back to `releases/v1`.
   - `systemctl restart demo` is issued.
   - Health probe passes on v1.
7. zook exits non-zero and prints a rolled-back message.

Confirm rollback:

```bash
readlink /opt/stacks/demo/current
# → releases/v1

ZOOK_STACKS_ROOT=/opt/stacks zook status demo
# STACK   RUNTIME  VERSION  STATUS
# demo    native   v1       healthy
```

---

## Teardown

```bash
sudo systemctl stop demo
sudo systemctl disable demo
sudo rm /etc/systemd/system/demo.service
sudo systemctl daemon-reload
sudo rm -rf /opt/stacks/demo
```

---

## Why this procedure is not automated

The native runtime requires `systemctl`, which is unavailable on the macOS dev
host. The docker runtime is tested end-to-end in CI; the native runtime is
covered by unit tests (fake runner, fake prober, httptest) in
`internal/runtime/native`. This document captures the full operator-level
end-to-end check for use on a Linux machine before promoting a new zook build to
production.
