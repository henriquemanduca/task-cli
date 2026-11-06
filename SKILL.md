---
name: task-cli
description: >-
  Manage tasks, projects, and document attachments with the `task-cli` binary
  (single-binary Go CLI backed by SQLite). Use when the user asks to add, list,
  view, update, prioritize, start, complete, abort, or delete tasks; manage
  projects; attach documents to tasks; or export/import tasks as JSON — e.g.
  "add a task", "list my pending tasks", "mark task 3 done", "move this to the
  BACKLOG project", "export my tasks".
---

# task-cli

Single-binary CLI task manager written in Go with embedded SQLite storage
(pure Go driver, static binary, no server). Use the installed `task-cli`
binary, or `./dist/task-cli` from a source checkout (`make build`).

## Database

- Default DB: `~/.task-cli.db`. Override with `TASK_CLI_DB` env var or `--db`.
- The global `--db` flag must come **before** the subcommand:
  `task-cli --db /tmp/x.db list`. After the subcommand it is parsed as a
  command flag and fails.
- Use a throwaway DB (`--db /tmp/...`) when testing or demonstrating commands.

## Projects

```bash
task-cli np BACKLOG "Product Backlog" "Main project backlog"   # add (alias: project-add)
task-cli lp                                                     # list (alias: projects), TSV
task-cli lp -n        # no header row
task-cli lp -d -t     # include description and creation date columns
```

## Tasks

Statuses: `pending`, `in-progress`, `done`, `abort`.
Priorities: `low`, `medium`, `high`, `urgent`.

```bash
task-cli add "Standalone Task" "Not linked to any project"
task-cli add -p BACKLOG "Project Task" "Linked to BACKLOG"
task-cli add -p BACKLOG --priority high "Urgent Task"

task-cli list              # pending tasks (default), TSV with header
task-cli list -a           # all tasks, any status
task-cli list --done       # only completed
task-cli list in-progress  # filter by status (positional)
task-cli list -p BACKLOG   # filter by project
task-cli list -n -d -t     # no header, with description + date columns

task-cli view 1            # full details, multiline description
task-cli view 1 -t         # also show timestamps
task-cli update 1 "New title"
task-cli update 1 -p BACKLOG   # move to project
task-cli update 1 -p none      # detach from project
task-cli edit 1            # edit description in $EDITOR (default nano)
task-cli set-prio 1 high
task-cli set-pending 1
task-cli set-in-progress 1
task-cli set-done 1
task-cli set-abort 1 "Out of scope"
task-cli delete 1          # soft delete
```

## Document attachments

`attach` stores only the file's absolute path; it does not copy the document.

```bash
task-cli attach 1 /path/to/spec.pdf              # alias: doc-add
task-cli attach 1 ./notes.txt "Meeting Notes"
task-cli docs 1                                  # alias: doc-list
task-cli doc-del 1                               # alias: doc-remove
```

## Export & Import (JSON)

```bash
task-cli export                                  # alias: exp; all to stdout
task-cli export --pretty --out backup.json
task-cli export -f BACKLOG --out backlog.json    # project filter: -f/--filter/--project/--proj (NOT -p)
task-cli export --status pending
task-cli export --include-deleted                # include soft-deleted tasks
task-cli export --dry-run

task-cli import backup.json                      # alias: imp
task-cli import backup.json --overwrite          # update metadata of existing projects
task-cli import backup.json --dry-run            # validate only
task-cli import backup.json --strict             # abort on first validation error
task-cli import backup.json --force-docs         # allow duplicate attachments
cat backup.json | task-cli imp -                 # import via stdin
```

Import dedupes projects by case-insensitive code, always inserts tasks as new
rows, and is transactional (dry-run rolls back). Invalid rows are skipped with
warnings unless `--strict`.

## TSV output & scripting

`lp` and `list` emit tab-separated values with a header row by default
(`-n` / `--no-header` suppresses it). No header is printed when there are
zero rows.

```
# list columns:  id  project  status  priority  title  [description]  [created_at]
# lp columns:    id  code  name  [description]  [created_at]
```

```bash
task-cli list -a -n | cut -f4 | sort | uniq -c                    # count by priority
task-cli list -a -n -p BACKLOG | awk -F'\t' '$3=="in-progress"'   # filter rows
task-cli list -a | column -t -s $'\t'                             # human-readable table
```

## Gotchas

- `list` with no arguments shows **pending** tasks only; use `-a` for everything.
- `delete` is a soft delete; deleted tasks are hidden from lists and exports
  unless `export --include-deleted`.
- Flags can appear before or after positional arguments.
- Version/help: `task-cli --version`, `task-cli --help`.
