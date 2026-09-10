// gitsemver prints or validates semVer-compatible version strings for git refs.
//
// Usage:
//
//	gitsemver get [--dir <path>] [--ref <ref>]
//	gitsemver next <bump-type> [--last-tag <tag>]
//	gitsemver validate [--type dev|rc|stable|any] <version>
//	gitsemver branch-hash [branch]
//
// The "get" subcommand resolves and prints the version for a git ref:
//
//	For a ref that carries a stable tag (vX.Y.Z) it prints X.Y.Z.
//	For a pre-release tag (vX.Y.Z-rc.N) it prints X.Y.Z-rc.N.
//	For an untagged ref it prints a dev build:
//
//	    X.Y.(Z+1)-b<branch-hash>t<YYYYMMDDHHMMSS>c<commit-sha>
//
//	where X.Y.Z is the most recent stable ancestor tag reachable from the ref,
//	or 0.0.0 when none exists, <branch-hash> is the 8-hex CRC32 of the branch
//	name, the time stamp is the committer date of the ref in UTC, and
//	<commit-sha> is the 7-char git short hash of the ref.
//
// The "next" subcommand computes the next semver tag from the highest-semver tag
// reachable from HEAD. Valid bump types from a stable tag: patch, minor, major,
// patch-rc, minor-rc, major-rc. Valid from an RC tag: rc, rc-release.
// Use --last-tag to skip git and compute directly from a supplied base tag.
//
// The "validate" subcommand checks whether a version string matches the
// expected format.  It exits 0 and prints "valid" on success, exits 1 and
// prints "invalid" otherwise.  For dev builds it accepts both the current
// schema and the superseded "-dev.<branch>.<date>.<time>" one.
//
// The "branch-hash" subcommand prints the 8-hex CRC32 fingerprint that a dev
// build embeds for a branch, so it can be used in a Flux semver filter. Without
// an argument it uses the current branch.
//
// Environment variables:
//
//	GS_BRANCH_NAME      Override the branch name that dev builds fingerprint.
//	                    Defaults to the HEAD branch of the repo, then "unknown".
//	GS_GIT_TAG_PREFIX   Monorepo support: only consider tags prefixed with
//	                    "<value>/", e.g. "module-a/v1.2.3". The "next --last-tag"
//	                    flag accepts both prefixed ("module-a/v1.2.3") and bare
//	                    ("v1.2.3") forms; the prefix is stripped automatically.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/giantswarm/gitsemver/v3/pkg/gitsemver"
	"github.com/giantswarm/gitsemver/v3/pkg/project"
)

// errInvalidVersion is returned by runValidate when the version string does
// not match the requested format.  It is not a usage error — the caller
// should print "invalid" and exit 1.
var errInvalidVersion = errors.New("invalid")

// usageError marks errors that should exit 2 (bad invocation) rather than exit 1 (runtime failure).
type usageError struct{ cause error }

func (e *usageError) Error() string { return e.cause.Error() }
func (e *usageError) Unwrap() error { return e.cause }

// usageArgs wraps a cobra PositionalArgs validator so failures exit 2.
func usageArgs(f cobra.PositionalArgs) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if err := f(cmd, args); err != nil {
			return &usageError{err}
		}
		return nil
	}
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		if errors.Is(err, errInvalidVersion) {
			fmt.Println("invalid")
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		var uErr *usageError
		// cobra returns "unknown command" as a plain error (not typed), so check the prefix too.
		if errors.As(err, &uErr) || strings.HasPrefix(err.Error(), "unknown command ") {
			os.Exit(2)
		}
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "gitsemver",
		Short:         "Print or validate semVer-compatible version strings for git refs.",
		Version:       fmt.Sprintf("%s (git: %s, built: %s)", project.Version(), project.GitSHA(), project.BuildTimestamp()),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return &usageError{fmt.Errorf("subcommand required; run 'gitsemver --help' for usage")}
		},
	}
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return &usageError{err}
	})
	root.AddCommand(newGetCmd(), newNextCmd(), newValidateCmd(), newBranchHashCmd())
	return root
}

func newGetCmd() *cobra.Command {
	var dir, ref string
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Resolve and print the semver version for a git ref.",
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGet(dir, ref)
		},
	}
	cmd.Flags().StringVar(&dir, "dir", ".", "path inside the git repository (resolved to the repo root)")
	cmd.Flags().StringVar(&ref, "ref", "HEAD", "git ref to resolve: branch name, tag, or commit SHA")
	return cmd
}

