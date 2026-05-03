package internal

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// TODO - encapsulate the rendering mechanisms (coloring, components, etc.) into a separate lib.

// ---------------------------------------------------------------- COLORS

type renderColor uint8

const (
	ColorReset renderColor = iota
	ColorPass
	ColorFail
	ColorMuted
	ColorHeader
	ColorHighlight
	colorCount // Automatically sized based on the iota length
)

// The Palettes are static, contiguous arrays. Zero map lookups.
var paletteNone = [colorCount][]byte{
	ColorReset:     {},
	ColorPass:      {},
	ColorFail:      {},
	ColorMuted:     {},
	ColorHeader:    {},
	ColorHighlight: {},
}

var paletteAnsi16 = [colorCount][]byte{
	ColorReset:     []byte("\033[0m"),
	ColorPass:      []byte("\033[32m"), // Standard Green
	ColorFail:      []byte("\033[31m"), // Standard Red
	ColorMuted:     []byte("\033[90m"), // Bright Black (Gray)
	ColorHeader:    []byte("\033[36m"), // Cyan
	ColorHighlight: []byte("\033[33m"), // Yellow
}

var paletteTrueColor = [colorCount][]byte{
	ColorReset:     []byte("\033[0m"),
	ColorPass:      []byte("\033[38;2;46;204;113m"),  // Vivid Emerald
	ColorFail:      []byte("\033[38;2;231;76;60m"),   // Vivid Crimson
	ColorMuted:     []byte("\033[38;2;127;140;141m"), // Slate Gray
	ColorHeader:    []byte("\033[38;2;52;152;219m"),  // Vivid Blue
	ColorHighlight: []byte("\033[38;2;241;196;15m"),  // Sunflower
}

// ---------------------------------------------------------------- CONFIGURATION

type RenderingColorMode uint8

const (
	Render_Color_None RenderingColorMode = iota + 1
	Render_Color_ANSI16
	Render_Color_True
)

type RenderingConfiguration struct {
	colorMode RenderingColorMode
}

func RenderingConfigurationCreate(
	colorMode RenderingColorMode,
) *RenderingConfiguration {
	return &RenderingConfiguration{
		colorMode: colorMode,
	}
}

// ---------------------------------------------------------------- RENDERER

type Renderer struct {
	config  *RenderingConfiguration
	palette *[colorCount][]byte
}

func RendererCreate(cfg *RenderingConfiguration) *Renderer {
	r := &Renderer{
		config: cfg,
	}

	switch cfg.colorMode {
	case Render_Color_None:
		r.palette = &paletteNone
	case Render_Color_ANSI16:
		r.palette = &paletteAnsi16
	case Render_Color_True:
		r.palette = &paletteTrueColor
	default:
		r.palette = &paletteNone
	}

	return r
}

func (r *Renderer) WriteColor(b *strings.Builder, c renderColor) {
	b.Write(r.palette[c])
}

func RenderScenarios(renderer *Renderer, scenarios []ScenarioRunResult) string {
	if len(scenarios) == 0 {
		return "No scenarios executed.\n"
	}

	builder := &strings.Builder{}

	var total, passed int
	var totalWall, totalSummed time.Duration
	for _, s := range scenarios {
		total++
		if s.Passed() {
			passed++
		}
		totalWall += s.WallDuration()
		totalSummed += s.SummedDuration()
	}

	renderer.WriteColor(builder, ColorHeader)
	builder.WriteString("=== SHIELD DEFENSE REPORT ===\n")
	renderer.WriteColor(builder, ColorReset)

	id, isPure := validateSliceIdentityPurity(scenarios)
	const headerPad = 10

	renderer.WriteColor(builder, ColorMuted)
	if isPure {
		builder.WriteString(fmt.Sprintf("%-*s: %s\n", headerPad, "Context", formatIdentity(id)))
		builder.WriteString(fmt.Sprintf("%-*s: %s\n", headerPad, "Executed", scenarios[0].StartedAt().Format(time.DateTime)))
	} else {
		builder.WriteString(fmt.Sprintf("%-*s: <Mixed Batch>\n", headerPad, "Context"))
	}

	builder.WriteString(fmt.Sprintf("%-*s: %d Total ", headerPad, "Scenarios", total))
	if passed == total {
		renderer.WriteColor(builder, ColorPass)
		builder.WriteString("(All Passed)\n")
	} else {
		renderer.WriteColor(builder, ColorFail)
		builder.WriteString(fmt.Sprintf("(%d Passed, %d Failed)\n", passed, total-passed))
	}
	renderer.WriteColor(builder, ColorReset)

	renderer.WriteColor(builder, ColorMuted)
	builder.WriteString(fmt.Sprintf("%-*s: %v Wall (%v Summed)\n\n", headerPad, "Duration", totalWall, totalSummed))
	renderer.WriteColor(builder, ColorReset)

	renderTreeIllusion(builder, renderer, scenarios)

	return builder.String()
}

