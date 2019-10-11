[![GoDoc](https://godoc.org/github.com/giantswarm/gitrepo?status.svg)](http://godoc.org/github.com/giantswarm/gitrepo)
[![CircleCI](https://circleci.com/gh/giantswarm/gitrepo.svg?style=shield)](https://circleci.com/gh/giantswarm/gitrepo)

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
// version is v0.0.0-2e7604b8b3806b20ff305eb4e1a852c784ba34ca
```

