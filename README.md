# S.H.I.E.L.D

SHIELD stands for `Systematic Hierarchical Integration and Evaluation Layer for Defense`.

SHIELD is a validation and regression harness for scenario-based testing with:

- operation registry and zone metadata
- scenario execution with deterministic snapshots
- persisted results in SQLite
- impacted rerun decisions based on persisted history
- shell diagnostics and report rendering

## Dependencies

SHIELD depends on these libraries:

- `Foundation`
- `Essence`
- `Persistence`
- `Statarch`
- `Blaze`
- `Memarch`
- `Memcore`
- `Memforge`
- `Memstruct`

Use the SHIELD setup script to clone exactly those dependencies:

```bash
./scripts/install_dependencies.sh
```

The installer is intentionally scoped to SHIELD requirements only.

## Quick Start

From the SHIELD repository root:

```bash
go run ./cmd -configuration-path ./shield_config.example.toml
```

Inside the shell:

- `list` / `ls`: list registered operations and last-run recency
- `run`: interactive operation selection
- `run impacted`: operation-scoped impacted execution with diagnostics
- `results`: paged persisted scenario rows
- `help`: command overview

## Configuration

Example config:

- `shield_config.example.toml`

Important sections:

- `[environment]`: database path + rendering mode
- `[discovery]`: directories containing SHIELD registrations; relative entries resolve from the configuration file location and are converted to Go import paths automatically
- `[runtime]`: transient bootstrap switch (managed by SHIELD runtime)
- `[zones]`: zone prefix to physical directory mapping for impacted heuristics

Descriptions:

- operation and scenario descriptions are optional in-memory metadata for shell/report rendering
- descriptions are never persisted to SQLite storage

## Architecture Notes

- CLI entrypoint is thin (`cmd/main.go`).
- Runtime and shell behavior live in `runner`.
- Public API surface is top-level `*.go`.
- Internal implementation is isolated under `internal`.

Transient shell bootstrap generates a tiny glue runner under `.shield/transient/cmd` that imports configured discovery modules and delegates into SHIELD runner logic.

## Regression Planes

SHIELD separates two fundamentally different failure planes:

- **Algorithmic fragility (deterministic):** logic fails the same way for the same seed/input.
- **Environmental fragility (probabilistic):** failures emerge from scheduling, IO contention, network jitter, or host pressure.

### Local Plane: Deterministic PR Defense

Use `run impacted` locally to validate that a code change did not break deterministic behavior.

- **Purpose:** prove the PR does not break the deterministic baseline.
- **Mechanism:** runs only for impacted operations based on persisted commit lineage.
- **Evaluation:** strict pairwise regression checks (`CheckIdentical` semantics). Any failure reason drift, pass/fail outcome shift, or earlier failure iteration is a regression.

This is where fuzzing-driven deterministic fragility is caught quickly.

### Fleet Plane: Continuous Stability Monitoring

Use CI/automation to run operations continuously on the same hash, then evaluate stability statistically.

- **Purpose:** prove x-branch behavior remains stable under environmental chaos.
- **Mechanism:** periodic forced runs (bypassing impacted gating), e.g. hourly.
- **Evaluation:** after accumulating a cohort for the exact same identity hash (for example N=50), run `CheckStability` to detect failure-rate drift and fragility shifts.

This is where probabilistic infrastructure degradation is detected.

### Operational Rule

Do not conflate local impacted workflows and fleet stability workflows into one command path.

- Local loop validates logic correctness.
- Fleet loop validates environment resilience.

## CI Orchestration Recommendation

Use a scheduled CI workflow that:

1. checks out the target branch/commit,
2. executes forced SHIELD runs for selected zones on a fixed cadence,
3. persists scenario rows,
4. triggers stability evaluation once cohort size threshold is reached.

If you want one default, start with a scheduled GitHub Actions workflow on `main` every hour and gate stability alerts on statistically significant drift.