func RenderIdenticalRegression(renderer *Renderer, regression RegressionResult) string {
	builder := &strings.Builder{}

	renderer.WriteColor(builder, ColorHeader)
	builder.WriteString("=== SHIELD IDENTICAL REGRESSION REPORT ===\n")
	renderer.WriteColor(builder, ColorReset)

	if regression.IsRegression {
		renderer.WriteColor(builder, ColorFail)
		builder.WriteString("Verdict: REGRESSION DETECTED\n\n")
	} else {
		renderer.WriteColor(builder, ColorPass)
		builder.WriteString("Verdict: STABLE / IMPROVED\n\n")
	}
	renderer.WriteColor(builder, ColorReset)

	if len(regression.Sources) >= 2 {
		renderer.WriteColor(builder, ColorMuted)
		baseStart := regression.Sources[0].StartedAt().Format(time.DateTime)
		tgtStart := regression.Sources[1].StartedAt().Format(time.DateTime)

		builder.WriteString(fmt.Sprintf("Baseline: %s at %s\n", formatIdentity(regression.Sources[0].SnapshotConfig().Identity), baseStart))
		builder.WriteString(fmt.Sprintf("Target  : %s at %s\n\n", formatIdentity(regression.Sources[1].SnapshotConfig().Identity), tgtStart))
		renderer.WriteColor(builder, ColorReset)
	} else {
		renderer.WriteColor(builder, ColorFail)
		builder.WriteString("[RENDER ERROR] Malformed regression result: Missing comparative sources.\n\n")
		renderer.WriteColor(builder, ColorReset)
	}

	if len(regression.GuardDeltas) == 0 {
		renderer.WriteColor(builder, ColorMuted)
		builder.WriteString("No significant guard deltas detected.\n")
		renderer.WriteColor(builder, ColorReset)
		return builder.String()
	}

	for _, delta := range regression.GuardDeltas {
		// 1. Determine Semantic Color
		c := ColorMuted
		switch delta.Severity {
		case RegressionSeverity_Improvement:
			c = ColorPass
		case RegressionSeverity_OutcomeShift, RegressionSeverity_Degradation, RegressionSeverity_FragilityShift:
			c = ColorFail
		}

		// 2. Render Header
		renderer.WriteColor(builder, c)
		fmt.Fprintf(builder, "[%s] ", delta.Severity)
		renderer.WriteColor(builder, ColorReset)
		builder.WriteString(delta.GuardName)
		builder.WriteString("\n")

		// 3. Render Baseline State
		builder.WriteString("  Baseline: ")
		if delta.BaselinePassed {
			renderer.WriteColor(builder, ColorPass)
			builder.WriteString("PASS\n")
		} else {
			renderer.WriteColor(builder, ColorFail)
			builder.WriteString(fmt.Sprintf("FAIL (Iter: %d) - %s\n", delta.BaselineIter, delta.BaselineReason))
		}
		renderer.WriteColor(builder, ColorReset)

		// 4. Render Target State
		builder.WriteString("  Target:   ")
		if delta.TargetPassed {
			renderer.WriteColor(builder, ColorPass)
			builder.WriteString("PASS\n")
		} else {
			renderer.WriteColor(builder, ColorFail)
			builder.WriteString(fmt.Sprintf("FAIL (Iter: %d) - %s\n", delta.TargetIter, delta.TargetReason))
		}
		renderer.WriteColor(builder, ColorReset)
		builder.WriteString("\n")
	}

	return builder.String()
}

