# Zook Handbook

**Zook** is a single-binary deployment controller for Docker Compose stacks.
The name comes from Ghanaian pidgin: *to zook* means *to hold / to manage*.
Zook holds your deployments — it manages the release lifecycle (deploy,
health-gate, roll back) of your Compose stacks, sitting alongside Docker and
Dockge rather than replacing them.

## Pages

| Page | What it covers |
|------|----------------|
| [concepts.md](concepts.md) | Mental model: one binary, many stacks; what a stack is |
| [layout.md](layout.md) | On-disk layout, `state.json` schema, `.zook/` directory, `ZOOK_STACKS_ROOT` |
| [deploy-contract.md](deploy-contract.md) | Immutable tags, healthchecks required, fail-closed, migrations |
| [commands.md](commands.md) | Full CLI reference with worked examples and transcripts |
| [stacks.md](stacks.md) | How to lay out a stack directory and experiment locally |
| [architecture.md](architecture.md) | Engine, `Runtime` interface, package boundaries |
| [roadmap.md](roadmap.md) | V2, V3, and later plans |

## Quick start

```
# Deploy a versioned stack
zook deploy my-app v1.2.3

# Check health across all stacks
zook status

# View the most recent deploy log
zook logs my-app
```

See [commands.md](commands.md) for the full reference.
