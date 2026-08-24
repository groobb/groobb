<!-- last_synced: 2026-08-22 -->

# Groobb Development Guide

> English | [日本語](./AGENTS.ja.md)

This file provides guidance for coding agents when working in this repository.

## Overview

Groobb is a bulletin board service.
Users can create their own bulletin boards and interact with one another.

An **instance** is one running Groobb: a single process together with the SQLite file and the data it owns.
An instance serves exactly one community, so a single server may run several instances.
"Instance" never refers to the server itself.

## Project Structure

This repository mainly manages a service implemented in Go.

```
/workspace/
├── go/                  # Service implemented in Go
├── caddy/               # Reverse proxy configuration
├── docs/                # Documentation (ADRs, work plans, etc.; mounted from other repos and gitignored)
├── .claude/             # Coding agent settings (rules and skills; mounted from other repos and gitignored)
├── .github/             # CI/CD configuration
├── Dockerfile.dev       # Dockerfile for the development container
├── docker-compose.yml   # Docker Compose configuration
├── AGENTS.md            # This file (project-wide guide)
└── CLAUDE.md            # Pointer to AGENTS.md (for Claude Code)
```

## Development with Feature Flags

Groobb controls feature rollout using **feature flags** rather than feature branches.
Pre-release features are developed with their flag turned off, and are exposed by switching the flag on once they are ready for production.

## Development Workflow

### Consistency with Existing Code

Before implementing, check whether similar logic already exists in the codebase.
If it does, follow that pattern to keep the codebase consistent as a whole.
Note that established conventions of the programming language and its ecosystem, as well as documented design improvements, take precedence over local consistency.

### Post-Implementation Checks

Before reporting that your work is complete, always verify the following:

- Code formatting
- Lint
- Tests

The commands to run are managed in the `Makefile`.
See [Makefile](./Makefile) and [go/Makefile](./go/Makefile).

## Language and Writing Rules

### English is the original; the writing workflow starts in Japanese

The English version is the official source of truth.
Write in Japanese first, then translate to English (with a coding agent's help), and after translating, review the English version as well to check for shifts in intent or unnatural wording.
When there is a discrepancy, English takes precedence.

### Code Comments

English block → blank line → Japanese block prefixed with the `[Ja]` marker.
The `[Ja]` marker always sits at the start of a line, so inline (end-of-line) bilingual comments are not used — write the two blocks on their own lines above the code.
Comments explain why the code is the way it is; let the code itself say what it does, and keep implementation history in the git log.

### Markdown Documents

Manage `xxx.md` (English, original) and `xxx.ja.md` (Japanese translation) in parallel.
Place a `<!-- last_synced: YYYY-MM-DD -->` HTML comment on the first line of both files and keep the sync dates aligned.
The bilingual pair is required only for documents published externally; non-public ones such as those under `docs/private/` may be written in Japanese only.

### Commit Messages

English title + optional body (English body + blank line + Japanese body prefixed with the `[Ja]` marker).
Do not keep a Japanese title (prioritize English scannability of `git log --oneline`).

### Identifiers

Type, function, and variable names are in English only.

### Japanese Text Style

In Japanese text, use half-width parentheses with a half-width space on both sides (e.g. `テスト (テスト) テスト`).
Where the parenthesis meets a line boundary or Japanese punctuation, no space is needed on that side.

### When you change one side, update the other in the same commit

This prevents translation drift.

### Existing Code and Documents

These rules apply from new writing onward.
Existing single-language code and documents are made bilingual when you edit or change them (no bulk migration needed).
"Edit or change" means actually modifying the comment or document itself; editing only the adjacent code does not count.
New comments added to an existing file follow these rules even when the surrounding comments do not; a mix of styles within one file is acceptable.

## Coding Conventions

### Settings

Settings come from a TOML configuration file and from environment variables, and an environment variable holding a non-empty value wins over the file.
For environment variables defined by Groobb, always prefix them with `GROOBB_` (except those required by external libraries).

Adding one setting means updating all of the following together:

- the `Config` field and its resolution through `newSetting` in `Load` ([internal/config](./go/internal/config/config.go))
- the `fileConfig` field ([internal/config/file.go](./go/internal/config/file.go))
- the commented entry in [groobb.example.toml](./go/groobb.example.toml)
- the entry in `Config.LogValue`, for a secret

Keep the environment variable name and the file key one to one, with the component the variable names becoming the table (`GROOBB_SMTP_HOST` is `host` under `[email.smtp]`).
Reject invalid values and missing required settings at startup.
Use the methods of `setting` so an invalid-value error names the value's actual source, while a missing-setting error names both accepted inputs.
Keep the example file in step with the schema, which rejects unknown keys: a sample that drifts from it stops the instance of whoever copied it.

Never log a secret.
`Config.LogValue` renders a configured secret as `[REDACTED]`, and a parse error from the configuration file drops the library's message, which can quote the file.

### Embedded Resources

Static assets, locales, and migrations ship inside the binary through `embed.FS`, so that they are read without depending on the directory the server runs from.
A self-hosted instance runs from the binary alone, so reading any of them from disk breaks that premise.
For the embedding, see [static](./go/static/static.go) and [db](./go/db/migrations.go).

### Groobb-Owned SQLite Schema

These conventions apply only to the application schema owned by Groobb.
Migrations owned by River retain their upstream declarations and are outside the scope of this section.

- Primary keys are `INTEGER PRIMARY KEY`, without `AUTOINCREMENT`
- Timestamp columns are declared `DATETIME` and boolean columns `BOOLEAN`
- Timestamps hold ISO8601 UTC at a fixed width
- When a column needs a database-generated current timestamp default, use `strftime('%Y-%m-%dT%H:%M:%fZ', 'now')`
- Use `COLLATE NOCASE` for case-insensitive uniqueness only when the column's values are restricted to ASCII; choose a Unicode-aware approach for values that allow Unicode
- A list-valued column is `TEXT` holding a JSON array, guarded by `json_valid` and `json_type` checks

The reasoning behind these conventions is in the comment at the top of the [initial schema](./go/db/migrations/20260821075404_create_initial_schema.sql).
The River exception is documented in the [River migration](./go/db/migrations/20260821101022_create_river_tables.sql).

### Reading and Writing SQLite

Pass a timestamp to a query through `sqlitetime.Time`; never bind a plain `time.Time`.
Columns declared `DATETIME` already carry this type through a sqlc override, so following the generated code is usually enough.
To read the stored text itself, use an expression that has no declared type (`CAST(x AS TEXT)`).

Write through the `Writer` of `database.DB` and read through its `Reader`.
A repository holds two `*query.Queries` and picks between them per method, while a UseCase takes only the write pool, as `writer *sql.DB`.
Open application connections through `database.Open`; do not call `sql.Open` directly in production code.
Do not construct an application write connection without the shared PRAGMAs and `_txlock=immediate`.

For the format and the types see [internal/sqlitetime](./go/internal/sqlitetime/sqlitetime.go), and for the pool setup see the `DB` and DSN builder comments in [internal/database](./go/internal/database/database.go).

## Working with Coding Agents

- Converse with the user in Japanese
- Do not create a branch, commit, or `git push` to a remote until the user explicitly asks for it

## Local Notes

If `AGENTS.local.md` exists at the repository root, read it as well.
It is not tracked in this repository, and it points to the detailed guidelines available in the local development environment.
