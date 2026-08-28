<!-- last_synced: 2026-08-28 -->

# Groobb

> English | [日本語](./README.ja.md)

Find your groove

Groobb is still under development and has not been released to the public.
A binary and installation steps for self-hosting are not available yet either.

[![Go CI](https://github.com/groobb/groobb/actions/workflows/go-ci.yml/badge.svg)](https://github.com/groobb/groobb/actions/workflows/go-ci.yml)

## About Groobb

Groobb is a bulletin board service you can run on your own server.
It can hold as many boards as you need, and conversations happen in the threads created inside them.
The aim is to keep installation and day-to-day operation light enough that self-hosting stays easy.

## Design

SQLite is the only database Groobb uses.
No separate database server is required, and the data fits in a single file.

Static assets, translations, and migrations are embedded in the binary.
Everything needed to run lives inside the binary, so the server does not depend on the directory it runs from.

## Related links

- [Contributing](./CONTRIBUTING.md)
- [Security reporting](./SECURITY.md)

## License

Groobb is published under the [GNU Affero General Public License v3.0](./LICENSE).
