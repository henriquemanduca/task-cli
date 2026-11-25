# internal

Private packages of `task-cli`:

- `config`: loads `~/.config/task-cli/config.yaml` (`vault:` path, default `~/.task-cli-vault`).
- `vault`: storage. Projects are folders (name = folder name, optional `.project.yaml` with a description), tasks are `<id>-<slug>.md` files with YAML frontmatter; per-vault settings (`next_id`) in `<vault>/.task-cli.yaml`.
- `cmd`: [./cmd/README.md](./cmd/README.md) - hierarchical router, global flags and command handlers.
