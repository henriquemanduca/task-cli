# Centralized Storage and Managed Attachment Duplication

* **Author:** H. Manduca
* **Date:** 2026-11-08

## The Problem

`task-cli` allows users to attach documents to tasks via `task doc add` / `attach`. Currently, the system stores only raw, external absolute file paths directly in the SQLite database without copying or managing the referenced files.

This presents several limitations:
1. **Broken Links upon External Operations:** When users move, delete, or rename files within their local work directories or downloads folder, the database pointers break silently, leading to missing documents.
2. **Scattered Attachments:** Task-related assets are distributed arbitrarily across the filesystem with no centralized organization or predictable location.
3. **Configuration Exposure:** Exposing `--db` and `TASK_CLI_DB` in public documentation and help screens adds unnecessary complexity for users who need a zero-configuration CLI, while leaving attachment storage paths unstandardized.

## Overview

This feature transitions `task-cli` to a centralized storage architecture following platform standard user data directories (`$XDG_DATA_HOME` or `~/.local/share/task-cli/` on Linux/BSD, `~/Library/Application Support/task-cli/` on macOS, and `%LOCALAPPDATA%/task-cli` on Windows).

Key mechanisms include:
- Storing the primary database at `<data_root>/task-cli.db` and safely migrating legacy `~/.task-cli.db` on first launch (stream copy with `fsync`, SQLite `PRAGMA quick_check;` validation, and renaming the legacy file to `~/.task-cli.db.migrated`).
- Creating a managed attachment repository under `<data_root>/attachments/<task_id>/`, making attachment paths invariant to project changes or project code renames.
- Automatically duplicating attached files into this managed hierarchy with a short content-hash suffix (`<filename_stem>_<short_hash>.<ext>`) to prevent filename collisions while retaining human readability.
- Enforcing a 100MB copy heuristic: files exceeding 100MB trigger a warning on `stderr` and link by reference without copying, while providing explicit `--copy` (force copy) and `--link` (deliberate reference) CLI flags.
- Retaining physical copies on disk when tasks are deleted or attachments removed, preserving historical integrity, with an explicit `task doc prune [--dry-run]` command for intentional cleanup.
- Adding `task doc open <task_id>` and `task doc open --doc <doc_id>` to open the task or attachment folder in the system file manager, with clean stdout path output in headless/SSH environments.
- Removing public database selection flags from help output and CLI documentation, while retaining internal evaluation for automated integration testing.

## Goals

* **Reliable Document Integrity:** Prevent link breakage caused by external file modifications by automatically copying attached files into a managed directory.
* **Organized, Stable Directory Structure:** Partition attachments by task ID (`<data_root>/attachments/<task_id>/`), ensuring paths remain valid even if the task changes project or project code.
* **Conflict-Free Naming:** Append an 8-character hex content hash suffix before the file extension to prevent collisions when files with identical names are attached.
* **Predictable Resource Consumption:** Apply an automated 100MB threshold with `--copy` and `--link` overrides.
* **Safe Retention & Explicit Hygiene:** Retain physical files on disk by default when attachments are detached or tasks deleted, while providing `task doc prune` for intentional cleanup.
* **Ergonomic Folder Navigation:** Provide `task doc open` to open the task's storage directory in the desktop file manager or print the path in headless terminals.
* **Clean Public Interface:** Consolidate default database resolution and remove database path options from public help texts.

## Non-Goals

* **Full-Text Document Indexing:** No parsing, searching, or extraction of text contents from attachments.
* **Automated Multi-Device Synchronization:** No cloud sync or physical asset synchronization across distributed machines.
* **Binary Packaging in JSON Export:** JSON export and import (`export` / `import`) will preserve document metadata and paths, but will not embed binary payloads into JSON files.
* **Automatic Silent Deletion:** The system will not automatically delete physical files from disk upon `task delete` or `doc remove`; disk cleanup must be invoked explicitly via `task doc prune`.

## Technical Plan

### 1. Storage Architecture & Directory Layout

