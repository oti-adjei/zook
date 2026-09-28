# Changelog

All notable changes to Zook are documented here. Format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); this project adheres
to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- `zook serve` daemon: HTTP API with `POST /deploy` / `POST /rollback`
  (async jobs, bearer-token auth, fail-closed when unset), `GET /stacks`,
  `GET /stacks/{name}`, `GET /jobs/{id}`, and `GET /healthz`.
- GitHub webhook receiver (`POST /hooks/github`) with HMAC-SHA256 signature
  verification; push events auto-deploy stacks configured via `deploy_on.github`
  (`branch:` exact match or `tag:` glob) in `zook.yaml`.
- Per-stack deploy serialization in the daemon (one in-flight deploy per
  stack, `409` on conflict).

## [0.1.0] - 2026-09-03

### Added
- CLI: `deploy`, `rollback`, `status`, `releases`, `list`, `logs`, `version`.
- Docker Compose runtime with health-gated deploys (`up -d --wait`).
- Preflight that refuses to deploy stacks whose active services lack a healthcheck.
- Automatic, health-gated rollback on failed deploys; halts for manual
  intervention when a rollback also fails.
- Per-stack release state (`.zook/state.json`) and per-deploy logs.
- Native/systemd runtime: deploy prebuilt binaries via release directories, a
  `current` symlink, and `systemctl restart`, with zook-managed health polling.
- `zook.yaml` per-stack config: selects the runtime (`docker` default, or
  `native`) and sets overrides (`health_timeout`, `rollback_on_fail`).
- HTTP/command health prober used by the native runtime.
- HTTP artifact fetcher (`.tar.gz` or single binary) for native releases.
- `RUNTIME` column in `zook list` and `zook status`.
- Static site generator (`site/gen`, `make site`) that renders the handbook.
- Homebrew distribution via GoReleaser: darwin/linux x amd64/arm64 archives,
  a GitHub release, and a cask pushed to `oti-adjei/homebrew-tap`.
- CI workflow running gofmt, `go vet`, `go test`, and a build on push and PR.

### Fixed
- Stack discovery follows symlinked stack directories, so a stack linked into
  the stacks root from an application repo appears in `zook list` and
  `zook status`, not just `zook deploy`.

### Licensing
- Released under the PolyForm Noncommercial License 1.0.0.

[Unreleased]: https://github.com/oti-adjei/zook/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/oti-adjei/zook/releases/tag/v0.1.0
