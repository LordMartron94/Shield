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

/*
WriteColor is a hot-path inline helper. No maps. Zero allocations.
Direct memory access `O(1)` pointer arithmetic.
*/
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

	renderer.WriteColor(builder, ColorMuted)
	builder.WriteString(fmt.Sprintf("Scenarios: %d Total | ", total))
	renderer.WriteColor(builder, ColorReset)

	if passed == total {
		renderer.WriteColor(builder, ColorPass)
		builder.WriteString(fmt.Sprintf("%d Passed\n", passed))
	} else {
		renderer.WriteColor(builder, ColorFail)
		builder.WriteString(fmt.Sprintf("%d Passed, %d Failed\n", passed, total-passed))
	}

	renderer.WriteColor(builder, ColorMuted)
	builder.WriteString(fmt.Sprintf("Duration:  %v Wall | %v Summed\n\n", totalWall, totalSummed))
	renderer.WriteColor(builder, ColorReset)

	renderTreeIllusion(builder, renderer, scenarios)

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
