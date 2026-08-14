# Changelog

All notable changes to Zook are documented here. Format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); this project adheres
to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- CLI: `deploy`, `rollback`, `status`, `releases`, `list`, `logs`.
- Docker Compose runtime with health-gated deploys (`up -d --wait`).
- Preflight that refuses to deploy stacks whose active services lack a healthcheck.
- Automatic, health-gated rollback on failed deploys; halts for manual
  intervention when a rollback also fails.
- Per-stack release state (`.zook/state.json`) and per-deploy logs.
