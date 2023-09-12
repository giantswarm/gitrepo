[![Go Reference](https://pkg.go.dev/badge/github.com/giantswarm/gitrepo.svg)](https://pkg.go.dev/github.com/giantswarm/gitrepo)
[![CircleCI](https://dl.circleci.com/status-badge/img/gh/giantswarm/gitrepo/tree/main.svg?style=svg)](https://dl.circleci.com/status-badge/redirect/gh/giantswarm/gitrepo/tree/main)

# gitrepo

This is a library used to compute a project version given a git reference.

Here is how to use it:

```go
c := Config{
	AuthBasicToken: "github-token",
	Dir:            "/path/to/some-repo",
	URL:            "git@github.com:giantswarm/some-repo.git",
}
repo, err := New(c)
version, err := repo.ResolveVersion(ctx, "master")
// version is 0.0.0-2e7604b8b3806b20ff305eb4e1a852c784ba34ca
```
