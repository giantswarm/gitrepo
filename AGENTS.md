# gitsemver — project guide for AI agents

## What this project does

`gitsemver` is a Go CLI tool and library that computes semver-compatible version strings from a git
repository's tag history.

- `gitsemver get [--dir <path>] [--ref <ref>]` — resolves and prints the version for a git ref
- `gitsemver next <bump-type> [--last-tag <tag>]` — computes the next semver tag
- `gitsemver validate [--type dev|rc|stable|any] <version>` — validates a version string
- `gitsemver branch-hash [branch]` — prints the CRC32 branch fingerprint used in dev builds

Version resolution rules:

- Commit carrying `vX.Y.Z` → prints `X.Y.Z`
- Commit carrying `vX.Y.Z-rc.N` → prints `X.Y.Z-rc.N`
- Untagged commit → dev build: `X.Y.(Z+1)-r<CRC32-of-branch>t<YYYYMMDDHHMMSS>h<7-char-SHA>`
- When a commit carries multiple version tags, the **highest semver tag wins** and a warning is written to
  `warn`

## Module and package layout

```
github.com/giantswarm/gitsemver/v3   (module)
main.go                               CLI entry point (cobra commands)
main_test.go                          CLI integration tests
pkg/gitsemver/
  repo.go          Repo struct, New(), ResolveVersion(), NextVersion(), buildVersionMaps()
  next.go          ComputeNextVersion(), parseVersionString(), compareSemver()
  validate.go      IsValid*(), semver regex definitions (numID, semverParseRegex)
  funcs.go         TopLevel()
  error.go         InvalidConfigError, ExecutionFailedError
  devversion_test.go, repo_test.go, next_test.go, next_repo_test.go, ...
pkg/project/       Version/GitSHA/BuildTimestamp metadata
```

## Environment variables

| Var                     | Purpose                                                               |
| ----------------------- | --------------------------------------------------------------------- |
| `GS_BRANCH_NAME`        | Override the branch name that dev builds fingerprint                  |
| `GS_GIT_TAG_PREFIX`     | Monorepo: only consider tags with this prefix, e.g. `module-a/v1.2.3` |

## Key architectural notes

**Version maps.** `buildVersionMaps` produces two maps keyed by commit hash:

- `versionsByHash` — highest semver tag (stable or RC) per commit
- `stableVersionsByHash` — highest _stable-only_ tag per commit

**Tag regexes:**

- `tagRegex` matches `vX.Y.Z` and `vX.Y.Z-rc.N` (no leading zeros)
- `stableTagRegex` matches only `vX.Y.Z`

**Dev build tags** follow [RFC: semver-based automatic upgrades](https://github.com/giantswarm/rfc/tree/main/semver-based-automatic-upgrades),
except for the separator letters: the RFC spells them `b`, `t` and `c`, this tool uses `r` (ref), `t` (time)
and `h` (hash), because `b` and `c` are hex digits and hide the field boundaries. The RFC needs an amendment.
The pre-release part is always 33 characters and holds no `.` and no `-`, so a caller that concatenates the
version into a Kubernetes label and trims it cannot cut the pre-release part on an illegal character (a
prefix long enough to push the cut into `X.Y.Z` can still land on the leading `-`). A current tag sorts
_above_ a superseded one at the same `X.Y.Z`, because `r` > `d`. Against `-rc.N` at the same base the order
depends on the first digit of the branch hash: `0` to `b` below the RC, `c` to `f` above it — select dev
builds with a `.*-r<hash>t.*` filter, not a bare range. None of `r`, `t` and `h` is a hex digit, so a reader
can always tell where a field ends. `BranchHash` uses
CRC-32/ISO-HDLC (`hash/crc32.ChecksumIEEE`) — not the POSIX `cksum` variant. Nothing in the tag is ever
truncated. `IsValidDev` also accepts the superseded `-dev.<branch>.<date>.<time>[.h<sha>]` schema, because
tags in that format are already published; `ResolveVersion` only ever generates the current one.

**`compareSemver` follows semver §11:** pre-release has lower precedence than stable at the same X.Y.Z. But an
RC at a _higher patch_ beats a stable at a lower patch — e.g. `v1.0.1-rc.1 > v1.0.0`.

**Warnings.** `Repo.warn io.Writer` defaults to `os.Stderr` so warnings never pollute stdout (script consumers
use stdout). Library callers can redirect via `Config.WarnWriter`. Tests (same package) assign
`repo.warn = &buf` directly.

## Development workflow

### Running tests

Always write tests before or alongside implementation (TDD). Run the full suite before committing:

```bash
go test ./...
```

Run a focused subset during development:

```bash
go test ./pkg/gitsemver/... -run TestName -v
```

### Pre-commit hooks

**Always run pre-commit before pushing.** The CI gate runs it and will fail the build if it's not clean. The
hooks auto-fix some issues (trailing whitespace, gofmt, goimports) and leave the files modified on disk — you
must stage and commit those fixes too.

```bash
pre-commit run --all-files
```

Common auto-fixes to watch for:

- `trailing-whitespace` — fires on `CHANGELOG.md` edits easily
- `go-fmt` / `go-imports` — will rewrite Go files in place

Workflow when pre-commit auto-fixes files:

1. Run `pre-commit run --all-files`
2. If any hook reports "files were modified by this hook", stage the changed files
3. Commit the fixes as a separate chore commit (e.g. `chore: fix trailing whitespace`)

### Committing

Stage files explicitly — never `git add -A` or `git add .`:

```bash
git add pkg/gitsemver/repo.go pkg/gitsemver/repo_test.go
git commit -m "..."
```

The pre-commit hooks run automatically on `git commit`. If a hook fails:

- It did **not** create a commit
- Fix the issue, re-stage, and run `git commit` again (do **not** amend the previous commit)

### Linters

Pre-commit runs `golangci-lint` with these extra linters enabled: `gosec`, `goconst`, `govet`. Fix lint issues
before committing.

## Testing conventions

Tests live in the same package (`package gitsemver`) so they can access unexported fields.

Helper patterns used in `repo_test.go`:

```go
repo, gitRepo := newTestRepo(t)     // creates an in-memory git repo + Repo
var warn bytes.Buffer
repo.warn = &warn                   // capture warnings

h := testCreateCommit(t, gitRepo, "file.txt", time.Date(...))
gitRepo.CreateTag("v1.0.0", h, nil)
```

When testing multi-tag behaviour, create tags in an order that would expose insertion-order bugs (e.g. highest
last, or RC before stable) to prove selection is semver-based.

## PR review patterns learned (PR #254)

- `%v` on `[]string` prints `[a b c]` — use `strings.Join(slice, ", ")` for readable output
- Unexported fields that are meaningful to library callers should be exposed via `Config` struct fields (with
  nil-default fallback), not buried as unexported-only
- Cover the mixed stable + RC case explicitly in tests — the `compareSemver × multi-tag` interaction is the
  subtlest part of the version resolution logic
