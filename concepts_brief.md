# SQLite JSON Metadata Column for Tasks & Projects

* **Author:** H. Manduca
* **Date:** 2026-11-07

## Overview

Introduce a `metadata TEXT DEFAULT '{}'` column to both `tasks` and `projects` tables in SQLite to store dynamic domain attributes and extensible properties (such as custom labels, external links, context tags, and eventual sub-entity data) without requiring schema DDL alterations for each new field. The Go models and DTOs will act as the bridge, unmarshaling/marshaling dynamic attributes while keeping relational core columns (`id`, `project_id`, `status`, `priority`, timestamps, soft-delete) explicit in SQL for indexing, filtering, and foreign key integrity.

## Analysis

### 1. Contrarian / YAGNI Lens
* SQLite schema alterations for adding columns (`ALTER TABLE ADD COLUMN`) in pure Go (`modernc.org/sqlite`) are already lightweight, fast, and supported natively without locking issues for local single-user CLI usage.
* Storing generic metadata inside a JSON column introduces dual-state drift: developers must decide whether a field belongs in SQL or JSON. If a JSON field later needs sorting or filtering, query performance and complexity increase.
* **Verdict:** Keep core domain fields explicitly typed. Only unstructured, optional, or third-party extension attributes belong in `metadata`.

### 2. Cost, Quota & Token Economics
* Single-binary local SQLite CLI incurs zero cloud API cost or token consumption.
* SQLite JSON functions (`json_extract`, `json_set`) parse textual JSON on demand. For CLI task volumes (< 100,000 tasks), in-memory JSON parsing in Go or SQLite JSON functions have negligible CPU and memory footprints.

### 3. UX & Developer Ergonomics
* **CLI UX:** Command flags (`task add`, `task update`) should maintain typed CLI ergonomics (e.g. `--meta key=value` or dedicated flags mapped to Go struct fields).
* **Output / Formatting:** TSV output format requires strict single-line cleanliness (`cleanOneLine`). Complex nested JSON must not leak raw line breaks or tabs into TSV list views.
* **Developer Ergonomics:** Working with Go `map[string]any` or a typed `Metadata` struct requires explicit null handling (`COALESCE(metadata, '{}')`) and clean JSON serialization in database scanning.

### 4. Migration & Backward Compatibility
* Existing user databases (`~/.task-cli.db`) lack the `metadata` column.
* `NewStore` must run an idempotent migration:
  ```sql
  ALTER TABLE projects ADD COLUMN metadata TEXT NOT NULL DEFAULT '{}';
  ALTER TABLE tasks ADD COLUMN metadata TEXT NOT NULL DEFAULT '{}';
  ```
  (checking column existence via `PRAGMA table_info` before executing to remain safe on SQLite versions).
* JSON import/export (`TransferData`, version `CurrentSchemaVersion = 1`):
  Adding `metadata` to `TaskTransfer` and `ProjectTransfer` needs backward-compatible JSON tags (`omitempty`), preserving compatibility with existing export backups.

### 5. Observability & SRE Debuggability
* Local CLI tool with no background daemon.
* Corruption or malformed JSON in `metadata` could cause Go unmarshaling errors during `task list` or `task show`. Graceful degradation is required: invalid JSON in `metadata` should fall back to an empty map with a logged warning rather than failing the entire command.

### 6. Long-Term Tech Debt & Maintenance
* Sub-tasks stored inside parent task JSON blobs risk concurrency loss and lose relational integrity (e.g., cannot enforce foreign keys, document attachments to subtasks, or independent completion timestamps cleanly).
* Decision: Sub-tasks should NOT be forced into the metadata blob at this stage. Keep `metadata` strictly for auxiliary attributes/tags.

### 7. Inventive Catalyst (Contradiction Elimination)
* **Contradiction:** Need schema flexibility to add arbitrary attributes without DDL changes vs. Need strict typing, query performance, and relational integrity.
* **Resolution (Separation by Domain Role):**
  - Rigid Backbone (Relational Columns): `id`, `project_id`, `status`, `priority`, `created_at`, `updated_at`, `deleted_at`.
  - Flexible Extremities (JSON Column): `metadata TEXT DEFAULT '{}'` holding optional tags, external references, and pluggable properties.
  - Go Struct Mapping: Provide a strongly-typed `Metadata` struct for recognized fields alongside an `Extra map[string]any` bucket for custom extensions.

## Goals vs. Non-Goals

* **In-Scope Goals:**
  - Add `metadata TEXT NOT NULL DEFAULT '{}'` to `projects` and `tasks` tables via idempotent schema initialization.
  - Update Go `Project` and `Task` structs with typed `Metadata` fields handling JSON serialization/deserialization.
  - Support storing and viewing custom key/value metadata in task view and project details.
  - Update `internal/db/transfer.go` (import/export) to serialize and deserialize `metadata`.
* **Out-of-Scope Non-Goals:**
  - Do not implement embedded sub-tasks inside JSON in this iteration.
  - Do not migrate existing core relational columns (`status`, `priority`, `title`, `description`) into JSON.
  - Do not replace relational document attachments with JSON metadata.

## Alternatives Weighed

1. **Full Document Store (All task attributes inside single JSON blob):**
   - *Rejected:* Breaks indexed queries (`status = 'pending'`), ordering, foreign key cascaded deletes, and simple SQL filters; increases fragility of simple CLI queries.
2. **Dynamic EAV (Entity-Attribute-Value) Tables:**
   - *Rejected:* Introduces multi-table JOIN overhead, complex migrations, and excessive boilerplate for a lightweight CLI.
3. **Pure Schema DDL Alterations for Every Field:**
   - *Rejected by Author:* Explicit goal is to avoid recurrent DDL migrations for ad-hoc and secondary fields.

## Key Decisions & Staging for Next Steps

1. **Storage Format:** Standard JSON text in SQLite column `metadata TEXT NOT NULL DEFAULT '{}'`, validated via `json_valid(metadata)` check constraint where supported.
2. **Go Representation:**
   ```go
   type TaskMetadata struct {
       Tags  []string          `json:"tags,omitempty"`
       Extra map[string]string `json:"extra,omitempty"`
   }
   ```
3. **Migration Strategy:** Dynamic detection using `PRAGMA table_info(tasks)` in `db.NewStore` to conditionally execute `ALTER TABLE ... ADD COLUMN metadata TEXT NOT NULL DEFAULT '{}'`.
4. **Import/Export:** Maintain schema version 1 compatibility by treating `metadata` as optional in `TaskTransfer` and `ProjectTransfer`.
