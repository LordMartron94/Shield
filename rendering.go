package shield

import "shield/internal"

/*
SHIELD_Rendering_ColorMode selects how ANSI escape sequences color SHIELD textual reports. None strips color bytes (pipes, logs).

ANSI16 uses sixteen-color ECMA-48 foreground SGR codes. True emits 24-bit RGB tuned for typical dark terminals.

Unrecognized numeric modes and the zero value map to None once SHIELD_Rendering_RendererCreate wires palettes.
*/
type SHIELD_Rendering_ColorMode = internal.RenderingColorMode

const (
	SHIELD_Rendering_Color_None   SHIELD_Rendering_ColorMode = internal.Render_Color_None
	SHIELD_Rendering_Color_ANSI16 SHIELD_Rendering_ColorMode = internal.Render_Color_ANSI16
	SHIELD_Rendering_Color_True   SHIELD_Rendering_ColorMode = internal.Render_Color_True
)

/*
SHIELD_Rendering_Configuration binds SHIELD_Rendering_RendererCreate inputs; today it only snapshots ColorMode for forward-compatible extension.
*/
type SHIELD_Rendering_Configuration = internal.RenderingConfiguration

/*
SHIELD_Rendering_ConfigurationCreate builds config; colorMode chooses palettes, with unknown values degrading to SHIELD_Rendering_Color_None semantics.
*/
func SHIELD_Rendering_ConfigurationCreate(
	colorMode SHIELD_Rendering_ColorMode,
) *SHIELD_Rendering_Configuration {
	return internal.RenderingConfigurationCreate(colorMode)
}

/*
SHIELD_Rendering_Renderer fronts immutable palette tables consumed by the Format* entry points.

Construct via SHIELD_Rendering_RendererCreate only; shared Renderer pointers stay safe under concurrent formatting because

post-create palettes never mutate and strings.Builder scratch stays per call (orthogonal to ScenarioRun SystemIdentity stamping).
*/
type SHIELD_Rendering_Renderer = internal.Renderer

/*
SHIELD_Rendering_RendererCreate maps cfg into a Renderer with matching palette routing; cfg must be non-nil.
*/
func SHIELD_Rendering_RendererCreate(cfg *SHIELD_Rendering_Configuration) *SHIELD_Rendering_Renderer {
	return internal.RendererCreate(cfg)
}

/*
SHIELD_Rendering_FormatScenarioRunResults emits the text “SHIELD DEFENCE REPORT”. Empty input yields “No scenarios executed.”

Otherwise it prints Context (homogeneous SystemIdentity or “<Mixed Batch>”), aggregate scenario timing summary, optional first-run

stamp, then a zone breadcrumb tree sorted in-place by ZonePath.Render("."). renderer must be non-nil from RendererCreate.

The input slice is reordered stable-sort by zone path; clone first if callers rely on original ordering.
*/
func SHIELD_Rendering_FormatScenarioRunResults(
	renderer *SHIELD_Rendering_Renderer,
	scenarios []SHIELD_Testing_ScenarioRunResult,
) string {
	return internal.RenderScenarios(renderer, scenarios)
}

/*
SHIELD_Rendering_FormatIdenticalRegressionReport renders SHIELD_Regression_CheckIdentical outcomes (or storage twins) for terminals.

# It prints the pairwise header, verdict coloring, optional dual-run SystemIdentity timelines, then either a muted “no deltas” line

or bullet guard transitions with baseline/target PASS traces. renderer must be non-nil.

The function does not mutate RegressionResult and only allocates transient builder memory.
*/
func SHIELD_Rendering_FormatIdenticalRegressionReport(
	renderer *SHIELD_Rendering_Renderer,
	report SHIELD_Regression_Result,
) string {
	return internal.RenderIdenticalRegression(renderer, report)
}

/*
SHIELD_Rendering_FormatStabilityRegressionReport lays out SHIELD_Regression_CheckStability verdicts (or storage-hydrated equivalents).

After severity and reason text it may print each cohort’s SystemIdentity alongside earliest/latest StartedAt span language, then the

tabular totals, failure-rate deltas, and mean earliest-failure iterations. Pure emission only; renderer must follow the same lifecycle

rules as the other formatters.
*/
func SHIELD_Rendering_FormatStabilityRegressionReport(
	renderer *SHIELD_Rendering_Renderer,
	report SHIELD_Regression_Stability_Result,
) string {
	return internal.RenderStabilityRegression(renderer, report)
}