The application resolves the base data directory according to platform standards:
- **Linux/BSD:** `$XDG_DATA_HOME/task-cli` (defaulting to `~/.local/share/task-cli` if unset or empty).
- **macOS:** `~/Library/Application Support/task-cli`.
- **Windows:** `%LOCALAPPDATA%\task-cli`.

Within this data root:
- SQLite database: `<data_root>/task-cli.db`
- Attachments root: `<data_root>/attachments/`
- Partitioned task directory: `<data_root>/attachments/<task_id>/`

**Legacy Database Migration:**
On startup during database initialization (`db.NewStore`), if `<data_root>/task-cli.db` does not exist but legacy `~/.task-cli.db` exists:
1. Streams `~/.task-cli.db` into `<data_root>/task-cli.db` via buffered I/O and issues `fsync` (avoiding cross-device `EXDEV` errors from atomic renames).
2. Runs SQLite `PRAGMA quick_check;` on the target copy.
3. If verification passes, renames the legacy database to `~/.task-cli.db.migrated` and emits an informational message to `stderr`.
4. If copying or verification fails, aborts startup with an error without modifying the original legacy file.

### 2. Component Design & Responsibilities

```
[CLI Command Layer: internal/cmd]
        |
        +---> [Store: internal/db] (SQLite schema, transactions, metadata)
        |
        +---> [Filestore: internal/filestore] (Directory creation, reflink/copy, hash, open, prune)
```

1. **`internal/filestore` Package (New):**
   - Resolves platform data roots and attachment paths without circular dependencies.
   - Computes short SHA-256 hashes (first 8 hex characters).
   - Manages physical file duplication:
     - Checks size against the 100MB threshold unless overridden by `--copy` or `--link`.
     - Attempts opportunistic Copy-on-Write (reflink) via build-tagged files before falling back to `io.Copy`.
     - Streams bytes to a temporary file (`.tmp_<stem>_<hash>_<nonce>.<ext>`) in the destination folder before atomically renaming to `<filename_stem>_<short_hash>.<ext>` with mode `0644`.
     - Guards rollback: if DB insertion fails, deletes only newly created files, without deleting pre-existing files.
   - Implements native directory opening with fallback:
     - Linux: Dispatches to `xdg-open` if `$DISPLAY` or `$WAYLAND_DISPLAY` is present; otherwise prints the path to `ctx.Stdout`.
     - macOS: Dispatches to `open` (or `open -R` for specific files).
     - Windows: Dispatches to `explorer.exe` (or `explorer.exe /select,<path>` for specific files).
     - In headless environments or upon opener failure, prints the exact absolute path followed by `\n` to `ctx.Stdout`.
   - Implements `PruneAttachments(activePaths []string, dryRun bool) ([]string, error)` to identify and clean orphaned attachment files not present in `activePaths`, removing empty `<task_id>` directories after cleanup.

2. **`internal/db` Changes:**
   - In `DefaultDBPath()`, updates default resolution to `<data_root>/task-cli.db`, retaining test overrides via `TASK_CLI_DB` / hidden `--db` flag.
   - Implements `ListAllDocumentPaths() ([]string, error)` querying paths of active documents.
   - Triggers `MigrateLegacyDB()` within `db.NewStore()`.

3. **`internal/cmd/doc.go` & `router.go` Changes:**
   - In `task doc add <task_id> <path> [name] [--type <t>] [--copy] [--link]`:
     1. Verifies source file existence.
     2. If `--link` is provided, skips copying and stores the original path.
     3. If size > 100MB and `--copy` is not specified, prints warning to `stderr` and links by reference.
     4. Otherwise copies the file into `attachments/<task_id>/`.
     5. Inserts document record into SQLite. If insertion fails, rolls back newly created file.
   - Registers `task doc open <task_id>` (opens folder) and `task doc open --doc <doc_id>` (reveals document).
     - Declares `valueFlags = map[string]bool{"doc": true, "-doc": true}` in `handleDocOpen`.
   - Registers `task doc prune [--dry-run]` to audit and clean orphaned files.

