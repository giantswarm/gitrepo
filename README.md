[![Go Reference](https://pkg.go.dev/badge/github.com/giantswarm/gitsemver/v3.svg)](https://pkg.go.dev/github.com/giantswarm/gitsemver/v3)
[![CircleCI](https://dl.circleci.com/status-badge/img/gh/giantswarm/gitsemver/tree/main.svg?style=svg)](https://dl.circleci.com/status-badge/redirect/gh/giantswarm/gitsemver/tree/main)

# gitsemver

Library and CLI tool for computing a semVer-compatible version from a git reference.

## Version format

| Situation | Returned version |
|---|---|
| HEAD carries stable tag `vX.Y.Z` | `X.Y.Z` |
| HEAD carries pre-release tag `vX.Y.Z-rc.N` | `X.Y.Z-rc.N` |
| HEAD is untagged, stable ancestor `vX.Y.Z` reachable | `X.Y.(Z+1)-r<branch-hash>t<YYYYMMDDHHMMSS>h<commit-sha>` |
| HEAD is untagged, no stable ancestor reachable | `0.0.0-r<branch-hash>t<YYYYMMDDHHMMSS>h<commit-sha>` |

For untagged commits the base is the most recent **stable** ancestor tag reachable from the ref (RC and other pre-release tags are skipped). When no stable ancestor exists the version prefix is `0.0.0` with no patch increment.

### Dev build pre-release part

Dev build versions are often used inside Kubernetes attributes, so the pre-release part is built from three fixed-width, alphanumeric fields:

| Field | Meaning |
|---|---|
| `r<branch-hash>` | CRC32 checksum of the full, unsanitized branch name, lowercase hex, padded to 8 digits. Print it with `gitsemver branch-hash`. |
| `t<YYYYMMDDHHMMSS>` | Committer date of the resolved commit, in UTC, with no separators. |
| `h<commit-sha>` | 7-char git short hash of the resolved commit, for tag-to-commit traceability. |

The result is always 33 characters and holds no `.` and no `-`. A caller that concatenates the version into a label and then trims to 63 characters therefore cannot cut the pre-release part on a character Kubernetes rejects. Only a prefix long enough to push the cut into the `X.Y.Z` part can still land on the leading `-`. The literal `r`, `t` and `h` prefixes keep every field alphanumeric: an all-digit pre-release identifier is compared numerically and forbids leading zeros, which would break time stamps. None of the three is a hex digit, so a reader can always tell where a field ends.

For one branch the `r<branch-hash>t` prefix is constant, so semVer compares the fixed-width time stamps and the per-branch chronological sort order is correct. Two commits in the same second still get different tags, but the commit hash then decides their order, which is arbitrary.

```sh
$ GS_BRANCH_NAME=renovate/update-all-dependencies-to-latest gitsemver get
1.2.4-r08a93c50t20260127094959h1a2b3c4
```

The schema changed once. `validate --type dev` still accepts the superseded `X.Y.Z-dev.<branch>.<YYYY-MM-DD>.<HH-MM-SS>[.h<commit-sha>]` format, because tags in that format are already published. `get` only ever generates the current one. See [RFC: semver-based automatic upgrades](https://github.com/giantswarm/rfc/tree/main/semver-based-automatic-upgrades).

The RFC spells the three separators `b`, `t` and `c`. This tool uses `r` (ref), `t` (time) and `h` (hash) instead, because `b` and `c` are hex digits and a reader cannot see where a field ends. Every other property is unchanged: the field order, the widths, the CRC variant and the 33-character total. **The RFC needs an amendment to match.**

**Sort order.** A current tag sorts *above* a superseded one at the same `X.Y.Z`, because `r` > `d` in the first pre-release identifier. A consumer therefore moves to the current schema at once:

```
1.2.4-dev.my-feature.2026-01-27.09-49-59.h1a2b3c4  <  1.2.4-r7b5b4fa7t20260127094959h1a2b3c4
```

Against an `-rc.N` tag at the same base the order depends on the first digit of the branch hash: `0` to `b` sort below the RC, `c` to `f` above it. Select dev builds by branch, which keeps them out of an RC range anyway:

```yaml
semverFilter: ".*-r7b5b4fa7t.*"
```

## Environment variables

| Variable | Effect |
|---|---|
| `GS_BRANCH_NAME` | Override the branch name that dev build versions are fingerprinted from. Defaults to the HEAD branch of the repo, then `"unknown"`. |
| `GS_GIT_TAG_PREFIX` | Monorepo support: only consider tags prefixed with `"<value>/"`, e.g. `module-a/v1.2.3`. |

## CLI — `gitsemver`

```
go install github.com/giantswarm/gitsemver/v3@latest
```

```
Usage:
  gitsemver get [--dir <path>] [--ref <ref>]
  gitsemver next <patch|minor|major|patch-rc|minor-rc|major-rc|rc|rc-release> [--last-tag <tag>]
  gitsemver validate [--type dev|rc|stable|any] <version>
  gitsemver branch-hash [branch]
  gitsemver completion <bash|zsh|fish|powershell>
```

### get

Print the version for a git ref:

```
  --dir string   path inside the git repository (default ".", resolved to repo root)
  --ref string   git ref to resolve: branch name, tag, or commit SHA (default "HEAD")
```

```sh
$ GS_BRANCH_NAME=my-feature gitsemver get
1.2.4-r7b5b4fa7t20260127094959h1a2b3c4

$ gitsemver get --ref v1.2.3
1.2.3
```

### next

Compute the next semver release tag after the highest-semver tag reachable from HEAD:

```sh
$ gitsemver next patch          # v1.2.3 ancestor → prints 1.2.4
$ gitsemver next minor-rc       # v1.2.3 ancestor → prints 1.3.0-rc.1
$ gitsemver next rc             # v1.3.0-rc.1 ancestor → prints 1.3.0-rc.2
$ gitsemver next rc-release     # v1.3.0-rc.1 ancestor → prints 1.3.0
```

Use `--last-tag` to supply the base explicitly — no git repository needed:

```sh
$ gitsemver next patch --last-tag v1.2.3
1.2.4
```

Valid bump types:

| Base tag | Bump type | Result |
|---|---|---|
| Stable `X.Y.Z` | `patch` | `X.Y.Z+1` |
| Stable `X.Y.Z` | `minor` | `X.Y+1.0` |
| Stable `X.Y.Z` | `major` | `X+1.0.0` |
| Stable `X.Y.Z` | `patch-rc` | `X.Y.Z+1-rc.1` |
| Stable `X.Y.Z` | `minor-rc` | `X.Y+1.0-rc.1` |
| Stable `X.Y.Z` | `major-rc` | `X+1.0.0-rc.1` |
| RC `X.Y.Z-rc.N` | `rc` | `X.Y.Z-rc.N+1` |
| RC `X.Y.Z-rc.N` | `rc-release` | `X.Y.Z` |

When no version tag is reachable from HEAD, `0.0.0` is used as the base.
Note: `rc` and `rc-release` require an actual reachable RC tag and cannot be used from the implicit `0.0.0` base — start a new RC series with `patch-rc`, `minor-rc`, or `major-rc` instead.

### branch-hash

Print the CRC32 fingerprint a dev build embeds for a branch. Use it to build a Flux `semverFilter` that matches one branch:

```sh
$ gitsemver branch-hash my-feature
7b5b4fa7

$ git branch --show-current
new-feature
$ gitsemver branch-hash          # no argument: the current branch
2ae1b065
```

The algorithm is CRC-32/ISO-HDLC, the variant Go's `hash/crc32.ChecksumIEEE` and Python's `zlib.crc32` implement. The POSIX `cksum` tool uses a different variant and returns a different value.

### Shell completion

Generate and load a completion script for your shell:

```sh
# bash — add to ~/.bashrc
source <(gitsemver completion bash)

# zsh — add to ~/.zshrc (requires compinit to be loaded)
source <(gitsemver completion zsh)

# fish
gitsemver completion fish | source

# PowerShell — add to $PROFILE
gitsemver completion powershell | Out-String | Invoke-Expression
```

Once loaded, `gitsemver next <TAB>` completes bump types and `gitsemver validate --type <TAB>` completes version types.

## Go library

```go
import "github.com/giantswarm/gitsemver/v3/pkg/gitsemver"

c := gitsemver.Config{
    Dir: "/path/to/some-repo",
    URL: "git@github.com:giantswarm/some-repo.git",
}
repo, err := gitsemver.New(c)
version, err := repo.ResolveVersion(ctx, "HEAD")
// e.g. "1.2.4-r7b5b4fa7t20260127094959h1a2b3c4"
```
