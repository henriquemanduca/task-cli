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

## Storage & Platform Data Roots

- Standard data directories (`<data_root>`):
  - Linux/BSD: `$XDG_DATA_HOME/task-cli` (default: `~/.local/share/task-cli`)
  - macOS: `~/Library/Application Support/task-cli`
  - Windows: `%LOCALAPPDATA%\task-cli`
- Primary DB: `<data_root>/task-cli.db`.
- Legacy migration: on first run, `~/.task-cli.db` is safely migrated to `<data_root>/task-cli.db` and renamed to `~/.task-cli.db.migrated`.
- Testing override: `TASK_CLI_DB` env var or `--db <path>` overrides DB location (use throwaway DB `--db /tmp/...` when testing or demonstrating commands).
- Global format flag `-F/--format` is transversal and can appear anywhere: `task-cli task list`.

## Workspace config (`.taskcli`)

`task-cli` searches for `.taskcli` from the current directory upward, stopping at
`$HOME` (nearest file wins). Format is `.env`-style:

```bash
# .taskcli at the project/workspace root
PROJECT_CODE=BACKLOG
```

- `task add` links new tasks to that project when `-p` is omitted.
- `task list` filters by that project when `-p` is omitted.
- Explicit flags override it: `task list -p OTHER`, `task add -p 2 "Title"`.
- To ignore the configured project for one invocation, pass an empty value:
  `task list -p ""` or `task add -p "" "Title"`.
- Blank lines, `#` comments, `export KEY=VALUE`, and quoted values are accepted.
- Malformed files fail with file path and line number; `version`/`help` ignore
  the file.

## Projects

```bash
task-cli project add BACKLOG "Product Backlog" "Main project backlog"   # (alias: create)
task-cli project list                                                  # (alias: ls), TSV default
task-cli project list -n         # no header row
task-cli project list -d -t      # include description and creation date columns
task-cli project view BACKLOG    # view project details (alias: show)
task-cli project view BACKLOG -t # with timestamps
task-cli project update BACKLOG --name "New Name" --desc "New description" # update project (aliases: edit, mod)
task-cli np BACKLOG "Product Backlog" "Main project backlog"   # shorthand add
task-cli lp                                                     # shorthand list
```

## Tasks

Statuses: `pending`, `in-progress`, `done`, `aborted` (aliases: `abort`). Soft-deleted tasks have status `deleted`.
Priorities: `low`, `medium`, `high`, `urgent`.

> **Immutability note**: Tasks with status `deleted` or `aborted` are terminal and immutable. Status changes, recovery, and updates (`update`, `edit`, `prio`) are prohibited and will return an error.

```bash
task-cli task add "Standalone Task" "Not linked to any project" # (aliases: create, new)
task-cli task add -p 1 "Project Task" "Linked to project ID 1"
task-cli task add -p 1 --priority high "Urgent Task"

task-cli task list              # pending tasks (default), TSV with header (aliases: ls)
task-cli task list -a           # all tasks, any status
task-cli task list --done       # only completed
task-cli task list in-progress  # filter by status (positional)
task-cli task list aborted      # filter aborted tasks (alias: abort)
task-cli task list -p BACKLOG   # filter by project
task-cli task list -n -t        # no header, with date column

task-cli task view 1            # full details, multiline description (aliases: show, get)
task-cli task view 1 -t         # also show timestamps
task-cli task update 1 "New title" # (alias: mod)
task-cli task update 1 -p 1        # move to project ID 1
task-cli task update 1 -p none     # detach from project
task-cli task edit 1            # edit description in $EDITOR (default nano)
task-cli task prio 1 high       # set priority (aliases: priority, set-prio, set-priority)
task-cli task status 1 in-progress # set status (aliases: set-status)
task-cli task status 1 abort "Out of scope" # set status to aborted with reason
task-cli task set-abort 1 "Reason" # abort shortcut (aliases: abort)
task-cli task status 1          # view current task status
task-cli task done 1            # shorthand to mark done
task-cli task delete 1          # soft delete (sets deleted_at and status 'deleted'; aliases: del, rm, remove)
```

## Document attachments

`task doc add` duplicates the document into centralized storage (`<data_root>/attachments/<task_id>/<stem>_<hash>.<ext>`) with a short 8-character SHA-256 content-hash suffix. Files over 100MB are linked by reference without copying unless `--copy` is explicitly passed. Use `--link` to intentionally link by reference without copying.

In headless/SSH environments without GUI, `task doc open` cleanly outputs the resolved path to stdout. Soft-deleting tasks or removing attachments retains physical files on disk; run `task doc prune` to audit and clean orphaned files.

```bash
task-cli task doc add 1 /path/to/spec.pdf              # alias: attach, doc-add (copies to attachments/1/)
task-cli task doc add 1 /path/to/spec.pdf --link       # link by reference without copying
task-cli task doc add 1 /path/to/large.iso --copy      # force copy even if > 100MB
task-cli task doc add 1 ./notes.txt "Meeting Notes"
task-cli task doc list 1                              # alias: docs, doc-list, ls
task-cli task doc remove 1                            # alias: doc-del, doc-remove, del, rm
task-cli task doc open 1                              # open task attachment directory (alias: doc-open)
task-cli task doc open --doc 1                        # reveal document in file manager
task-cli task doc prune --dry-run                     # preview orphan attachments to delete (alias: doc-prune)
task-cli task doc prune                               # delete orphan attachments and empty directories
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
