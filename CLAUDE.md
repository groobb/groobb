<!-- last_synced: 2026-05-30 -->

# Groobb Development Guide

> English | [日本語](./CLAUDE.ja.md)

This file provides guidance for Claude Code when working in this repository.

## Overview

Groobb is a bulletin board service.
Users can create their own bulletin boards and interact with one another.

## Project Structure

This repository mainly manages a service implemented in Go.

```
/workspace/
├── go/                  # Service implemented in Go
├── caddy/               # Reverse proxy configuration
├── docs/                # Groobb-specific documentation (ADRs, work plans, etc.)
├── .github/             # CI/CD configuration
├── Dockerfile.dev       # Dockerfile for the development container
├── docker-compose.yml   # Docker Compose configuration
└── CLAUDE.md            # This file (project-wide guide)
```

## Development with Feature Flags

Groobb controls feature rollout using **feature flags** rather than feature branches. Pre-release features are developed with their flag turned off, and are exposed by switching the flag on once they are ready for production.

## Development Workflow

### Implementation Guidelines

**Consistency with existing code**:

Before implementing, check whether similar logic already exists in the codebase.
If it does, follow that pattern to keep the codebase consistent as a whole.

### Post-Implementation Checks

Before reporting that your work is complete, always verify the following:

- Code formatting
- Lint
- Tests

The commands to run are managed in the `Makefile`.
See [Makefile](./Makefile) and [go/Makefile](./go/Makefile).

## Language and Writing Rules

- **English is the original; the writing workflow starts in Japanese**: The English version is the official source of truth. Write in Japanese first, then translate to English (with Claude Code's help), and after translating, review the English version as well to check for shifts in intent or unnatural wording. When there is a discrepancy, English takes precedence.
- **Code comments**: English block → blank line → Japanese block prefixed with the `[Ja]` marker. Short comments may be written on a single line, e.g. `# Returns ... / [Ja] ... を返す`.
- **Markdown documents**: Manage `xxx.md` (English, original) and `xxx.ja.md` (Japanese translation) in parallel. Place a `<!-- last_synced: YYYY-MM-DD -->` HTML comment on the first line of both files and keep the sync dates aligned.
- **Commit messages**: English title + English body + blank line + Japanese body prefixed with the `[Ja]` marker. Do not keep a Japanese title (prioritize English scannability of `git log --oneline`).
- **Identifiers**: Type, function, and variable names are in English only.
- **When you change one side, update the other in the same commit**: This prevents translation drift.
- **Existing code**: These rules apply from new writing onward. Existing single-language code is made bilingual when you edit or change it (no bulk migration needed).
