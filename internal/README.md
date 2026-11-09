# internal

## Overview
The `internal` directory groups the private implementation packages for `task-cli`: workspace configuration loading (`config`), the command-line router and action handlers (`cmd`), the SQLite persistence layer (`db`), and the physical file/attachment storage engine (`filestore`).

## Subdirectories & Boundaries
- `config`: [./config/README.md](./config/README.md) - Discovery and parsing of the `.taskcli` workspace defaults file.
- `cmd`: [./cmd/README.md](./cmd/README.md) - Command-line interface routing, flat alias mapping, transversal flag handling, and command execution handlers.
- `db`: [./db/README.md](./db/README.md) - Embedded SQLite storage layer, schema migrations, CRUD operations, and JSON transfer import/export.
- `filestore`: [./filestore](./filestore) - Platform standard data directory resolution, CoW reflink and managed attachment duplication, content hashing, folder opening, and orphaned attachment pruning.
