# Release schedule

## Cadence

- Planned release train: fortnightly on Tuesdays.
- Emergency patch releases: as needed for production-impacting regressions.

## Versioning policy

This project uses Semantic Versioning (`MAJOR.MINOR.PATCH`).

- Patch: bug fixes and refactors with no new user-facing capability.
- Minor: backwards-compatible features.
- Major: breaking changes.

## Cut process

1. Ensure `main` is green in CI on Linux and macOS.
2. Move entries from `[Unreleased]` to a new dated version section in
   `CHANGELOG.md`.
3. Create and push a signed tag in the form `vX.Y.Z`.
4. Let release workflows publish platform archives from the tag.
