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

Construct exclusively via SHIELD_Rendering_RendererCreate; shared Renderer handles stay safe across concurrent format calls because palettes never mutate post-create and builders stay local per invocation (orthogonal to ScenarioRun SystemIdentity fields).
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

Empty input emits “No scenarios executed.” plus newline. Otherwise emits a Context line summarising SystemIdentity when

every row agrees; mixed Identity batches print “<Mixed Batch>” before scenario counts, aggregate wall versus summed

durations, optional first-run execution timestamp, zone tree sorted stably by ZonePath.Render("."),

and per-scenario pass markers with failing-guard FailureReason lines.

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

(STABLE / IMPROVED versus REGRESSION DETECTED from IsRegression). When Sources carries two runs it prints each side’s

SystemIdentity plus wall StartedAt stamp before deltas. With GuardDeltas empty a muted “no deltas” sentence appears;

otherwise each retained delta emits severity token, guard name, and indented baseline/target PASS versus FAIL rows with fuzz

iteration snippets plus stitched reason strings.

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

(or Storage hydrator): cohort verdict line, enumerated Severity plus Reason excerpt, then—for each side when temporal data

exists—the cohort SystemIdentity, earliest and latest StartedAt timestamps, and span wording before the tabular stats.

The table lists total runs, conditional-highlighted failure-rate deltas, and mean earliest-failing iteration (“Mean Fragility”)

when targets fail sooner on average while the target cohort logged failures.

Pure string emission only—no mutation of StabilityRegressionResult. renderer wiring constraints match the other rendering entry points.
*/
func SHIELD_Rendering_FormatStabilityRegressionReport(
	renderer *SHIELD_Rendering_Renderer,
	report SHIELD_Regression_Stability_Result,
) string {
	return internal.RenderStabilityRegression(renderer, report)
}
