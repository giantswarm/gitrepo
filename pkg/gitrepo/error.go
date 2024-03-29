package gitrepo

import "github.com/giantswarm/microerror"

// executionFailedError is an error type for situations where Resource execution
// cannot continue and must always fall back to operatorkit.
//
// This error should never be matched against and therefore there is no matcher
// implement. For further information see:
//
// https://github.com/giantswarm/fmt/blob/master/go/errors.md#matching-errors
var executionFailedError = &microerror.Error{
	Kind: "executionFailedError",
}

var invalidConfigError = &microerror.Error{
	Kind: "invalidConfigError",
}

// IsInvalidConfig asserts invalidConfigError.
func IsInvalidConfig(err error) bool {
	return microerror.Cause(err) == invalidConfigError
}

var fileNotFoundError = &microerror.Error{
	Kind: "fileNotFoundError",
}

// IsFileNotFound asserts fileNotFoundError.
func IsFileNotFound(err error) bool {
	return microerror.Cause(err) == fileNotFoundError
}

var folderNotFoundError = &microerror.Error{
	Kind: "folderNotFoundError",
}

// IsFolderNotFound asserts folderNotFoundError.
func IsFolderNotFound(err error) bool {
	return microerror.Cause(err) == folderNotFoundError
}

var referenceNotFoundError = &microerror.Error{
	Kind: "referenceNotFoundError",
}

// IsReferenceNotFound asserts referenceNotFoundError.
func IsReferenceNotFound(err error) bool {
	return microerror.Cause(err) == referenceNotFoundError
}

var repositoryNotFoundError = &microerror.Error{
	Kind: "repositoryNotFoundError",
}

// IsRepositoryNotFound asserts referenceNotFoundError.
func IsRepositoryNotFound(err error) bool {
	return microerror.Cause(err) == repositoryNotFoundError
}
