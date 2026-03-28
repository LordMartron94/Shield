# shield

Shield is a structured harness for regression checks, invariants, and unfinished-feature guards. You describe **units** (coarse groupings), **atoms** (ordered checks inside a unit), and **cases** (inputs and expected outputs). The library runs them in deterministic order and reports through **echo** (log prefix `Shield`).

## What it is not

Shield is not a substitute for Go’s `testing` package or a full test runner with assertions libraries. It is an explicit graph of named checks with runner/validator functions, optional setup/teardown, and runtime name-based filtering.

## Core concepts

| Concept | Role |
| -------- | ------ |
| **Configuration** | Top-level list of units you pass to `EngineCreate`. |
| **Unit** | Named group with an `order` field (lower runs first among siblings). Can contain **sub-units** (run first, also ordered) then **atoms**. |
| **Atom** | One check: a `Runner`, a `Validator`, and registered **cases**. Also ordered by `order`. |
| **Case** | Named `input` / `expected` pair for one run of the atom’s runner. |
| **Engine** | Runnable snapshot built from a `Configuration`. |
| **RunReport** | Returned by `Run`: `Failed` if any top-level unit failed (per gate), `Elapsed`, and `TopLevel` outcomes with full nested `UnitRunReport` trees. |
| **RuntimeConfiguration** | Blacklists units and/or atoms by **exact name** (see `docs/adr/0001-Runtime-Configuration-Filtering.md`). |
| **UnitRunReport** | Filled after a unit runs: skips, atom stats, child outcomes. Fed to `UnitEvaluationGate`. |
| **UnitEvaluationGate** | `func(UnitRunReport) bool` — return `true` if the unit counts as **failed** (for parents and stop-on-child). |

Validators return an `AtomResult` **by value**. Helpers return pointers; dereference when returning, for example `return *shield.AtomResultFailureCreate("reason")`. Use `AtomResultSetSkipFurtherAtomsInUnit` to stop running later atoms in the **current** unit after the current atom completes.

## Public API

All symbols live in the `shield` package (`api.go`, `doc.go`). Implementation details stay in `shield/internal`.

Browse documentation:

```bash
cd tools/shield && go doc -all .
```

## Minimal usage sketch

```go
package main

import "shield"

func main() {
    cfg := shield.ConfigurationCreate()

    unit := shield.UnitCreate(0, "example")
    atom := shield.AtomCreate(0, "equals_self", func(in int) int { return in },
        func(out int, c shield.Case[int, int]) shield.AtomResult {
            if out == c.expected {
                return *shield.AtomResultSuccessCreate()
            }
            return *shield.AtomResultFailureCreate("output != expected")
        })
    shield.AtomRegisterCase(atom, shield.CaseCreate("identity", 42, 42))
    shield.UnitRegisterAtom(unit, atom)
    shield.ConfigurationRegisterUnits(cfg, *unit)

    engine := shield.EngineCreate(*cfg)
    rt := shield.RuntimeConfigurationCreate(nil, nil)
    report := shield.Run(engine, rt)
    // use report.Failed for pass/fail; report.TopLevel[i].Report for nested detail
}
```

Adjust imports if your `go.mod` uses a module path other than `shield` (for example a vanity import or workspace replace).

## Behaviour notes

- **Ordering**: Top-level units, sub-units under a parent, and atoms within a unit are sorted by `order` before execution. Cases run in registration order.
- **Panics**: Runner, validator, setup, and teardown panics are recovered and logged; they do not crash the process.
- **Sub-units**: Register with `UnitRegisterSubUnits`. A sub-unit cannot use the same `name` as its parent (registration panics). Nesting is recursive; each unit’s gate sees its own `DirectChildren` with full nested `Report` values.
- **Dependencies between sibling sub-units**: Call `UnitSetStopRemainingSubUnitsOnChildFailure(parent, true)` so a failed sub-unit skips later siblings. If the parent’s own atoms also depend on those sub-units, add `UnitSetSkipOwnAtomsWhenChildFailureStopsSubUnits(parent, true)`. Default failure classification is `UnitEvaluationDefaultFailed` (override with `UnitSetEvaluationGate`). Blacklist skips are not treated as failure.
- **Output**: `Run` returns `RunReport` with `Failed` and per–top-level-unit outcomes (`TopLevel` with nested `Report`). Echo also emits a final **Shield run summary** (structured fields: counts, `run_failed`, timing aggregates; atom-duration mean/min/max/stddev via statarch/blaze when atoms ran). Use `report.Failed` (or walk `TopLevel`) for exit codes and CI.

## Repository layout

- `api.go` — public types and functions (thin wrappers over `internal`).
- `internal/` — execution engine, echo registration, end-of-run summary (statarch/blaze when available through the workspace).
- `docs/adr/` — architecture decisions (filtering, testing stance, etc.).
