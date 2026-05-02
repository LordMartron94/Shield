package shield

import "shield/internal"

/*
SHIELD_Rendering_ColorMode selects how escape sequences color SHIELD textual reports.

None emits no ANSI bytes (pipes, persisted logs).

ANSI16 uses standard ECMA-48 sixteen-colour foreground SGR sequences.

True uses 24-bit foreground RGB tuned for typical dark-terminal backgrounds.

Unrecognized numeric modes and the zero value map to None when SHIELD_Rendering_RendererCreate initializes palettes.
*/
type SHIELD_Rendering_ColorMode = internal.RenderingColorMode

const (
	SHIELD_Rendering_Color_None   SHIELD_Rendering_ColorMode = internal.Render_Color_None
	SHIELD_Rendering_Color_ANSI16 SHIELD_Rendering_ColorMode = internal.Render_Color_ANSI16
	SHIELD_Rendering_Color_True   SHIELD_Rendering_ColorMode = internal.Render_Color_True
)

/*
SHIELD_Rendering_Configuration binds SHIELD_Rendering_RendererCreate inputs.

Currently this only snapshots ColorMode while leaving room for future toggles without breaking call sites again.
*/
type SHIELD_Rendering_Configuration = internal.RenderingConfiguration

/*
SHIELD_Rendering_ConfigurationCreate wraps RenderingConfigurationCreate for public callers.

colorMode chooses the backing palette SHIELD_Rendering_RendererCreate wires; unset or unrecognized values degrade to SHIELD_Rendering_Color_None semantics.
*/
func SHIELD_Rendering_ConfigurationCreate(
	colorMode SHIELD_Rendering_ColorMode,
) *SHIELD_Rendering_Configuration {
	return internal.RenderingConfigurationCreate(colorMode)
}

/*
SHIELD_Rendering_Renderer fronts fixed palette tables for SHIELD_Rendering_FormatScenarioRunResults,

SHIELD_Rendering_FormatIdenticalRegressionReport, and SHIELD_Rendering_FormatStabilityRegressionReport.

Construct exclusively via SHIELD_Rendering_RendererCreate; renderer identity is concurrency-safe across format calls because palettes are immutable and each invocation owns its scratch builder memory.
*/
type SHIELD_Rendering_Renderer = internal.Renderer

/*
SHIELD_Rendering_RendererCreate maps cfg into a Renderer with matching palette bindings.

Reuse the returned pointer across batches; cfg must not be nil.
*/
func SHIELD_Rendering_RendererCreate(cfg *SHIELD_Rendering_Configuration) *SHIELD_Rendering_Renderer {
	return internal.RendererCreate(cfg)
}

/*
SHIELD_Rendering_FormatScenarioRunResults returns a textual “SHIELD DEFENCE REPORT” for the supplied ScenarioRunResults.

Empty input emits “No scenarios executed.” plus newline. Otherwise emits aggregate scenario counts,

wall versus summed durations, then each scenario sorted stably by ZonePath.Render("."). Zone segments render as indented

breadcrumbs; each scenario prints pass/fail, name, wall time, then failing guards with FailureReason text.

renderer must originate from SHIELD_Rendering_RendererCreate; nil panics inside renderer wiring.

Sorts scenarios in-place (stable) by each row’s ZonePath.Render("."); clone the slice before calling when callers must preserve the incoming order.
*/
func SHIELD_Rendering_FormatScenarioRunResults(
	renderer *SHIELD_Rendering_Renderer,
	scenarios []SHIELD_Testing_ScenarioRunResult,
) string {
	return internal.RenderScenarios(renderer, scenarios)
}

/*
SHIELD_Rendering_FormatIdenticalRegressionReport turns a pairwise SHIELD_Regression_Result from

SHIELD_Regression_CheckIdentical (or Storage twin) into a terminal-oriented summary.

Outputs a labelled header (“SHIELD IDENTICAL REGRESSION REPORT”), pass/fail verdict coloring

(STABLE / IMPROVED versus REGRESSION DETECTED from IsRegression), then either a muted “no deltas” sentence

when GuardDeltas is empty or—for each retained delta—severity token, guard name,

and indented baseline/target PASS versus FAIL rows with fuzz iteration snippets and stitched reason strings.

Improvement deltas use pass hues; regressing severities use fail hues; severity None deltas use muted styling if ever present.

Does not reorder report fields or mutate RegressionResult; purely allocates through strings.Builder wiring.

renderer must come from SHIELD_Rendering_RendererCreate (nil dereferences internally).
*/
func SHIELD_Rendering_FormatIdenticalRegressionReport(
	renderer *SHIELD_Rendering_Renderer,
	report SHIELD_Regression_Result,
) string {
	return internal.RenderIdenticalRegression(renderer, report)
}

/*
SHIELD_Rendering_FormatStabilityRegressionReport formats StabilityRegressionResult from SHIELD_Regression_CheckStability

(or Storage hydrator): cohort verdict line, enumerated Severity plus Reason excerpt, then a fixed-width baseline→target projection listing total runs, failure-rate percentages with conditional fail highlights

when the target rate strictly exceeds baseline, and mean earliest-failing iteration (“Mean Fragility”) with conditional fail highlights

when targets fail sooner on average while the target cohort recorded failures.

Pure string emission only—no mutation of StabilityRegressionResult. renderer wiring constraints match the other rendering entry points.
*/
func SHIELD_Rendering_FormatStabilityRegressionReport(
	renderer *SHIELD_Rendering_Renderer,
	report SHIELD_Regression_Stability_Result,
) string {
	return internal.RenderStabilityRegression(renderer, report)
}
