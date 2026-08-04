# AGENTS.md — pdrest-go

Typed Go client for the PalDefender REST API.

## Conventions

- **Language:** Go 1.25+
- **Module:** `github.com/tagamer-net/pdrest-go` (package `pdrest`)
- **Library only:** no CLI, no config loading, no env vars. Constructors take
  everything explicitly (URL, credentials, options). Environment reads are the
  caller's responsibility.
- **Single package:** all helpers are unexported and live with the client
  code — keep the module standalone with no external dependencies.
- PalDefender REST status enrichment is deferred: keep `APIError`
  structured (StatusCode/Method/Path/ResponseBody); consumers sanitize.
- **64-bit target:** `int64` inputs are converted to `int` without a range
  check. Do not reintroduce an int64 range guard in `asInt`; the coverage
  gate in `make check` runs on the native architecture.

## Coding conventions

- Development entirely in English; no code comments unless necessary.
- Prefer `errors.New` over `fmt.Errorf` for static strings; use `%w` when wrapping.
- All public functions/types need Go doc comments.
- Tests: standard `testing` package, table-driven, `*_test.go` alongside source.
- Run `make check` before committing (lint + test).

## Commit guidelines

Conventional Commits (`feat:`, `fix:`, `refactor:`, `chore:`, `docs:`, `test:`),
one logical change per commit, `make check` before committing.

## Versioning

Published with semver tags (currently `v0.1.1`). Breaking changes bump the
minor version while on `v0.x`.
