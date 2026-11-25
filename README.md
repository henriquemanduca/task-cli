# Task CLI

Single-binary task manager written in Go. Tasks are plain Markdown files inside a
**vault** folder: each subfolder is a project, each `.md` file is a task. No
database, no server — the vault can be versioned with git, synced, or opened in
Obsidian/any editor.

## Vault layout

```
~/.task-cli-vault/
  .task-cli.yaml                   # vault settings (next_id), created automatically
  0004-tarefa-avulsa.md            # task without project (vault root)
  Task CLI/                        # project folder (folder name = project name)
    .project.yaml                  # project settings: optional description
    0003-ajustar-filtro.md         # task: <id>-<slug>.md
    0007-novo-design.md
  .trash/                          # deleted tasks (keeps the project subfolder)
    Task CLI/0005-descartada.md
```

Task file:

```markdown
---
status: pending          # pending | in-progress | done | deleted
priority: high           # low | medium | high | urgent
created: 2026-11-25T10:00:00-03:00
updated: 2026-11-25T10:00:00-03:00
deleted: ...             # only for soft-deleted tasks
---
# Ajustar filtro

Free-form Markdown description.
```

- **IDs** are global across the vault and never reused: a new task gets
  `max(next_id in .task-cli.yaml, highest ID on disk + 1)`. Commands refer to tasks
  by number (`task done 3`).
- The file name follows the title and project: changing the title renames the file,
  moving the task to another project moves the file.
- Files can be edited by hand. Unknown frontmatter keys (e.g. `tags`) are kept when
  the CLI rewrites a file. Files that do not match `<id>-*.md` are ignored.
- **Projects have no code or ID: the project name is the folder name** (spaces and
  accents allowed). Commands match it case-insensitively (`-p "task cli"`).
- `project add` creates the folder plus `.project.yaml`, which holds only an optional
  `description`. A folder created by hand is a project too (no `.project.yaml`
  needed). Hidden folders (starting with `.`) are ignored.

## Configuration

`~/.config/task-cli/config.yaml` (`$XDG_CONFIG_HOME/task-cli/config.yaml`; on
macOS/Windows, the OS user config dir). The file is optional.

```yaml
vault: ~/.task-cli-vault   # default; ~, $VARS and paths relative to the config file work
```

`task-cli config` opens this file in `$EDITOR` (then `$VISUAL`, default `nano`),
creating it from a commented template if it does not exist, and validates it when
the editor closes. It never opens the vault, so it works even with a broken config.

Per-vault settings live in `<vault>/.task-cli.yaml`, which travels with the vault.
Today it only holds `next_id` (the ID counter); the CLI updates that key and keeps
comments and other keys intact.

Overrides (mainly for tests): `TASK_CLI_CONFIG=<file>` selects another config file;
`TASK_CLI_VAULT=<dir>` or the hidden global flag `--vault <dir>` select the vault directly.

## Commands

```bash
# Projects
task-cli project add "Task CLI" "Optional description"       # alias: create
task-cli project list [-d] [-n]                               # alias: ls; -d adds descriptions
task-cli project view "Task CLI"                              # alias: show
task-cli project update "Task CLI" --name "New name"          # renames the folder (tasks move with it)
task-cli project update "Task CLI" --desc "New description"   # --desc "" clears it

# Tasks
task-cli task add "Title" "Description"                       # vault root
task-cli task add -p "Task CLI" --priority high "Title"       # inside project "Task CLI"
task-cli task list                                            # pending (default)
task-cli task list -a | --done | in-progress                  # status filters
task-cli task list -p "Task CLI" -t -n                        # project filter, date column, no header
task-cli task view 3 [-t]                                     # alias: show, get
task-cli task update 3 "New title" "New description"          # alias: mod
task-cli task update 3 -p Other | -p none --priority urgent   # move / detach / reprioritize
task-cli task edit 3                                          # open the .md in $EDITOR (default nano)
task-cli task status 3 [in-progress]                          # show or set status
task-cli task start 3 | done 3 | pending 3                    # status shortcuts
task-cli task prio 3 urgent
task-cli task delete 3                                        # move to .trash (status: deleted)
```

Global flags: `-F/--format tsv|table` (anywhere in the command line), `-h/--help`,
`-v/--version`. `task-cli help <command>` shows help for a subcommand.

## Rules

- There is no aborted/cancelled status: a task that is stopped or cancelled is deleted.
- `deleted` tasks are immutable: updates, edits and status changes fail.
- `delete` moves the task to `<vault>/.trash/[<project>/]` (same file name, marked
  `status: deleted` + `deleted:` date). Trashed tasks are invisible to every command;
  their IDs are never reused. To restore one, move the file back and set its status.
- `list` output is TSV with a header by default (`-n` hides it). Nothing is printed
  when there are no rows.

```
# task list columns:    id  project  status  priority  title  [created_at]
# project list columns: name  [description]
```

## Build & test

```bash
make build        # static binary (CGO_ENABLED=0) at dist/task-cli
make install      # installs to ~/.local/bin
make test         # go test ./...
```
