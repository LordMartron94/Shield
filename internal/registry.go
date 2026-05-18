package internal

import (
	"slices"
	"strings"
	"sync"
)

// ----------------------------------------------------------- TYPES

type RegisteredOperation struct {
	name        string
	description string
	zonePath    ZonePath
	runner      func(execCtx ExecutionContext) OperationRunResult
}

/*
Run executes the registered operation.
*/
func (o RegisteredOperation) Run(execCtx ExecutionContext) OperationRunResult {
	return o.runner(execCtx)
}

/*
Name returns the registered operation's name.
*/
func (o RegisteredOperation) Name() string {
	return o.name
}

/*
Description returns the registered operation description metadata.
*/
func (o RegisteredOperation) Description() string {
	return o.description
}

/*
ZonePath returns a copy of this registered operation's zone path.
*/
func (o RegisteredOperation) ZonePath() ZonePath {
	return ZonePathCreate(slices.Clone(o.zonePath.parts)...)
}

var registryLock = &sync.Mutex{}

var operationRegistry = []RegisteredOperation{}

// ----------------------------------------------------------- REGISTRATION

func OperationRegister[TState any](operation Operation[TState]) {
	registryLock.Lock()

	operationRegistry = append(operationRegistry, RegisteredOperation{
		name:        operation.name,
		description: operation.description,
		zonePath:    operation.zonePath,
		runner: func(execCtx ExecutionContext) OperationRunResult {
			return OperationRun(&operation, execCtx)
		},
	})

	registryLock.Unlock()
}

// ----------------------------------------------------------- FILTERING

type RegistryFilter func(operation RegisteredOperation) bool

func FilterRegistry(predicate RegistryFilter) []RegisteredOperation {
	var filtered []RegisteredOperation

	registryLock.Lock()
	snapshot := make([]RegisteredOperation, len(operationRegistry))
	copy(snapshot, operationRegistry)
	registryLock.Unlock()

	for _, sc := range snapshot {
		if predicate(sc) {
			filtered = append(filtered, sc)
		}
	}

	return filtered
}

func FilterByZonePrefix(prefix string) RegistryFilter {
	return func(operation RegisteredOperation) bool {
		rendered := operation.zonePath.Render(".")
		return strings.HasPrefix(rendered, prefix)
	}
}
