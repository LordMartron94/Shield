package runner

import (
	"encoding/json"
	"fmt"
	"foundation/formatting"
	"os"
	"path/filepath"
	"shield/internal"
	"strings"
	"time"
)

type MemoryDiagnosticsMode string

const (
	MemoryDiagnosticsOff       MemoryDiagnosticsMode = "off"
	MemoryDiagnosticsAlways    MemoryDiagnosticsMode = "always"
	MemoryDiagnosticsLeaksOnly MemoryDiagnosticsMode = "leaks-only"
)

type MemoryDiagnosticResult struct {
	Available                bool
	Provider                 string
	AllocatorTypeCount       int
	AllocatorCount           int
	ActiveAllocatorCount     int
	DestroyedAllocatorCount  int
	AllocatorsWithLiveAllocs int
	TotalAllocationCount     int
	LiveAllocationCount      int
	TotalBytes               uint64
	LiveBytes                uint64
	LeakDetected             bool
	Allocators               []MemoryDiagnosticAllocatorResult
}

type MemoryDiagnosticAllocatorResult struct {
	Name                  string
	Address               uintptr
	Destroyed             bool
	CreatedAt             time.Time
	Creator               string
	TotalAllocations      int
	LiveAllocations       int
	TotalBytes            uint64
	LiveBytes             uint64
	Status                string
	LiveAllocationDetails []MemoryDiagnosticAllocationResult
}

type MemoryDiagnosticAllocationResult struct {
	Address   uintptr
	SizeBytes uint64
	CreatedAt time.Time
	Creator   string
}

func parseMemoryDiagnosticsMode(raw string) (MemoryDiagnosticsMode, error) {
	trimmed := strings.TrimSpace(strings.ToLower(raw))
	if trimmed == "" {
		return MemoryDiagnosticsOff, nil
	}

	switch MemoryDiagnosticsMode(trimmed) {
	case MemoryDiagnosticsOff, MemoryDiagnosticsAlways, MemoryDiagnosticsLeaksOnly:
		return MemoryDiagnosticsMode(trimmed), nil
	default:
		return MemoryDiagnosticsOff, fmt.Errorf("unknown runtime.memory_diagnostics value '%s' (expected off, always, or leaks-only)", raw)
	}
}

func memoryDiagnosticsModeFromConfig(cfg *ShieldConfiguration) MemoryDiagnosticsMode {
	if cfg == nil {
		return MemoryDiagnosticsOff
	}
	mode, err := parseMemoryDiagnosticsMode(cfg.Runtime.MemoryDiagnostics)
	if err != nil {
		panic(fmt.Errorf("engine error: %w", err))
	}
	return mode
}

func memoryDiagnosticsEnabled(cfg *ShieldConfiguration) bool {
	return memoryDiagnosticsModeFromConfig(cfg) != MemoryDiagnosticsOff
}

func shouldRenderMemoryDiagnostics(mode MemoryDiagnosticsMode, result MemoryDiagnosticResult) bool {
	switch mode {
	case MemoryDiagnosticsOff:
		return false
	case MemoryDiagnosticsLeaksOnly:
		if !result.Available {
			return false
		}
		return result.LeakDetected
	default:
		return true
	}
}