4. **CLI Help & Documentation:**
   - Removes `--db` and `TASK_CLI_DB` from public help (`printUsage`), while preserving runtime evaluation in `cmd.ResolveContext`.

### 3. Attachment Data Flow Diagram

```mermaid
flowchart TD
    User["User: task doc add 42 ./spec.pdf"] --> CheckFlags{"Flags: --link or --copy?"}
    CheckFlags -->|--link| StoreRef["Store Original Path in SQLite"]
    CheckFlags -->|--copy or normal| CheckSize{"File Size > 100MB and no --copy?"}
    CheckSize -->|Yes| Warn["Warn on stderr: Exceeds 100MB\nFallback to Reference"]
    Warn --> StoreRef
    CheckSize -->|No| Hash["Compute 8-char Hash & Check Destination"]
    Hash --> Copy["Copy to Temp via CoW Reflink / io.Copy + Atomic Rename"]
    Copy -->|Success| DBStoreCopy["Store Copied Path in SQLite"]
    Copy -->|Fail (Disk Full)| Abort["Abort with Error (No DB record)"]
    DBStoreCopy -->|DB Fail| Rollback["Delete Newly Copied File (Rollback)"]
    DBStoreCopy -->|DB Success| Done["Report Success to User"]
```

## Alternatives Considered

1. **Raw Path Referencing Only (Status Quo):**
   - *Rejected by author:* Leaving attachments as raw external pointers leaves them perpetually vulnerable to accidental deletion, file renames, or directory movements elsewhere on the filesystem. Document links break silently without any safeguard.

2. **Database SQLite BLOB Storage:**
   - *Rejected by author:* Embedding document binary bytes directly inside SQLite `documents` table records causes massive database file bloat, degrades SQLite vacuuming/backup performance, and prevents easy inspection through standard command-line tools or native desktop file managers.

3. **Flat Content-Addressable Storage (CAS) without Directory Hierarchy:**
   - *Rejected by author:* Storing all files flatly named by their SHA-256 hash (e.g. `attachments/<sha256>`) maximizes raw deduplication, but destroys file readability and prevents human users from browsing or navigating their task files using desktop file managers (`task doc open`). Partitioning by `<task_id>/` retains human-friendly discoverability while CoW reflinks provide block deduplication.

4. **Partitioning Attachments by Mutable Project Code (`<CODE>/<task_id>/`):**
   - *Rejected by author:* When tasks change projects (`task update -p`) or project codes are updated, directory paths become desynchronized or require fragile cross-directory physical moves. Partitioning strictly by immutable task ID (`attachments/<task_id>/`) guarantees permanent path validity.

5. **Exposing Configurable Storage Paths in Public CLI Options:**
   - *Rejected by author:* Exposing database and storage path options in public help screens adds clutter to a tool designed for zero-configuration use. Standard platform data directories provide predictable locations while internal flags retain test flexibility.

## Detailed Implementation

The implementation is structured across discrete files with strict rationale and boundaries:

### 1. New Package: `internal/filestore/`
- **`internal/filestore/paths.go`**:
  - Centralizes platform data root resolution (`XDG_DATA_HOME` / macOS `Application Support` / Windows `%LOCALAPPDATA%`).
  - Implements `BaseDir() (string, error)`.
  - Implements `DefaultDBPath() (string, error)`.
  - Implements `TaskAttachmentDir(taskID int64) (string, error)` resolving `<BaseDir>/attachments/<task_id>/`.
  - Implements `CopyLegacyDB(src, dst string) error` to stream-copy `~/.task-cli.db` with `fsync`.
  - Implements `FinalizeLegacyDBMigration(src string) error` to rename `src` to `src + ".migrated"`.
- **`internal/filestore/reflink_linux.go`**:
  - Build tag: `//go:build linux`
  - Implements opportunistic `cloneFile(src, dst *os.File) error` using `unix.SYS_IOCTL` with `FICLONE = 0x40049409` (`CGO_ENABLED=0` compliant).
