# internal

## Overview
The `internal` directory groups the private implementation packages for `task-cli`: the command-line router and action handlers (`cmd`) and the SQLite persistence layer (`db`).

## Subdirectories & Boundaries
- `cmd`: [./cmd/README.md](./cmd/README.md) - Command-line interface routing, flat alias mapping, transversal flag handling, and command execution handlers.
- `db`: [./db/README.md](./db/README.md) - Embedded SQLite storage layer, schema migrations, CRUD operations, and JSON transfer import/export.
