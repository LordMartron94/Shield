package shield

import "shield/internal"

/*
This file binds SHIELD_Registry_* facades onto the internal global Operation ledger.

Consumers register SHIELD_Testing_Operation bundles at package init when tooling needs to discover workloads from zone breadcrumbs.

Enumerate snapshots the ledger under a brief mutex lock, evaluates predicates afterward without holding that lock—a predicate that calls Registration again settles on whatever the next Enumeration observes.
*/

/*
SHIELD_Registry_RegisteredOperation is one append-only ledger row produced whenever SHIELD_Registry_OperationRegister captures an Operation[TState].

Read fields only through RegisteredOperationName, RegisteredOperationZonePath, and RegisteredOperationRun so future internal layout tweaks stay encapsulated.
*/
type SHIELD_Registry_RegisteredOperation = internal.RegisteredOperation

/*
SHIELD_Registry_FilterPredicate selects rows during SHIELD_Registry_OperationEnumerate: returning true retains the scanned entry following snapshot order.

Build custom matchers with SHIELD_Registry_RegisteredOperationName plus rendered SHIELD_Testing_ZonePath values or compose SHIELD_Registry_FilterPredicateZonePathRenderedPrefix.
*/
type SHIELD_Registry_FilterPredicate = internal.RegistryFilter

/*
SHIELD_Registry_OperationRegister appends Operation metadata into the process-global mutex-protected ledger.

Duplicates are deliberate non-errors—colliding names rely on organisational discipline upstream. Heavy panics still spring from guarded Operation internals (startup/teardown/scenario closures) exactly as documented for SHIELD_Testing_OperationRun, not from the bookkeeping append alone.
*/
func SHIELD_Registry_OperationRegister[TState any](operation SHIELD_Testing_Operation[TState]) {
	internal.OperationRegister(operation)
}

/*
SHIELD_Registry_OperationEnumerate returns RegisteredOperation snapshots whose predicates pass while preserving insertion order captured at invocation start.

Expensive predicates only block concurrent Register callers for the shallow copy—not for the entirety of filtering work.
*/
func SHIELD_Registry_OperationEnumerate(predicate SHIELD_Registry_FilterPredicate) []SHIELD_Registry_RegisteredOperation {
	return internal.FilterRegistry(predicate)
}

/*
SHIELD_Registry_FilterPredicateZonePathRenderedPrefix manufactures a matcher that succeeds when ZonePath.Render(".") reports a dotted string prefixed exactly by renderedZonePathPrefix.

Supplying empty prefix matches rendered paths unconditionally (normally only useful during sanity scaffolding). Narrow prefixes collide with unintended siblings whenever zone labels share textual stems—prefer fully qualified dotted segments terminating distinct boundaries whenever ambiguity exists.
*/
func SHIELD_Registry_FilterPredicateZonePathRenderedPrefix(renderedZonePathPrefix string) SHIELD_Registry_FilterPredicate {
	return internal.FilterByZonePrefix(renderedZonePathPrefix)
}