func newNextCmd() *cobra.Command {
	var lastTag string
	cmd := &cobra.Command{
		Use:       "next <bump-type>",
		Short:     "Compute the next semver tag after the last tag reachable from HEAD.",
		Args:      usageArgs(cobra.ExactArgs(1)),
		ValidArgs: gitsemver.ValidBumpTypes,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runNext(args[0], lastTag)
		},
	}
	cmd.Flags().StringVar(&lastTag, "last-tag", "", "base tag to compute from; when set, no git repo is needed")
	return cmd
}

func newValidateCmd() *cobra.Command {
	var typFlag string
	cmd := &cobra.Command{
		Use:   "validate <version>",
		Short: "Check whether a version string matches a known format. Exits 0 on success, 1 otherwise.",
		Args:  usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runValidate(typFlag, args[0])
		},
	}
	cmd.Flags().StringVar(&typFlag, "type", "any", "version type to validate: dev, rc, stable, or any")
	_ = cmd.RegisterFlagCompletionFunc("type", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{"dev", "rc", "stable", "any"}, cobra.ShellCompDirectiveNoFileComp
	})
	return cmd
}

func newBranchHashCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "branch-hash [branch]",
		Short: "Print the CRC32 branch fingerprint that dev build versions embed.",
		Args:  usageArgs(cobra.MaximumNArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBranchHash(args)
		},
	}
}

func runGet(dir, ref string) error {
	ctx := context.Background()

	topLevel, err := gitsemver.TopLevel(dir)
	if err != nil {
		return fmt.Errorf("finding git root from %q: %w", dir, err)
	}

	repo, err := openRepo(topLevel)
	if err != nil {
		return err
	}

	version, err := repo.ResolveVersion(ctx, ref)
	if err != nil {
		return fmt.Errorf("resolving version for %q: %w", ref, err)
	}

	fmt.Println(version)
	return nil
}

func openRepo(topLevel string) (*gitsemver.Repo, error) {
	repo, err := gitsemver.New(gitsemver.Config{Dir: topLevel})
	if err != nil {
		// New() fails when there is no origin remote and no URL was given.
		// Fall back to a placeholder URL so we can still read local tags.
		var invalidCfg *gitsemver.InvalidConfigError
		if errors.As(err, &invalidCfg) {
			repo, err = gitsemver.New(gitsemver.Config{Dir: topLevel, URL: "_"})
		}
		if err != nil {
			return nil, fmt.Errorf("opening repository at %q: %w", topLevel, err)
		}
	}
	return repo, nil
}

func runNext(bumpType, lastTag string) error {
	if !slices.Contains(gitsemver.ValidBumpTypes, bumpType) {
		return fmt.Errorf("unknown bump type %q: must be one of %s", bumpType, strings.Join(gitsemver.ValidBumpTypes, ", "))
	}

	lastTag = strings.TrimSpace(lastTag)
	if lastTag != "" {
		tag := lastTag
		if prefix := strings.TrimSpace(os.Getenv("GS_GIT_TAG_PREFIX")); prefix != "" {
			tag = strings.TrimPrefix(tag, prefix+"/")
		}
		next, err := gitsemver.ComputeNextVersion(tag, bumpType)
		if err != nil {
			return err
		}
		fmt.Println(next)
		return nil
	}

	ctx := context.Background()

	topLevel, err := gitsemver.TopLevel(".")
	if err != nil {
		return fmt.Errorf("finding git root: %w", err)
	}

	repo, err := openRepo(topLevel)
	if err != nil {
		return err
	}

	version, err := repo.NextVersion(ctx, bumpType)
	if err != nil {
		return fmt.Errorf("computing next version: %w", err)
	}

	fmt.Println(version)
	return nil
}

// runBranchHash prints the fingerprint of the branch named in args, or of the
// current branch when args is empty.
func runBranchHash(args []string) error {
	branch := gitsemver.CurrentBranch()
	if len(args) == 1 {
		branch = args[0]
	}
	fmt.Println(gitsemver.BranchHash(branch))
	return nil
}

func runValidate(typFlag, version string) error {
	var ok bool
	switch typFlag {
	case "stable":
		ok = gitsemver.IsValidStable(version)
	case "rc":
		ok = gitsemver.IsValidRC(version)
	case "dev":
		ok = gitsemver.IsValidDev(version)
	case "any":
		ok = gitsemver.IsValid(version)
	default:
		return fmt.Errorf("unknown --type %q: must be dev, rc, stable, or any", typFlag)
	}

	if ok {
		fmt.Println("valid")
		return nil
	}
	return errInvalidVersion
}
