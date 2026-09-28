# Serve smoke procedure

**Platform:** Any host that can run `zook serve` and reach the stacks root.
Webhook steps additionally need a GitHub repo you control (a throwaway is
fine). Manual checklist — not automated.

---

## Prerequisites

- `zook` binary installed and on `$PATH`.
- `curl`, `jq`, and `openssl` for requests and signature math.
- A stacks root with at least one healthy-gated docker stack (the
  `native-smoke.md` stack works too — the daemon is runtime-agnostic).

This example uses:

```bash
export ROOT=/tmp/serve-smoke/stacks
export ZOOK_STACKS_ROOT=$ROOT
export ZOOK_API_TOKEN=devtoken
export ZOOK_WEBHOOK_SECRET=devsecret
export ZOOK_ADDR=127.0.0.1:8484
```

---

## Step 1 — Prepare a demo stack

```bash
mkdir -p $ROOT/demo
cat > $ROOT/demo/compose.yaml <<'EOF'
services:
  web:
    image: nginx:${VERSION}
    healthcheck:
      test: ["CMD", "true"]
      interval: 1s
EOF
```

(For a smoke that doesn't pull real images, pre-tag a local image as
`nginx:smoke` and deploy version `smoke`.)

## Step 2 — Start the daemon

```bash
zook serve
# zook serve listening on 127.0.0.1:8484 (stacks root /tmp/serve-smoke/stacks)
```

With no `ZOOK_API_TOKEN` set, verify the fail-closed warning prints and:

```bash
curl -s -X POST localhost:8484/deploy -d '{"stack":"demo","version":"smoke"}'
# {"error":"api token not configured — mutating endpoints disabled (set ZOOK_API_TOKEN)"}  [503]
```

Restart with the token set before continuing.

## Step 3 — Read endpoints

```bash
curl -s localhost:8484/healthz            # {"status":"ok"}
curl -s localhost:8484/stacks | jq .      # demo: docker, current ""
curl -s localhost:8484/stacks/nope -o /dev/null -w '%{http_code}\n'   # 404
```

## Step 4 — Deploy over the API

```bash
JOB=$(curl -s -X POST localhost:8484/deploy \
  -H "Authorization: Bearer $ZOOK_API_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"stack":"demo","version":"smoke"}' | jq -r .id)
echo "job: $JOB"

# no auth → 401
curl -s -X POST localhost:8484/deploy -d '{"stack":"demo","version":"smoke"}' -o /dev/null -w '%{http_code}\n'

# poll until status is done or failed
curl -s localhost:8484/jobs/$JOB | jq .
```

Verify the deploy left the same artifacts a CLI deploy would:

```bash
cat $ROOT/demo/.zook/state.json | jq .current   # "smoke"
ls $ROOT/demo/.zook/logs/                        # one log per deploy attempt
```

## Step 5 — Serialization (409)

While a slow deploy is running (a big image pull works), fire a second one:

```bash
curl -s -X POST localhost:8484/deploy \
  -H "Authorization: Bearer $ZOOK_API_TOKEN" \
  -d '{"stack":"demo","version":"smoke2"}' -o /dev/null -w '%{http_code}\n'
# 409 — one in-flight deploy per stack
```

## Step 6 — Webhook: signature and matching

Add to `$ROOT/demo/zook.yaml` (a docker stack may carry a zook.yaml):

```yaml
deploy_on:
  github:
    repo: youruser/yourrepo
    tag: "v*"
```

Simulate a signed tag push:

```bash
BODY='{"ref":"refs/tags/v1.0.0","after":"deadbeef","repository":{"full_name":"youruser/yourrepo"}}'
SIG="sha256=$(printf '%s' "$BODY" | openssl dgst -sha256 -hmac $ZOOK_WEBHOOK_SECRET -r | cut -d' ' -f1)"

# good signature → 202 + job that deploys version v1.0.0
curl -s -X POST localhost:8484/hooks/github \
  -H 'X-GitHub-Event: push' -H "X-Hub-Signature-256: $SIG" -d "$BODY"

# bad signature → 401, nothing enqueued
curl -s -X POST localhost:8484/hooks/github \
  -H 'X-GitHub-Event: push' -H 'X-Hub-Signature-256: sha256=badbadbad' \
  -d "$BODY" -o /dev/null -w '%{http_code}\n'

# non-matching repo → 200 ignored
curl -s -X POST localhost:8484/hooks/github \
  -H 'X-GitHub-Event: push' -H "X-Hub-Signature-256: $(printf '%s' '{"ref":"refs/tags/v1.0.0","repository":{"full_name":"other/repo"}}' | openssl dgst -sha256 -hmac $ZOOK_WEBHOOK_SECRET -r | cut -d' ' -f1 | sed 's/^/sha256=/')" \
  -d '{"ref":"refs/tags/v1.0.0","repository":{"full_name":"other/repo"}}'
```

## Step 7 — Real GitHub webhook (optional)

1. Create a throwaway repo; push a tag `v2.0.0`.
2. Repo Settings → Webhooks → Add webhook:
   - Payload URL: `http://<host>:8484/hooks/github` (or your TLS proxy)
   - Content type: `application/json`
   - Secret: `ZOOK_WEBHOOK_SECRET`'s value
3. Confirm the ping arrives (daemon log shows nothing; GitHub shows ✓).
4. Push another tag. Watch the daemon log: `webhook: youruser/yourrepo push → deploying demo v2.0.0`.
5. Verify via `GET /stacks/demo` that `current` became `v2.0.0`.

## Step 8 — Graceful shutdown

Send `Ctrl-C` (or SIGTERM to the systemd unit). Expect:

```
shutting down…
bye
```

Exit code `0`; in-flight jobs are awaited up to 10s.
