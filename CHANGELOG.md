# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- `ResolveVersion`: bump patch part of the resolved version.

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

[Unreleased]: https://github.com/giantswarm/architect-orb/compare/v0.1.1...HEAD
[0.1.1]: https://github.com/giantswarm/architect-orb/releases/tag/v0.1.1
[0.1.0]: https://github.com/giantswarm/architect-orb/releases/tag/v0.1.0
