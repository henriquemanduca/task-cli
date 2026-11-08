# config

## Overview
Package `config` discovers and parses the `.taskcli` workspace configuration file. The format is `.env`-style (`KEY=VALUE`) with `#` comments, optional `export ` prefixes, and quoted values.

## Interface
- `config.go:FileName` - Configuration file name (`.taskcli`).
- `config.go:KeyProject` - Key for the default project code (`PROJECT_CODE`).
- `config.go:Config` - Parsed workspace defaults (`Project`).
- `config.go:LoadFromDir` - Finds the nearest `.taskcli` from a directory upward, stopping at `$HOME` (inclusive), and parses it. Returns an empty config when no file exists.
- `config.go:LoadFile` - Parses a specific `.taskcli` file.

## Invariants
- The nearest file wins; if `$HOME` is an ancestor of the starting directory, the search never goes above it.
- When `$HOME` is not an ancestor (e.g. working outside home), the search walks up to the filesystem root.
- Unknown keys and lines without `=` are errors that include the file path and line number.
- Duplicate keys use the last value; an empty `PROJECT_CODE` means "no default".
- `.taskcli` is read-only: the package never creates or modifies it.

## Verification
- Run: `go test ./internal/config`
