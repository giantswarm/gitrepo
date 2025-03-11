package gitrepo

import (
	"reflect"
)

type ExecutionFailedError struct {
	message string
}

func (e *ExecutionFailedError) Error() string {
	return "ExecutionFailedError: " + e.message
}

func (e *ExecutionFailedError) Is(target error) bool {
	return reflect.TypeOf(target) == reflect.TypeOf(e)
}

type InvalidConfigError struct {
	message string
}

func (e *InvalidConfigError) Error() string {
	return "InvalidConfigError: " + e.message
}

func (e *InvalidConfigError) Is(target error) bool {
	return reflect.TypeOf(target) == reflect.TypeOf(e)
}

type FileNotFoundError struct {
	message string
}

func (e *FileNotFoundError) Error() string {
	return "fileNotFoundError: " + e.message
}

func (e *FileNotFoundError) Is(target error) bool {
	return reflect.TypeOf(target) == reflect.TypeOf(e)
}

type FolderNotFoundError struct {
	message string
}

func (e *FolderNotFoundError) Error() string {
	return "folderNotFoundError: " + e.message
}

func (e *FolderNotFoundError) Is(target error) bool {
	return reflect.TypeOf(target) == reflect.TypeOf(e)
}

type ReferenceNotFoundError struct {
	message string
}

func (e *ReferenceNotFoundError) Error() string {
	return "ReferenceNotFoundError: " + e.message
}

func (e *ReferenceNotFoundError) Is(target error) bool {
	return reflect.TypeOf(target) == reflect.TypeOf(e)
}

type RepositoryNotFoundError struct {
	message string
}

func (e *RepositoryNotFoundError) Error() string {
	return "repositoryNotFoundError: " + e.message
}

func (e *RepositoryNotFoundError) Is(target error) bool {
	return reflect.TypeOf(target) == reflect.TypeOf(e)
}
