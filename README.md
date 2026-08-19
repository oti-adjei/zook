# Zook

A tiny, single-binary deployment controller for Docker Compose stacks.
"To zook" is Ghanaian pidgin for *to hold / to manage* — Zook holds your
deployments: it manages the release lifecycle (deploy, health-gate, roll back)
of your Compose stacks, sitting alongside Docker and Dockge rather than
replacing them.

Zook supports two runtimes — Docker Compose stacks and native systemd binaries
— selected per stack via `zook.yaml`. See [handbook/native.md](handbook/native.md)
for the native runtime.

## Install

```sh
brew install oti-adjei/tap/zook
```

Or build from source: `make build` (outputs `bin/zook`), then `make install`.

See the [handbook](handbook/README.md) for concepts, the deploy contract,
command reference, architecture, and roadmap.

The project includes a website generated from the handbook pages via `make site` — see `site/` for the generator and templates.

## License

Zook is licensed under the [PolyForm Noncommercial License 1.0.0](LICENSE):
free for personal and other noncommercial use; commercial use requires a
separate license. Contact the author for commercial terms.