func renderMemoryDiagnostics(renderer *internal.Renderer, builder *strings.Builder, result MemoryDiagnosticResult) {
	renderer.WriteColor(builder, internal.ColorHeader)
	builder.WriteString("=== SHIELD MEMORY DIAGNOSTICS ===\n")
	renderer.WriteColor(builder, internal.ColorReset)

	if !result.Available {
		renderer.WriteColor(builder, internal.ColorMuted)
		builder.WriteString("Provider  : memforge (unavailable — rebuild with -tags memforge_debug)\n")
		renderer.WriteColor(builder, internal.ColorReset)
		builder.WriteString("\n")
		return
	}

	renderer.WriteColor(builder, internal.ColorMuted)
	builder.WriteString(fmt.Sprintf("Provider  : %s\n", result.Provider))
	builder.WriteString(fmt.Sprintf("Allocators: %d Total (%d Active, %d Destroyed, %d Types)\n",
		result.AllocatorCount, result.ActiveAllocatorCount, result.DestroyedAllocatorCount, result.AllocatorTypeCount))
	builder.WriteString(fmt.Sprintf("Activity  : %d Total Allocations, %s Total Bytes\n",
		result.TotalAllocationCount, formatting.FormatMemoryBytes(result.TotalBytes)))
	builder.WriteString(fmt.Sprintf("Live      : %d Allocations, %s, %d Allocators With Live Data\n",
		result.LiveAllocationCount, formatting.FormatMemoryBytes(result.LiveBytes), result.AllocatorsWithLiveAllocs))
	renderer.WriteColor(builder, internal.ColorReset)

	builder.WriteString("Verdict   : ")
	if result.LeakDetected {
		renderer.WriteColor(builder, internal.ColorFail)
		builder.WriteString("MEMORY LEAK DETECTED\n")
	} else {
		renderer.WriteColor(builder, internal.ColorPass)
		builder.WriteString("No leaks detected\n")
	}
	renderer.WriteColor(builder, internal.ColorReset)

	if result.LeakDetected {
		renderLeakyAllocators(renderer, builder, result.Allocators)
	}
	builder.WriteString("\n")
}

func renderLeakyAllocators(renderer *internal.Renderer, builder *strings.Builder, allocators []MemoryDiagnosticAllocatorResult) {
	for _, allocator := range allocators {
		if allocator.LiveAllocations == 0 {
			continue
		}

		renderer.WriteColor(builder, internal.ColorFail)
		builder.WriteString(fmt.Sprintf("  %s", allocator.Name))
		renderer.WriteColor(builder, internal.ColorReset)
		builder.WriteString(fmt.Sprintf(" [%s | addr=%s]\n", allocator.Status, formatMemoryDiagnosticAddress(allocator.Address)))

		renderer.WriteColor(builder, internal.ColorMuted)
		builder.WriteString(fmt.Sprintf("    Allocations: %d Total, %d Live | Bytes: %s Total, %s Live\n",
			allocator.TotalAllocations,
			allocator.LiveAllocations,
			formatting.FormatMemoryBytes(allocator.TotalBytes),
			formatting.FormatMemoryBytes(allocator.LiveBytes),
		))
		if strings.TrimSpace(allocator.Creator) != "" {
			builder.WriteString(fmt.Sprintf("    Created By : %s\n", allocator.Creator))
		}
		renderer.WriteColor(builder, internal.ColorReset)

		for _, allocation := range allocator.LiveAllocationDetails {
			builder.WriteString(fmt.Sprintf("    - addr=%s size=%s age=%s\n",
				formatMemoryDiagnosticAddress(allocation.Address),
				formatting.FormatMemoryBytes(allocation.SizeBytes),
				formatMemoryDiagnosticAge(allocation.CreatedAt),
			))
			if strings.TrimSpace(allocation.Creator) != "" {
				renderer.WriteColor(builder, internal.ColorMuted)
				builder.WriteString(fmt.Sprintf("      %s\n", allocation.Creator))
				renderer.WriteColor(builder, internal.ColorReset)
			}
		}
	}
}

func formatMemoryDiagnosticAddress(address uintptr) string {
	if address == 0 {
		return "<invalid>"
	}
	return fmt.Sprintf("0x%016x", address)
}

func formatMemoryDiagnosticAge(createdAt time.Time) string {
	if createdAt.IsZero() {
		return "unknown"
	}
	return time.Since(createdAt).Round(time.Millisecond).String()
}

func runPostExecutionMemoryDiagnostics(renderer *internal.Renderer, builder *strings.Builder, cfg *ShieldConfiguration) {
	mode := memoryDiagnosticsModeFromConfig(cfg)
	if mode == MemoryDiagnosticsOff {
		return
	}

	result := memoryDiagnosticsCollect()
	if !shouldRenderMemoryDiagnostics(mode, result) {
		return
	}

	renderMemoryDiagnostics(renderer, builder, result)
}

