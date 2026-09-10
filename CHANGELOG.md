# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/), and this project adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- **Breaking:** the module path moved to `github.com/giantswarm/gitsemver/v3`, as the removals below break the
  library API. Importers must update their import paths; users of the CLI must run
  `go install github.com/giantswarm/gitsemver/v3@latest`.
- **Breaking:** dev build versions use the new schema from
  [RFC #157](https://github.com/giantswarm/rfc/pull/157):
  `X.Y.Z-r<CRC32-of-branch>t<YYYYMMDDHHMMSS>h<commit-sha>`, e.g.
  `1.9.2-r7b5b4fa7t20260127094959h1a2b3c4`. The pre-release part is always 33 characters and holds no `.` and
  no `-`, so a chart that concatenates the version into a Kubernetes label and trims the result can no longer
  cut the pre-release part on a character Kubernetes rejects — only a prefix long enough to push the cut into
  the `X.Y.Z` part can still do that
  ([giantswarm#37079](https://github.com/giantswarm/giantswarm/issues/37079)). The branch is identified by the
  CRC-32/ISO-HDLC checksum of its full, unsanitized name instead of the sanitized name itself, so no part of
  the tag is truncated any more. The separators differ from the RFC, which spells them `b`, `t` and `c`:
  `b` and `c` are hex digits and hide the field boundaries, so this tool uses `r` (ref), `t` (time) and
  `h` (hash). The field order, the widths, the CRC variant and the 33-character total are unchanged.
  [RFC PR #159](https://github.com/giantswarm/rfc/pull/159) amends the RFC to match.
- `validate --type dev` accepts both the new schema and the superseded
  `X.Y.Z-dev.<branch>.<YYYY-MM-DD>.<HH-MM-SS>[.h<commit-sha>]` one, because tags in the old format are already
  published. `get` only ever generates the new one.

  Note the sort order: a new tag sorts **above** an old one at the same `X.Y.Z`, because `r` > `d` in the
  first pre-release identifier, so a consumer moves to the new schema at once. Against an `-rc.N` tag at the
  same base the order depends on the first digit of the branch hash (`0` to `b` below the RC, `c` to `f`
  above it), so select dev builds with `semverFilter: ".*-r<branch-hash>t.*"` rather than a bare range.

### Added

- `gitsemver branch-hash [branch]` prints the 8-hex CRC32 fingerprint that a dev build embeds for a branch,
  for use in a Flux `semverFilter`. Without an argument it uses the current branch. The library equivalents
  are `gitsemver.BranchHash` and `gitsemver.CurrentBranch`.

### Removed

- **Breaking:** `GS_MAX_VERSION_LENGTH` and `Config.MaxVersionLength`. The generated version can no longer
  overflow, so there is nothing left to bound or shorten.
- **Breaking:** branch name sanitizing and middle-truncation (the `--` marker) in dev versions.

## [2.0.1] - 2026-06-08

### Fixed

- When a single commit reachable from HEAD carries multiple version tags, `get` and `next` no longer fail. The
  highest tag in semver terms is chosen and a warning listing all discovered tags and the chosen one is
  printed to stderr. commit) for tag-to-commit traceability, e.g.
  `1.2.4-dev.my-feature.2026-01-27.09-49-59.h1a2b3c4`.
- Generated dev versions are bounded to a maximum length (default 63, configurable via `GS_MAX_VERSION_LENGTH`
  or `Config.MaxVersionLength`) so they stay usable as Kubernetes/DNS attributes. Only the branch part is
  shortened to fit: its middle is dropped and replaced with a `--` marker, keeping the head and the more
  distinctive tail (e.g. `renovate-up--s-to-latest`). The base, timestamp and commit hash are always kept
  intact, so per-branch chronological sort order is unaffected. To aid with telling shortened clashing
  branches from each other, the dev build versions now carry a trailing `.h<commit-sha>` segment (7-char git
  short hash of the resolved

### Changed

- Branch names embedded in dev versions are now lowercased and reduced to the DNS-safe set `[a-z0-9-]` (hyphen
  runs collapsed, ends trimmed). Purely-numeric branch names have leading zeros stripped so the segment is a
  valid semVer numeric identifier (e.g. `0042` -> `42`).
- Release binaries now include darwin/amd64, darwin/arm64, windows/amd64, and windows/arm64 alongside the
  existing linux targets. Windows binaries are named `gitsemver-windows-<arch>.exe`.

## [2.0.0] - 2026-06-02

### Changed

- Renamed the `version` subcommand to `get` to avoid confusion with the `--version` flag. Use `gitsemver get`
  to resolve and print the version for a git ref.
- Dev build versions now derive their timestamp from the committer date of the resolved commit (in UTC)
  instead of the current wall-clock time. This makes the version deterministic: resolving the same commit
  always yields the same `X.Y.Z-dev.<branch>.<date>.<time>` string.

## [1.1.2] - 2026-05-28

### Fixed

- `--version` flag for `gitsemver` self-info was not working

## [1.1.1] - 2026-05-27

### Added

- Shell completion scripts for bash, zsh, fish, and PowerShell via `gitsemver completion <shell>`. Bump types
  for `next` and type values for `validate --type` are completed automatically.

## [1.1.0] - 2026-05-27

### Added

- `gitsemver next <bump-type> [--last-tag <tag>]` subcommand that computes the next semver release tag from
  the highest-semver reachable ancestor tag (or an explicit `--last-tag` value). Bump types from a stable tag:
  `patch`, `minor`, `major`, `patch-rc`, `minor-rc`, `major-rc`. Bump types from an RC tag: `rc` (increment
  counter), `rc-release` (finalize to stable). `--last-tag` bypasses git entirely, making the command usable
  outside a repository.

### Fixed

- Multiple version tags on the same commit now produce an error instead of silently returning a
  non-deterministic result (`buildVersionMaps` guard was previously unreachable).

## [1.0.2] - 2026-05-26

### Fixed

- calling `gitsemver` now correctly returns usage info; to get a version, the `version` subcommand must be
  used: `gitsemver version`

## [1.0.1] - 2026-05-25

### Added

- `gitsemver version` subcommand that prints the build version, git SHA, and build timestamp. Version defaults
  to `devel` between releases and is updated to the release tag in source before each release. Git SHA and
  build timestamp are injected at link time via ldflags.

## [1.0.0] - 2026-05-21

### Changed

- feat!: Repository and Go module renamed from `gitrepo` to `gitsemver`. CLI binary renamed from
  `gitrepo-version` to `gitsemver`. Import path changed to `github.com/giantswarm/gitsemver/pkg/gitsemver`.
- feat: implement the tagging RFC
  (https://github.com/giantswarm/rfc/tree/main/semver-based-automatic-upgrades)
- feat: add tag validation API

## [0.3.5] - 2026-05-13

### Changed

- Dependency updates

## [0.3.4] - 2026-02-10

### Changed

- Dependency updates

## [0.3.3] - 2025-12-08

### Changed

- Dependency updates

## [0.3.2] - 2025-03-13

## Changed

- Errors have been made public

## [0.3.1] - 2025-01-21

- Dependency updates

## [0.3.0] - 2024-08-01

### Added

- Add support for git tag prefixes in version calculation logics. If the `GS_GIT_TAG_PREFIX` environment
  variable is set to e.g. `mymodule-a` then tags like `mymodule-a/v1.2.3` will be looked for in the history
  instead of the normal semantic versioning tags, when the env var is not set (default). New tags will be
  generated in the same format and with the same logic tho. For the above example, a few commits ahead of that
  tag the new version in a test build would be: `1.2.3-<GIT_HASH>`. When on the tag itself, it would be:
  `1.2.3`. When no tag found with the given prefix, then it would be: `0.0.0-<GIT_HASH>`. This replicates the
  original behaviour, just the tag looked up for reference changes in the behaviour. This enables creating
  sort of mono repositories where multiple modules, libraries or smaller projects are stored in a single repo
  that needs to be versioned separately.

## [0.2.4] - 2024-06-03

### Changed

- Dependency updates

## [0.2.3] - 2023-09-29

### Changed

- Upgrade go-git and go-billy dependencies to their new location. Moving from github.com/src-d to
  github.com/go-git. v4 to v5 is a drop-in replacement, see
  https://github.com/go-git/go-git/releases/tag/v5.0.0

## [0.2.2] - 2021-04-16

### Fixed

- Clean after checkout of repo to avoid leaking of folders/files.

## [0.2.1] - 2021-01-21

### Fixed

- Reading files from default branch after calling `EnsureUpToDate` on empty repo

## [0.2.0] - 2021-01-15

### Added

- Add `GetFolderContent` which fetches the contents of a folder.

## [0.1.2] - 2020-07-24

### Added

- Introduce new `IsRepositoryNotFound` error matcher

## [0.1.1] - 2020-03-17

### Added

- Add `EnsureUpToDate`: fetches latest changes from remote.
- Add `GetFileContent`: retrieves content of file.
- Add `HeadBranch`: returns branch name for the HEAD ref.
- Add `HeadSHA`: returns sha for the HEAD ref.
- Add `HeadTag`: returns tag for the HEAD ref.
- Add `ResolveVersion`: resolves version of a reference.
- Add `TopLevel`: finds absolute path of top-level git directory.

## [0.1.0] - 2019-10-10

### Added

- Functions signature for `EnsureUpToDate` and `ResolveVersion`.

[Unreleased]: https://github.com/giantswarm/gitsemver/compare/v2.0.1...HEAD
[2.0.1]: https://github.com/giantswarm/gitsemver/compare/v2.0.0...v2.0.1
[2.0.0]: https://github.com/giantswarm/gitsemver/compare/v1.1.2...v2.0.0
[1.1.2]: https://github.com/giantswarm/gitsemver/compare/v1.1.1...v1.1.2
[1.1.1]: https://github.com/giantswarm/gitsemver/compare/v1.1.0...v1.1.1
[1.1.0]: https://github.com/giantswarm/gitsemver/compare/v1.0.2...v1.1.0
[1.0.2]: https://github.com/giantswarm/gitsemver/compare/v1.0.1...v1.0.2
[1.0.1]: https://github.com/giantswarm/gitsemver/compare/v1.0.0...v1.0.1
[1.0.0]: https://github.com/giantswarm/gitsemver/compare/v0.3.5...v1.0.0
[0.3.5]: https://github.com/giantswarm/gitsemver/compare/v0.3.4...v0.3.5
[0.3.4]: https://github.com/giantswarm/gitsemver/compare/v0.3.3...v0.3.4
[0.3.3]: https://github.com/giantswarm/gitsemver/compare/v0.3.2...v0.3.3
[0.3.2]: https://github.com/giantswarm/gitsemver/compare/v0.3.1...v0.3.2
[0.3.1]: https://github.com/giantswarm/gitsemver/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/giantswarm/gitsemver/compare/v0.2.4...v0.3.0
[0.2.4]: https://github.com/giantswarm/gitsemver/compare/v0.2.3...v0.2.4
[0.2.3]: https://github.com/giantswarm/gitsemver/compare/v0.2.2...v0.2.3
[0.2.2]: https://github.com/giantswarm/gitsemver/compare/v0.2.1...v0.2.2
[0.2.1]: https://github.com/giantswarm/gitsemver/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/giantswarm/gitsemver/compare/v0.1.2...v0.2.0
[0.1.2]: https://github.com/giantswarm/gitsemver/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/giantswarm/architect-orb/releases/tag/v0.1.1
[0.1.0]: https://github.com/giantswarm/architect-orb/releases/tag/v0.1.0
