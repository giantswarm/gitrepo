# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.3] - 2023-09-29

### Changed

- Upgrade go-git and go-billy dependencies to their new location.
  Moving from github.com/src-d to github.com/go-git.
  v4 to v5 is a drop-in replacement, see https://github.com/go-git/go-git/releases/tag/v5.0.0

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

[Unreleased]: https://github.com/giantswarm/gitrepo/compare/v0.2.3...HEAD
[0.2.3]: https://github.com/giantswarm/gitrepo/compare/v0.2.2...v0.2.3
[0.2.2]: https://github.com/giantswarm/gitrepo/compare/v0.2.1...v0.2.2
[0.2.1]: https://github.com/giantswarm/gitrepo/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/giantswarm/gitrepo/compare/v0.1.2...v0.2.0
[0.1.2]: https://github.com/giantswarm/gitrepo/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/giantswarm/architect-orb/releases/tag/v0.1.1
[0.1.0]: https://github.com/giantswarm/architect-orb/releases/tag/v0.1.0
