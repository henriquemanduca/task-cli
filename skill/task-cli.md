---
name: task-cli
description: >-
  Manage tasks and projects with the `task-cli` binary (single-binary Go CLI that
  stores tasks as Markdown files in a vault folder). Use when the user asks to add,
  list, view, update, prioritize, start, complete, abort, or delete tasks, or to
  manage projects — e.g. "add a task", "list my pending tasks", "mark task 3 done",
  "move this to the Backlog project".
---

# task-cli

Single-binary CLI task manager written in Go. Use the installed `task-cli`
binary, or `./dist/task-cli` from a source checkout (`make build`).

## Storage (vault)

- Tasks are Markdown files in a vault folder (default `~/.task-cli-vault`).
- Each subfolder is a project; **the project name is the folder name** (no codes/IDs,
  spaces allowed, matched case-insensitively); tasks without a
  project live in the vault root.
- Task file name: `<id>-<slug>.md` (e.g. `Task CLI/0003-ajustar-filtro.md`). IDs are
  global across the vault and never reused (counter `next_id` in `<vault>/.task-cli.yaml`).
- Task file content: YAML frontmatter (`status`, `priority`, `created`,
  `updated`, optional `abort_reason`/`deleted`), then `# Title`, then the
  description in Markdown.
- `<project>/.project.yaml` holds only an optional `description` (created by `project add`).
- Config: `~/.config/task-cli/config.yaml` with `vault: <path>`. `task-cli config`
  opens it in `$EDITOR` (interactive — don't run it as an agent; edit the file directly).
- For tests/demos, use a throwaway vault: `TASK_CLI_VAULT=/tmp/v task-cli ...`
  or `task-cli --vault /tmp/v ...`.
- Reading a task file directly is fine; to change tasks prefer the CLI (it keeps
  file names, `updated` and immutability rules consistent).

## Projects

```bash
task-cli project add "Product Backlog" "Main project backlog"   # alias: create; description optional
task-cli project list            # alias: ls; TSV: name
task-cli project list -d -n      # + description, no header
task-cli project view "product backlog"   # alias: show
task-cli project update "Product Backlog" --name "Backlog"      # renames the folder; aliases: edit, mod
task-cli project update Backlog --desc "New description"        # --desc "" clears it
```

## Tasks

Statuses: `pending`, `in-progress`, `done`, `aborted`. Deleted tasks are moved to `<vault>/.trash/` with status `deleted` and no longer show up in any command.
Priorities: `low`, `medium`, `high`, `urgent`.

> **Immutability**: `aborted` and `deleted` tasks cannot be updated, edited, reprioritized or change status.

```bash
task-cli task add "Standalone Task" "Description"         # aliases: create, new
task-cli task add -p Backlog --priority high "Project Task"

task-cli task list              # pending tasks (default), TSV with header; alias: ls
task-cli task list -a           # all statuses (except deleted)
task-cli task list --done       # only done
task-cli task list in-progress  # positional status filter (or -s/--status)
task-cli task list -p Backlog   # filter by project name
task-cli task list -n -t        # no header, with created_at column

task-cli task view 1            # details + file path (aliases: show, get); -t for timestamps
task-cli task update 1 "New title" "New description"   # alias: mod
task-cli task update 1 -p Other     # move to project "Other" (moves the file)
task-cli task update 1 -p none      # move back to the vault root
task-cli task update 1 --priority urgent
task-cli task edit 1            # open the task's .md in $EDITOR (default nano)
task-cli task prio 1 high       # aliases: priority, set-prio, set-priority
task-cli task status 1          # show status
task-cli task status 1 in-progress
task-cli task start 1           # set-in-progress
task-cli task done 1            # set-done
task-cli task pending 1         # set-pending
task-cli task abort 1 "Out of scope"   # set-abort, optional reason
task-cli task delete 1          # move to <vault>/.trash (aliases: del, rm, remove)
```

## TSV output & scripting

`task list` and `project list` emit tab-separated values with a header row by
default (`-n` suppresses it). Nothing is printed when there are zero rows.
`-F table` (anywhere in the command) prints an aligned table instead.

```
# task list columns:    id  project  status  priority  title  [created_at]
# project list columns: name  [description]
```

```bash
task-cli task list -a -n | cut -f4 | sort | uniq -c                    # count by priority
task-cli task list -a -n -p Backlog | awk -F'\t' '$3=="in-progress"'   # filter rows
```

## Gotchas

- Commands are hierarchical (`task ...`, `project ...`); there are no flat
  top-level shortcuts like `task-cli add`.
- `task list` with no arguments shows **pending** tasks only; use `-a` for everything.
- `-p` takes a project **name** (the folder name, case-insensitive; quote names with spaces), never a number.
- Flags can appear before or after positional arguments.
- Version/help: `task-cli --version`, `task-cli --help`, `task-cli help task add`.
