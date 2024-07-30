package gitrepo

import (
	"reflect"
)

type executionFailedError struct {
	message string
}

func (e *executionFailedError) Error() string {
	return "executionFailedError: " + e.message
}

func (e *executionFailedError) Is(target error) bool {
	return reflect.TypeOf(target) == reflect.TypeOf(e)
}

type invalidConfigError struct {
	message string
}

func (e *invalidConfigError) Error() string {
	return "invalidConfigError: " + e.message
}

func (e *invalidConfigError) Is(target error) bool {
	return reflect.TypeOf(target) == reflect.TypeOf(e)
}

type fileNotFoundError struct {
	message string
}

func (e *fileNotFoundError) Error() string {
	return "fileNotFoundError: " + e.message
}

func (e *fileNotFoundError) Is(target error) bool {
	return reflect.TypeOf(target) == reflect.TypeOf(e)
}

type folderNotFoundError struct {
	message string
}

func (e *folderNotFoundError) Error() string {
	return "folderNotFoundError: " + e.message
}

func (e *folderNotFoundError) Is(target error) bool {
	return reflect.TypeOf(target) == reflect.TypeOf(e)
}

type referenceNotFoundError struct {
	message string
}

func (e *referenceNotFoundError) Error() string {
	return "referenceNotFoundError: " + e.message
}

func (e *referenceNotFoundError) Is(target error) bool {
	return reflect.TypeOf(target) == reflect.TypeOf(e)
}

type repositoryNotFoundError struct {
	message string
}

func (e *repositoryNotFoundError) Error() string {
	return "repositoryNotFoundError: " + e.message
}

func (e *repositoryNotFoundError) Is(target error) bool {
	return reflect.TypeOf(target) == reflect.TypeOf(e)
}
