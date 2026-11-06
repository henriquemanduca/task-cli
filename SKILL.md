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
- Global flags `--db` and `-F/--format` are transversal and can appear anywhere
  before, between, or after subcommands: `task-cli --db /tmp/x.db task list` or
  `task-cli task list --db /tmp/x.db -F table`.
- Use a throwaway DB (`--db /tmp/...`) when testing or demonstrating commands.

## Projects

```bash
task-cli project add BACKLOG "Product Backlog" "Main project backlog"   # (alias: create)
task-cli project list                                                  # (alias: ls), TSV default
task-cli project list -F table  # formatted table view (shorthand -F, --format table)
task-cli project list -n        # no header row
task-cli project list -d -t     # include description and creation date columns
task-cli project view BACKLOG   # view project details (alias: show)
task-cli project view BACKLOG -t # with timestamps
task-cli np BACKLOG "Product Backlog" "Main project backlog"   # shorthand add
task-cli lp                                                     # shorthand list
```

## Tasks

Statuses: `pending`, `in-progress`, `done`, `abort`.
Priorities: `low`, `medium`, `high`, `urgent`.

```bash
task-cli task add "Standalone Task" "Not linked to any project" # (aliases: create, new)
task-cli task add -p BACKLOG "Project Task" "Linked to BACKLOG"
task-cli task add -p BACKLOG --priority high "Urgent Task"

task-cli task list              # pending tasks (default), TSV with header (aliases: ls)
task-cli task list -F table     # formatted table view (shorthand -F, --format table)
task-cli task list -a           # all tasks, any status
task-cli task list --done       # only completed
task-cli task list in-progress  # filter by status (positional)
task-cli task list -p BACKLOG   # filter by project
task-cli task list -n -t        # no header, with date column

task-cli task view 1            # full details, multiline description (aliases: show, get)
task-cli task view 1 -t         # also show timestamps
task-cli task update 1 "New title" # (alias: mod)
task-cli task update 1 -p BACKLOG   # move to project
task-cli task update 1 -p none      # detach from project
task-cli task edit 1            # edit description in $EDITOR (default nano)
task-cli task prio 1 high       # set priority (aliases: priority, set-prio, set-priority)
task-cli task status 1 in-progress # set status (aliases: set-status)
task-cli task status 1 abort "Out of scope"
task-cli task status 1          # view current task status
task-cli task done 1            # shorthand to mark done
task-cli task delete 1          # soft delete (aliases: del, rm, remove)
```

## Document attachments

`attach` stores only the file's absolute path; it does not copy the document.

```bash
task-cli task doc add 1 /path/to/spec.pdf              # alias: attach, doc-add
task-cli task doc add 1 ./notes.txt "Meeting Notes"
task-cli task doc list 1                              # alias: docs, doc-list, ls
task-cli task doc remove 1                            # alias: doc-del, doc-remove, del, rm
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

`task list` and `project list` emit tab-separated values with a header row by default
(`-n` / `--no-header` suppresses it). No header is printed when there are
zero rows.

```
# task list columns:    id  project  status  priority  title  [created_at]
# project list columns: id  code     name    [description]   [created_at]
```

```bash
task-cli task list -a -n | cut -f4 | sort | uniq -c                    # count by priority
task-cli task list -a -n -p BACKLOG | awk -F'\t' '$3=="in-progress"'   # filter rows
task-cli task list -a | column -t -s $'\t'                             # human-readable table
```

## Gotchas

- Flat aliases for backwards compatibility: traditional flat commands (e.g. `add`, `list`, `done`, `lp`, `np`, `attach`) map directly to their hierarchical routes.
- `task list` with no arguments shows **pending** tasks only; use `-a` for everything.
- `delete` is a soft delete; deleted tasks are hidden from lists and exports
  unless `export --include-deleted`.
- Flags can appear before or after positional arguments.
- Version/help: `task-cli --version`, `task-cli --help`.
