# Changelog

All notable changes to Zook are documented here. Format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); this project adheres
to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