type MemoryTimelineMode string

const (
	MemoryTimelineOff    MemoryTimelineMode = "off"
	MemoryTimelineRender MemoryTimelineMode = "render"
	MemoryTimelineExport MemoryTimelineMode = "export"
	MemoryTimelineBoth   MemoryTimelineMode = "both"
)

type MemoryTimelineFreedAllocation struct {
	AllocationAddress string `json:"alloc_addr"`
	SizeBytes         uint64 `json:"size_bytes"`
	OriginalSeq       uint64 `json:"orig_seq"`
	OriginalCreatedAt string `json:"orig_ts"`
}

type MemoryTimelineEvent struct {
	Seq               uint64                          `json:"seq"`
	Timestamp         string                          `json:"ts"`
	Kind              string                          `json:"kind"`
	AllocatorAddress  string                          `json:"allocator_addr"`
	AllocatorName     string                          `json:"allocator_name"`
	Stack             []string                        `json:"stack,omitempty"`
	AllocationAddress string                          `json:"alloc_addr,omitempty"`
	SizeBytes         uint64                          `json:"size_bytes,omitempty"`
	OriginalSeq       uint64                          `json:"orig_seq,omitempty"`
	OriginalCreatedAt string                          `json:"orig_ts,omitempty"`
	Freed             []MemoryTimelineFreedAllocation `json:"freed,omitempty"`
}

type MemoryTimelineResult struct {
	Available  bool
	Provider   string
	CapturedAt time.Time
	EventCount int
	Events     []MemoryTimelineEvent
}

func parseMemoryTimelineMode(raw string) (MemoryTimelineMode, error) {
	trimmed := strings.TrimSpace(strings.ToLower(raw))
	if trimmed == "" {
		return MemoryTimelineOff, nil
	}

	switch MemoryTimelineMode(trimmed) {
	case MemoryTimelineOff, MemoryTimelineRender, MemoryTimelineExport, MemoryTimelineBoth:
		return MemoryTimelineMode(trimmed), nil
	default:
		return MemoryTimelineOff, fmt.Errorf("unknown runtime.memory_timeline value '%s' (expected off, render, export, or both)", raw)
	}
}

func validateMemoryTimelineConfig(modeRaw, pathRaw string) error {
	mode, err := parseMemoryTimelineMode(modeRaw)
	if err != nil {
		return err
	}
	switch mode {
	case MemoryTimelineExport, MemoryTimelineBoth:
		if strings.TrimSpace(pathRaw) == "" {
			return fmt.Errorf("runtime.memory_timeline_path is required when memory_timeline is '%s'", mode)
		}
	}
	return nil
}

func memoryTimelineModeFromConfig(cfg *ShieldConfiguration) MemoryTimelineMode {
	if cfg == nil {
		return MemoryTimelineOff
	}
	mode, err := parseMemoryTimelineMode(cfg.Runtime.MemoryTimeline)
	if err != nil {
		panic(fmt.Errorf("engine error: %w", err))
	}
	return mode
}

func resolveMemoryTimelinePath(configPath, configuredPath string) (string, error) {
	trimmed := strings.TrimSpace(configuredPath)
	if trimmed == "" {
		return "", fmt.Errorf("memory timeline path is empty")
	}
	if filepath.IsAbs(trimmed) {
		return trimmed, nil
	}
	cfgAbs, err := filepath.Abs(configPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve config path: %w", err)
	}
	return filepath.Join(filepath.Dir(cfgAbs), trimmed), nil
}

