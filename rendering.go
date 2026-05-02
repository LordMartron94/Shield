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
SHIELD_Rendering_Renderer fronts fixed palette tables queried by SHIELD_Rendering_FormatScenarioRunResults.

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
