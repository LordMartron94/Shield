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
| **Case** | Named input for one run of the atom’s runner, with optional expected output. |
| **Engine** | Runnable snapshot built from a `Configuration`. |
| **RunReport** | Returned by `Run`: `Failed`, `Elapsed`, `TopLevel` with nested unit outcomes. |
| **RunOutcome** | Returned by `RunWithReportPersistence`: wraps `Report` (`RunReport`) and optional `WrittenReportPath` when persistence wrote a file. |
| **ReportPersistence** | Output directory, enable flag, `ReportPersistenceMode` (JSON / TXT / JSONAndTXT), or custom `ReportPersistAdapter`. |
| **ReportPersistenceMode** | Built-in writers: `ReportPersistenceModeJSON`, `ReportPersistenceModeTXT`, `ReportPersistenceModeJSONAndTXT`. |
| **RunMetrics** | Snapshot passed to `ReportPersistAdapter` (aggregates + atom timing stats). |
| **RuntimeConfiguration** | Blacklists units and/or atoms by **exact name** (see `docs/adr/0001-Runtime-Configuration-Filtering.md`). |
| **UnitRunReport** | Filled after a unit runs: skips, atom stats, child outcomes. Fed to `UnitEvaluationGate`. |
| **UnitEvaluationGate** | `func(UnitRunReport) bool` — return `true` if the unit counts as **failed** (for parents and stop-on-child). |

Validators return an `AtomResult` **by value**. Helpers return pointers; dereference when returning, for example `return *shield.AtomResultFailureCreate("reason")`. Use `AtomResultSetSkipFurtherAtomsInUnit` to stop running later atoms in the **current** unit after the current atom completes.

## Shield CLI (interactive)

Shield includes an Anvil-style interactive CLI under `./cmd/shield`:

```bash
cd tools/shield
go run ./cmd/shield -config=shieldconfig.toml
```

The CLI is explicit by design: it does not auto-discover domain tests. Register a harness builder from your domain package:

```go
shield.CLIHarnessBuilderRegister(func() (*shield.Configuration, error) {
    cfg := shield.ConfigurationCreate()
    // register units/atoms/cases here
    return cfg, nil
})
```

Then in the CLI:

- `run [target] [flags]` executes the registered harness (target from TOML)
- `list` / `ls` lists persisted run artifacts
- `show <latest|index|file>` prints report metadata
- `delete <latest|index|file>` deletes a report (with confirmation)
- `clean` deletes all Shield artifacts in the active results directory
- `baseline show|set <latest|index|file>|clear` manages a pinned baseline pointer
- `help`, `quit`, `exit`

`run` supports:

- `--unit=a,b,c` unit blacklist
- `--atom=x,y,z` atom blacklist
- `--mode=json|txt|both` persistence mode
- `--out=<dir>` output directory override
- `--persist=true|false` toggle persistence

Result directory defaults to `results/tests`, overrideable via `SHIELD_RESULTS_DIR` or `run --out`.

### TOML configuration

Shield CLI supports a TOML config (similar to Anvil's benchconfig flow) through:

```bash
go run ./cmd/shield -config=shieldconfig.toml
```

Template file: `tools/shield/shieldconfig.toml`

Supported keys:

- `results_dir` default output directory for report management commands and `run`
- `[run].unit_blacklist` default unit blacklist for `run`
- `[run].atom_blacklist` default atom blacklist for `run`
- `[run].persist` default persistence toggle for `run`
- `[run].mode` default persistence mode (`json|txt|both`) for `run`
- `[run].out_dir` default output directory for `run` when set
- `[targets.<name>]` named run target overrides for `run <name>`
  - optional `entrypoint_kind = "go_test"` to delegate execution to `go test`
  - `entrypoint` package/path for delegated execution
  - `test_run_pattern` optional `-run` regex for Go tests
  - `entrypoint_args` optional extra arguments appended to `go test`

Precedence for `run`:

1. Explicit CLI flags (`--unit`, `--atom`, `--persist`, `--mode`, `--out`)
2. Selected target overrides (`run <target>`)
3. TOML defaults
4. Built-in defaults

### In-Go entrypoint targets

For Anvil-like target resolution of Go entrypoints, define a target as:

```toml
[targets.tests]
entrypoint_kind = "go_test"
entrypoint = "./tests"
test_run_pattern = "^TestShieldFrameworkSample$"
entrypoint_args = ["-count=1", "-v"]
```

Then invoke:

```text
run tests
```

When `entrypoint_kind = "go_test"` is set, Shield CLI delegates run execution to `go test`
for that target entrypoint instead of using the in-process `CLIHarnessBuilderRegister` path.

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
            if out == shield.CaseExpectedGet(c) {
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
- **Output**: `Run` returns `RunReport` with `Failed` and per–top-level-unit outcomes (`TopLevel` with nested `Report`). Echo also emits a final **Shield run summary** (structured fields: counts, `run_failed`, timing aggregates; atom-duration stats via statarch when atoms ran: mean, population stddev, min/max from the five-number path, sum, median, Q1/Q3, IQR, p95/p99, and population coefficient of variation only when `atoms_timed >= 2` and `|mean|` is above a tiny ns epsilon so statarch never divides by zero). Use `report.Failed` (or walk `TopLevel`) for exit codes and CI. With very few timed atoms, quartiles and tail percentiles are still defined (interpolation) but are noisier; interpret accordingly.

## Persisting run reports

Use `ReportPersistenceCreate(dir)` or `ReportPersistenceCreateFromEnv(dir)`, `ReportPersistenceSetEnabled(true)`, `ReportPersistenceSetMode` with `ReportPersistenceModeJSON`, `ReportPersistenceModeTXT`, or `ReportPersistenceModeJSONAndTXT` (default after create is JSON), then `RunWithReportPersistence(engine, rt, persist)`. For a custom writer, `ReportPersistenceSetAdapter(persist, func(dir string, report shield.RunReport, metrics shield.RunMetrics) (string, error) { ... })`; when set, **mode is ignored**. The directory is created with mode `0750`; built-in files use mode `0640`.

- **Filenames**: `shield-run-YYYYMMDD-hhmmss.nnnnnnnnn.json` (or `.txt`), UTC wall time from the metrics snapshot. If that name exists, `_1`, `_2`, … are inserted before the extension (up to 1000 attempts).
- **JSON**: `schema_version` is `ReportDocumentSchemaVersion` (currently 2). The document includes `written_at`, `run_failed`, `elapsed_ns`, a **summary** block (aggregate counts + optional `duration` object: mean/stddev/min/max, sum, median, Q1/Q3, IQR, p95/p99, human-readable strings for mean/min/max/median/p95/p99, and `coeff_var_pop` when computed), and a **tree** mirroring top-level units and nested `UnitRunReport` data. Per-atom / per-case rows are not included (aggregates only).
- **Primary path**: On success, `RunOutcome.WrittenReportPath` is the JSON path when JSON was written, otherwise the text path. Write errors are logged via echo and do not fail the run.

## Repository layout

- `api.go` — public types and functions (thin wrappers over `internal`).
- `internal/` — execution engine, echo registration, metrics/summary, optional report persistence (`report_persist.go`), statarch/blaze when available through the workspace.
- `docs/adr/` — architecture decisions (filtering, testing stance, etc.).