func runPostExecutionMemoryTimeline(renderer *internal.Renderer, builder *strings.Builder, cfg *ShieldConfiguration, configPath string) {
	mode := memoryTimelineModeFromConfig(cfg)
	if mode == MemoryTimelineOff {
		return
	}

	result := memoryTimelineCollect()
	if !result.Available && (mode == MemoryTimelineRender || mode == MemoryTimelineBoth) {
		renderMemoryTimelineUnavailable(renderer, builder)
		return
	}
	if !result.Available && mode == MemoryTimelineExport {
		renderer.WriteColor(builder, internal.ColorFail)
		builder.WriteString("Memory timeline export skipped: memforge debugger unavailable (rebuild with -tags memforge_debug)\n")
		renderer.WriteColor(builder, internal.ColorReset)
		return
	}

	switch mode {
	case MemoryTimelineRender:
		renderMemoryTimeline(renderer, builder, result)
	case MemoryTimelineExport:
		if err := exportMemoryTimelineJSONL(configPath, cfg.Runtime.MemoryTimelinePath, result); err != nil {
			renderer.WriteColor(builder, internal.ColorFail)
			builder.WriteString(fmt.Sprintf("Memory timeline export failed: %v\n", err))
			renderer.WriteColor(builder, internal.ColorReset)
			return
		}
		logMemoryTimelineExportPath(renderer, builder, configPath, cfg.Runtime.MemoryTimelinePath)
	case MemoryTimelineBoth:
		renderMemoryTimeline(renderer, builder, result)
		if err := exportMemoryTimelineJSONL(configPath, cfg.Runtime.MemoryTimelinePath, result); err != nil {
			renderer.WriteColor(builder, internal.ColorFail)
			builder.WriteString(fmt.Sprintf("Memory timeline export failed: %v\n", err))
			renderer.WriteColor(builder, internal.ColorReset)
			return
		}
		logMemoryTimelineExportPath(renderer, builder, configPath, cfg.Runtime.MemoryTimelinePath)
	}
}

func renderMemoryTimelineUnavailable(renderer *internal.Renderer, builder *strings.Builder) {
	renderer.WriteColor(builder, internal.ColorHeader)
	builder.WriteString("=== SHIELD MEMORY TIMELINE ===\n")
	renderer.WriteColor(builder, internal.ColorReset)
	renderer.WriteColor(builder, internal.ColorMuted)
	builder.WriteString("Provider  : memforge (unavailable — rebuild with -tags memforge_debug)\n")
	renderer.WriteColor(builder, internal.ColorReset)
	builder.WriteString("\n")
}

func renderMemoryTimeline(renderer *internal.Renderer, builder *strings.Builder, result MemoryTimelineResult) {
	renderer.WriteColor(builder, internal.ColorHeader)
	builder.WriteString("=== SHIELD MEMORY TIMELINE ===\n")
	renderer.WriteColor(builder, internal.ColorReset)

	if result.EventCount == 0 {
		renderer.WriteColor(builder, internal.ColorMuted)
		builder.WriteString("No timeline events recorded.\n")
		renderer.WriteColor(builder, internal.ColorReset)
		builder.WriteString("\n")
		return
	}

	kindCounts := make(map[string]int)
	var firstTS, lastTS time.Time
	for i, evt := range result.Events {
		kindCounts[evt.Kind]++
		if ts, err := time.Parse(time.RFC3339Nano, evt.Timestamp); err == nil {
			if i == 0 || ts.Before(firstTS) {
				firstTS = ts
			}
			if i == 0 || ts.After(lastTS) {
				lastTS = ts
			}
		}
	}

	renderer.WriteColor(builder, internal.ColorMuted)
	builder.WriteString(fmt.Sprintf("Provider  : %s\n", result.Provider))
	builder.WriteString(fmt.Sprintf("Events    : %d\n", result.EventCount))
	if !firstTS.IsZero() && !lastTS.IsZero() {
		builder.WriteString(fmt.Sprintf("Span      : %s to %s (%s)\n",
			firstTS.Format("15:04:05.000"),
			lastTS.Format("15:04:05.000"),
			lastTS.Sub(firstTS).Round(time.Millisecond)))
	}
	for _, kind := range sortedTimelineKinds(kindCounts) {
		builder.WriteString(fmt.Sprintf("  %-28s %d\n", kind+":", kindCounts[kind]))
	}
	renderer.WriteColor(builder, internal.ColorReset)
	builder.WriteString("\n")

	for _, evt := range result.Events {
		renderMemoryTimelineEvent(renderer, builder, evt)
	}
	builder.WriteString("\n")
}

