package internal

/*
ScenarioGuardIsolation configures which guards in a scenario execute in a subprocess.

Zero value: no guards run isolated (default). Use ScenarioGuardIsolationAllGuardsSet or
ScenarioGuardIsolationPerGuardSet on a scenario, or ScenarioCreateWithGuardIsolation at creation time.
*/
type ScenarioGuardIsolation struct {
	allGuards bool
	perGuard  map[string]bool
}

/*
ScenarioGuardIsolationAllGuards marks every guard in the scenario for subprocess isolation.
*/
func ScenarioGuardIsolationAllGuards() ScenarioGuardIsolation {
	return ScenarioGuardIsolation{allGuards: true}
}

/*
ScenarioGuardIsolationPerGuard marks only the named guards for subprocess isolation.

Guard names must match SHIELD_Testing_GuardCreate names. Unlisted guards stay in-process.
*/
func ScenarioGuardIsolationPerGuard(perGuard map[string]bool) ScenarioGuardIsolation {
	if perGuard == nil {
		return ScenarioGuardIsolation{}
	}
	copied := make(map[string]bool, len(perGuard))
	for guardName, isolated := range perGuard {
		copied[guardName] = isolated
	}
	return ScenarioGuardIsolation{perGuard: copied}
}

/*
ScenarioGuardIsolationEnabled reports whether guardName should run in a subprocess for this scenario.
*/
func ScenarioGuardIsolationEnabled(isolation ScenarioGuardIsolation, guardName string) bool {
	if isolation.allGuards {
		return true
	}
	if isolation.perGuard == nil {
		return false
	}
	return isolation.perGuard[guardName]
}

/*
ScenarioGuardIsolationAllGuardsSet configures subprocess isolation for all guards in scenario.
*/
func ScenarioGuardIsolationAllGuardsSet[TInput, TOutput any](
	scenario Scenario[TInput, TOutput],
	enabled bool,
) Scenario[TInput, TOutput] {
	if enabled {
		scenario.guardIsolation = ScenarioGuardIsolationAllGuards()
	} else {
		scenario.guardIsolation = ScenarioGuardIsolation{}
	}
	return scenario
}

/*
ScenarioGuardIsolationPerGuardSet configures subprocess isolation for specific guards in scenario.
*/
func ScenarioGuardIsolationPerGuardSet[TInput, TOutput any](
	scenario Scenario[TInput, TOutput],
	perGuard map[string]bool,
) Scenario[TInput, TOutput] {
	scenario.guardIsolation = ScenarioGuardIsolationPerGuard(perGuard)
	return scenario
}