- **`internal/filestore/reflink_darwin.go`**:
  - Build tag: `//go:build darwin`
  - Implements opportunistic `cloneFile(src, dst *os.File) error` using `syscall.Syscall` with `clonefile(2)`.
- **`internal/filestore/reflink_other.go`**:
  - Build tag: `//go:build !linux && !darwin`
  - Implements `cloneFile(src, dst *os.File) error` returning `ErrUnsupported`.
- **`internal/filestore/copy.go`**:
  - Defines `MaxCopySize = 100 * 1024 * 1024` (100MB).
  - Implements `cloneFileOrCopy(srcPath, dstPath string) error` attempting `cloneFile` before streaming via `io.Copy`.
  - Computes 8-char SHA-256 hash suffix.
  - Implements `SaveAttachment(srcPath string, taskID int64, forceCopy, forceLink bool) (savedPath string, copied bool, err error)` with `.tmp_<stem>_<hash>_<nonce>.<ext>` staging.
  - Implements `RemoveFile(filePath string) error` for transaction failure rollback.
- **`internal/filestore/open.go`**:
  - Implements `OpenFolder(path string) (string, error)` and `OpenFile(path string) (string, error)`.
  - Dispatches to desktop file manager if GUI session exists, or prints formatted path to stdout in headless environments.
- **`internal/filestore/prune.go`**:
  - Implements `PruneAttachments(activePaths []string, dryRun bool) ([]string, error)` to scan `attachments/` and clean unreferenced files, removing empty `<task_id>` directories after cleanup.
- **`internal/filestore/filestore_test.go`**:
  - Unit tests for path resolution, CoW fallback, rollback, pruning, and migration.

### 2. Database Modifications: `internal/db/`
- **`internal/db/db.go`**:
  - Updates `DefaultDBPath()` to call `filestore.DefaultDBPath()` while preserving `TASK_CLI_DB` env override for test execution.
  - Adds `ListAllDocumentPaths() ([]string, error)` querying paths of active documents from SQLite.
  - In `NewStore()`, if target database does not exist and legacy database exists, coordinates `filestore.CopyLegacyDB`, validates target with `PRAGMA quick_check;`, and calls `filestore.FinalizeLegacyDBMigration`.
- **`internal/db/db_test.go`**:
  - Verifies that database initialization honors default paths, test overrides, and `ListAllDocumentPaths()`.

### 3. Command Routing & Handlers: `internal/cmd/`
- **`internal/cmd/doc.go`**:
  - Modifies `handleDocAdd` to support `--copy` and `--link` flags, calls `filestore.SaveAttachment`, and executes guarded rollback on failure.
  - Implements `handleDocOpen(ctx *Context, args []string) error` supporting task ID or `--doc <doc_id>`, declaring `valueFlags = map[string]bool{"doc": true, "-doc": true}`.
  - Implements `handleDocPrune(ctx *Context, args []string) error` querying `store.ListAllDocumentPaths()` and calling `filestore.PruneAttachments`.
- **`internal/cmd/router.go`**:
  - Registers `task doc open` and `task doc prune`.
- **`internal/cmd/alias.go`**:
  - Registers flat aliases: `"doc-open": {"doc", "open"}` and `"doc-prune": {"doc", "prune"}`.
- **`internal/cmd/doc_test.go` & `internal/cmd/router_test.go`**:
  - Tests for `task doc add` with `--copy`/`--link`, rollback, `task doc open`, and `task doc prune`.

### 4. Entrypoint and Documentation Updates
- **`main.go`**:
  - Removes `--db` and `TASK_CLI_DB` from `printUsage()`.
  - Adds `task doc open` and `task doc prune` lines to subcommands overview.
- **`README.md`**:
  - Documents platform data directories, attachment copying, `--copy`/`--link` flags, `task doc open`, and `task doc prune`.
- **`skill/task-cli.md`**:
  - Synchronizes agent skill with updated subcommands and storage behaviors.
- **`AGENTS.md`**:
  - Updates project rules with new paths and testing notes.