func sortedTimelineKinds(counts map[string]int) []string {
	kinds := make([]string, 0, len(counts))
	for kind := range counts {
		kinds = append(kinds, kind)
	}
	for i := 0; i < len(kinds); i++ {
		for j := i + 1; j < len(kinds); j++ {
			if kinds[j] < kinds[i] {
				kinds[i], kinds[j] = kinds[j], kinds[i]
			}
		}
	}
	return kinds
}

func renderMemoryTimelineEvent(renderer *internal.Renderer, builder *strings.Builder, evt MemoryTimelineEvent) {
	renderer.WriteColor(builder, internal.ColorHighlight)
	builder.WriteString(fmt.Sprintf("#%-6d ", evt.Seq))
	renderer.WriteColor(builder, internal.ColorReset)
	builder.WriteString(fmt.Sprintf("%s %-28s ", evt.Timestamp, evt.Kind))
	builder.WriteString(fmt.Sprintf("%s (%s)\n", evt.AllocatorName, evt.AllocatorAddress))

	switch evt.Kind {
	case "allocation", "free_manual":
		builder.WriteString(fmt.Sprintf("  addr=%s size=%s", evt.AllocationAddress, formatting.FormatMemoryBytes(evt.SizeBytes)))
		if evt.Kind == "free_manual" {
			builder.WriteString(fmt.Sprintf(" orig=#%d@%s", evt.OriginalSeq, evt.OriginalCreatedAt))
		}
		builder.WriteString("\n")
	case "free_regional_reset", "free_regional_destroy":
		builder.WriteString(fmt.Sprintf("  freed=%d allocation(s)\n", len(evt.Freed)))
		for _, freed := range evt.Freed {
			builder.WriteString(fmt.Sprintf("    - %s %s orig=#%d@%s\n",
				freed.AllocationAddress,
				formatting.FormatMemoryBytes(freed.SizeBytes),
				freed.OriginalSeq,
				freed.OriginalCreatedAt))
		}
	}

	if len(evt.Stack) > 0 {
		renderer.WriteColor(builder, internal.ColorMuted)
		builder.WriteString("  ")
		builder.WriteString(strings.Join(evt.Stack, " → "))
		builder.WriteString("\n")
		renderer.WriteColor(builder, internal.ColorReset)
	}
}

func logMemoryTimelineExportPath(renderer *internal.Renderer, builder *strings.Builder, configPath, configuredPath string) {
	resolved, err := resolveMemoryTimelinePath(configPath, configuredPath)
	if err != nil {
		return
	}
	renderer.WriteColor(builder, internal.ColorMuted)
	builder.WriteString(fmt.Sprintf("Memory timeline exported to: %s\n", resolved))
	renderer.WriteColor(builder, internal.ColorReset)
}

func exportMemoryTimelineJSONL(configPath, configuredPath string, result MemoryTimelineResult) error {
	resolved, err := resolveMemoryTimelinePath(configPath, configuredPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(resolved), 0o755); err != nil {
		return fmt.Errorf("failed to create timeline export directory: %w", err)
	}

	file, err := os.Create(resolved)
	if err != nil {
		return fmt.Errorf("failed to create timeline export file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	for _, evt := range result.Events {
		if err := encoder.Encode(evt); err != nil {
			return fmt.Errorf("failed to write timeline event: %w", err)
		}
	}

	summary := map[string]any{
		"type":        "summary",
		"provider":    result.Provider,
		"captured_at": result.CapturedAt.Format(time.RFC3339Nano),
		"event_count": result.EventCount,
	}
	if err := encoder.Encode(summary); err != nil {
		return fmt.Errorf("failed to write timeline summary: %w", err)
	}
	return nil
}