func RenderStabilityRegression(renderer *Renderer, regression StabilityRegressionResult) string {
	builder := &strings.Builder{}

	b := regression.Baseline
	t := regression.Target

	renderer.WriteColor(builder, ColorHeader)
	builder.WriteString("=== SHIELD STABILITY REGRESSION REPORT ===\n")
	renderer.WriteColor(builder, ColorReset)

	if regression.IsRegression {
		renderer.WriteColor(builder, ColorFail)
		builder.WriteString("Verdict: REGRESSION DETECTED\n\n")
	} else {
		renderer.WriteColor(builder, ColorPass)
		builder.WriteString("Verdict: STABLE\n\n")
	}
	renderer.WriteColor(builder, ColorReset)

	const headerPad = 10

	renderer.WriteColor(builder, ColorMuted)
	builder.WriteString(fmt.Sprintf("%-*s: %s\n", headerPad, "Severity", regression.Severity))
	builder.WriteString(fmt.Sprintf("%-*s: %s\n\n", headerPad, "Reason", regression.Reason))

	if !b.EarliestRun.IsZero() && !b.LatestRun.IsZero() {
		span := formatTemporalSpan(b.LatestRun.Sub(b.EarliestRun))
		builder.WriteString(fmt.Sprintf("%-*s: %s (Span: %s to %s %s)\n",
			headerPad, "Baseline",
			formatIdentity(b.Identity),
			b.EarliestRun.Format(time.DateTime),
			b.LatestRun.Format(time.DateTime),
			span,
		))
	}

	if !t.EarliestRun.IsZero() && !t.LatestRun.IsZero() {
		span := formatTemporalSpan(t.LatestRun.Sub(t.EarliestRun))
		builder.WriteString(fmt.Sprintf("%-*s: %s (Span: %s to %s %s)\n",
			headerPad, "Target",
			formatIdentity(t.Identity),
			t.EarliestRun.Format(time.DateTime),
			t.LatestRun.Format(time.DateTime),
			span,
		))
	}
	builder.WriteString("\n")
	renderer.WriteColor(builder, ColorReset)

	renderer.WriteColor(builder, ColorMuted)
	builder.WriteString(fmt.Sprintf("%-16s %12s -> %12s\n", "", "Baseline", "Target"))
	renderer.WriteColor(builder, ColorReset)

	builder.WriteString(fmt.Sprintf("%-16s %12d -> %12d\n", "Total Runs:", b.TotalRuns, t.TotalRuns))

	// Failure Rate Formatting
	builder.WriteString(fmt.Sprintf("%-16s ", "Failure Rate:"))
	rateStr := fmt.Sprintf("%11.2f%% -> %11.2f%%\n", b.FailureRate*100, t.FailureRate*100)
	if t.FailureRate > b.FailureRate {
		renderer.WriteColor(builder, ColorFail)
		builder.WriteString(rateStr)
		renderer.WriteColor(builder, ColorReset)
	} else {
		builder.WriteString(rateStr)
	}

	// Fragility Formatting
	builder.WriteString(fmt.Sprintf("%-16s ", "Mean Fragility:"))
	iterStr := fmt.Sprintf("%12d -> %12d\n", b.AverageFailedIter, t.AverageFailedIter)
	if t.AverageFailedIter < b.AverageFailedIter && t.FailedRuns > 0 {
		renderer.WriteColor(builder, ColorFail) // Dropping iterations means it breaks faster (worse)
		builder.WriteString(iterStr)
		renderer.WriteColor(builder, ColorReset)
	} else {
		builder.WriteString(iterStr)
	}

	return builder.String()
}

// ---------------------------------------------------------------- PRIVATE HELPERS

func renderTreeIllusion(b *strings.Builder, r *Renderer, scenarios []ScenarioRunResult) {
	sort.SliceStable(scenarios, func(i, j int) bool {
		return scenarios[i].ZonePath().Render(".") < scenarios[j].ZonePath().Render(".")
	})

	var prevPath []string

	for _, scenario := range scenarios {
		currPath := scenario.ZonePath().parts

		divergenceIndex := 0
		for divergenceIndex < len(prevPath) &&
			divergenceIndex < len(currPath) &&
			prevPath[divergenceIndex] == currPath[divergenceIndex] {
			divergenceIndex++
		}

		for i := divergenceIndex; i < len(currPath); i++ {
			indent := strings.Repeat("  ", i)
			b.WriteString(indent)
			r.WriteColor(b, ColorHeader)
			b.WriteString(currPath[i])
			r.WriteColor(b, ColorReset)
			b.WriteString("\n")
		}

		scenarioIndent := strings.Repeat("  ", len(currPath))
		b.WriteString(scenarioIndent)

		if scenario.Passed() {
			r.WriteColor(b, ColorPass)
			b.WriteString("✓ ")
		} else {
			r.WriteColor(b, ColorFail)
			b.WriteString("✗ ")
		}

		r.WriteColor(b, ColorReset)
		b.WriteString(scenario.Name())

		r.WriteColor(b, ColorMuted)
		b.WriteString(fmt.Sprintf(" (%v)\n", scenario.WallDuration()))
		r.WriteColor(b, ColorReset)

		if !scenario.Passed() {
			for _, g := range scenario.GuardResults() {
				if !g.Passed() {
					b.WriteString(scenarioIndent)
					b.WriteString("  - ")
					r.WriteColor(b, ColorFail)
					b.WriteString(g.Name())
					r.WriteColor(b, ColorReset)
					b.WriteString(": ")
					b.WriteString(g.FailureReason())
					b.WriteString("\n")
				}
			}
		}

		prevPath = currPath
	}
}

func formatTemporalSpan(d time.Duration) string {
	d = d.Round(time.Minute)

	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("(%dd %dh %dm)", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("(%dh %dm)", hours, minutes)
	}
	return fmt.Sprintf("(%dm)", minutes)
}

func formatIdentity(id SystemIdentity) string {
	return fmt.Sprintf("%s (%s)", id.Version, id.Environment)
}

func validateSliceIdentityPurity(scenarios []ScenarioRunResult) (SystemIdentity, bool) {
	if len(scenarios) == 0 {
		return SystemIdentity{}, true
	}

	baselineID := scenarios[0].SnapshotConfig().Identity
	for _, s := range scenarios[1:] {
		if s.SnapshotConfig().Identity != baselineID {
			return SystemIdentity{}, false
		}
	}
	return baselineID, true
}
