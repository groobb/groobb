<!-- last_synced: 2026-08-02 -->

# Groobb Development Guide

> English | [日本語](./AGENTS.ja.md)

This file provides guidance for coding agents when working in this repository.

## Overview

Groobb is a bulletin board service.
Users can create their own bulletin boards and interact with one another.

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

- For environment variables defined by Groobb, always prefix them with `GROOBB_` (except those required by external libraries)

## Working with Coding Agents

- Converse with the user in Japanese
- Do not create a branch, commit, or `git push` to a remote until the user explicitly asks for it

## Local Notes

If `AGENTS.local.md` exists at the repository root, read it as well.
It is not tracked in this repository, and it points to the detailed guidelines available in the local development environment.
