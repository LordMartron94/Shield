package internal

import (
	"slices"
	"strings"
	"sync"
)

// ----------------------------------------------------------- TYPES

type RegisteredOperation struct {
	name     string
	zonePath ZonePath
	runner   func() OperationRunResult
}

func RegisteredOperationRun(operation RegisteredOperation, config ScenarioRunConfig) OperationRunResult {
	return operation.runner()
}

func RegisteredOperationName(operation RegisteredOperation) string {
	return operation.name
}

func RegisteredOperationZonePath(operation RegisteredOperation) ZonePath {
	return ZonePathCreate(slices.Clone(operation.zonePath.parts)...)
}

var registryLock = &sync.Mutex{}

var operationRegistry = []RegisteredOperation{}

// ----------------------------------------------------------- REGISTRATION

func OperationRegister[TState any](operation Operation[TState]) {
	registryLock.Lock()

	operationRegistry = append(operationRegistry, RegisteredOperation{
		name:     operation.name,
		zonePath: operation.zonePath,
		runner: func() OperationRunResult {
			return OperationRun(&operation)
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
