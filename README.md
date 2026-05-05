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
- `[discovery]`: modules to blank-import for registration discovery (canonical `shield/...` paths)
- `[runtime]`: transient bootstrap switch (managed by SHIELD runtime)
- `[zones]`: zone prefix to physical directory mapping for impacted heuristics

## Architecture Notes

- CLI entrypoint is thin (`cmd/main.go`).
- Runtime and shell behavior live in `runner`.
- Public API surface is top-level `*.go`.
- Internal implementation is isolated under `internal`.

Transient shell bootstrap generates a tiny glue runner under `.shield/transient/cmd` that imports configured discovery modules and delegates into SHIELD runner logic.

