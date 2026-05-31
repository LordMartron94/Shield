package runner

import (
	"fmt"
	"foundation/formatting"
	"memforge"
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

func shouldRenderMemoryDiagnostics(mode MemoryDiagnosticsMode, analysis memforge.MemforgeMemoryTimelineAnalysis) bool {
	switch mode {
	case MemoryDiagnosticsOff:
		return false
	case MemoryDiagnosticsLeaksOnly:
		if !analysis.Available {
			return false
		}
		return analysis.LeakDetected
	default:
		return true
	}
}

func renderMemoryDiagnostics(renderer *internal.Renderer, builder *strings.Builder, analysis memforge.MemforgeMemoryTimelineAnalysis) {
	renderer.WriteColor(builder, internal.ColorHeader)
	builder.WriteString("=== SHIELD MEMORY DIAGNOSTICS ===\n")
	renderer.WriteColor(builder, internal.ColorReset)

	if !analysis.Available {
		renderer.WriteColor(builder, internal.ColorMuted)
		builder.WriteString("Provider  : memforge (unavailable — rebuild with -tags memforge_debug)\n")
		renderer.WriteColor(builder, internal.ColorReset)
		builder.WriteString("\n")
		return
	}

	renderer.WriteColor(builder, internal.ColorMuted)
	builder.WriteString("Provider  : memforge\n")
	builder.WriteString(fmt.Sprintf("Events    : %d\n", analysis.TotalEvents))
	builder.WriteString(fmt.Sprintf("Allocators: %d Total (%d Active, %d Destroyed)\n",
		analysis.TotalAllocators, analysis.ActiveAllocators, analysis.DestroyedAllocators))
	builder.WriteString(fmt.Sprintf("Live      : %d Allocations, %s, %d Leaking Arenas\n",
		analysis.TotalLiveAllocations, formatting.FormatMemoryBytes(analysis.TotalLiveBytes), analysis.LeakingAllocators))
	builder.WriteString(fmt.Sprintf("Peak      : %d Allocations, %s\n",
		analysis.TotalPeakLiveAllocations, formatting.FormatMemoryBytes(analysis.TotalPeakLiveBytes)))
	builder.WriteString(fmt.Sprintf("Ever      : %d Allocations, %s\n",
		analysis.TotalEverAllocations, formatting.FormatMemoryBytes(analysis.TotalEverBytes)))
	renderer.WriteColor(builder, internal.ColorReset)

	builder.WriteString("Verdict   : ")
	if analysis.LeakDetected {
		renderer.WriteColor(builder, internal.ColorFail)
		builder.WriteString("MEMORY LEAK DETECTED\n")
	} else {
		renderer.WriteColor(builder, internal.ColorPass)
		builder.WriteString("No leaks detected\n")
	}
	renderer.WriteColor(builder, internal.ColorReset)

	if analysis.LeakDetected || analysis.TotalAllocators > 0 {
		renderArenaSummary(renderer, builder, analysis.Arenas)
	}
	builder.WriteString("\n")
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

	analysis := memoryTimelineAnalyzeFromConfig(cfg, memoryTimelineCollect(), "memory", "")
	if !shouldRenderMemoryDiagnostics(mode, analysis) {
		return
	}

	renderMemoryDiagnostics(renderer, builder, analysis)
}

func memoryTimelineAnalyzeFromConfig(cfg *ShieldConfiguration, snapshot memforge.MemforgeMemoryTimelineSnapshot, sourceKind, sourcePath string) memforge.MemforgeMemoryTimelineAnalysis {
	analysis := memforge.MemforgeMemoryTimelineAnalyze(snapshot, shieldStackFilterFromConfig(cfg))
	analysis.SourceKind = sourceKind
	analysis.SourcePath = sourcePath
	return analysis
}

type MemoryTimelineMode string

const (
	MemoryTimelineOff    MemoryTimelineMode = "off"
	MemoryTimelineRender MemoryTimelineMode = "render"
	MemoryTimelineExport MemoryTimelineMode = "export"
	MemoryTimelineBoth   MemoryTimelineMode = "both"
)

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

	snapshot := memoryTimelineCollect()
	views := memoryTimelineViewsFromConfig(cfg)

	if !snapshot.Available && (mode == MemoryTimelineRender || mode == MemoryTimelineBoth) {
		renderMemoryTimelineUnavailable(renderer, builder)
		return
	}
	if !snapshot.Available && mode == MemoryTimelineExport {
		renderer.WriteColor(builder, internal.ColorFail)
		builder.WriteString("Memory timeline export skipped: memforge debugger unavailable (rebuild with -tags memforge_debug)\n")
		renderer.WriteColor(builder, internal.ColorReset)
		return
	}

	switch mode {
	case MemoryTimelineRender:
		analysis := memoryTimelineAnalyzeFromConfig(cfg, snapshot, "memory", "")
		renderMemoryTimelineAnalysis(renderer, builder, analysis, views)
	case MemoryTimelineExport:
		if err := exportMemoryTimelineJSONL(configPath, cfg.Runtime.MemoryTimelinePath, snapshot); err != nil {
			renderer.WriteColor(builder, internal.ColorFail)
			builder.WriteString(fmt.Sprintf("Memory timeline export failed: %v\n", err))
			renderer.WriteColor(builder, internal.ColorReset)
			return
		}
		logMemoryTimelineExportPath(renderer, builder, configPath, cfg.Runtime.MemoryTimelinePath)
	case MemoryTimelineBoth:
		analysis := memoryTimelineAnalyzeFromConfig(cfg, snapshot, "memory", "")
		renderMemoryTimelineAnalysis(renderer, builder, analysis, views)
		if err := exportMemoryTimelineJSONL(configPath, cfg.Runtime.MemoryTimelinePath, snapshot); err != nil {
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

func renderMemoryTimelineAnalysis(renderer *internal.Renderer, builder *strings.Builder, analysis memforge.MemforgeMemoryTimelineAnalysis, views []string) {
	renderer.WriteColor(builder, internal.ColorHeader)
	builder.WriteString("=== SHIELD MEMORY TIMELINE ===\n")
	renderer.WriteColor(builder, internal.ColorReset)

	if analysis.TotalEvents == 0 {
		renderer.WriteColor(builder, internal.ColorMuted)
		builder.WriteString("No timeline events recorded.\n")
		renderer.WriteColor(builder, internal.ColorReset)
		builder.WriteString("\n")
		return
	}

	renderMemoryTimelineAnalysisHeader(renderer, builder, analysis)

	if memoryTimelineViewsInclude(views, memoryTimelineViewSummary) {
		renderArenaSummary(renderer, builder, analysis.Arenas)
	}
	if memoryTimelineViewsInclude(views, memoryTimelineViewLeaks) && analysis.TotalLiveAllocations > 0 {
		renderLeakGroups(renderer, builder, analysis.LeakGroups)
	}
	if memoryTimelineViewsInclude(views, memoryTimelineViewTimeline) {
		renderGranularTimeline(renderer, builder, analysis.Events)
	}
	builder.WriteString("\n")
}

func renderMemoryTimelineAnalysisHeader(renderer *internal.Renderer, builder *strings.Builder, analysis memforge.MemforgeMemoryTimelineAnalysis) {
	kindCounts := make(map[string]int)
	var firstTS, lastTS time.Time
	for i, evt := range analysis.Events {
		kindCounts[string(evt.Kind)]++
		if i == 0 || evt.Timestamp.Before(firstTS) {
			firstTS = evt.Timestamp
		}
		if i == 0 || evt.Timestamp.After(lastTS) {
			lastTS = evt.Timestamp
		}
	}

	renderer.WriteColor(builder, internal.ColorMuted)
	builder.WriteString(fmt.Sprintf("Source    : %s", analysis.SourceKind))
	if analysis.SourcePath != "" {
		builder.WriteString(fmt.Sprintf(" (%s)", analysis.SourcePath))
	}
	builder.WriteString("\n")
	builder.WriteString(fmt.Sprintf("Events    : %d\n", analysis.TotalEvents))
	if !firstTS.IsZero() && !lastTS.IsZero() {
		builder.WriteString(fmt.Sprintf("Span      : %s to %s (%s)\n",
			firstTS.Format("15:04:05.000"),
			lastTS.Format("15:04:05.000"),
			lastTS.Sub(firstTS).Round(time.Millisecond)))
	}
	builder.WriteString(fmt.Sprintf("Live      : %d allocations, %s\n",
		analysis.TotalLiveAllocations, formatting.FormatMemoryBytes(analysis.TotalLiveBytes)))
	builder.WriteString(fmt.Sprintf("Peak      : %d allocations, %s\n",
		analysis.TotalPeakLiveAllocations, formatting.FormatMemoryBytes(analysis.TotalPeakLiveBytes)))
	builder.WriteString(fmt.Sprintf("Ever      : %d allocations, %s\n",
		analysis.TotalEverAllocations, formatting.FormatMemoryBytes(analysis.TotalEverBytes)))
	for _, kind := range sortedTimelineKinds(kindCounts) {
		builder.WriteString(fmt.Sprintf("  %-28s %d\n", kind+":", kindCounts[kind]))
	}
	renderer.WriteColor(builder, internal.ColorReset)
	builder.WriteString("\n")
}

func renderArenaSummary(renderer *internal.Renderer, builder *strings.Builder, arenas []memforge.MemforgeArenaSummary) {
	renderer.WriteColor(builder, internal.ColorHighlight)
	builder.WriteString("Arena Summary\n")
	renderer.WriteColor(builder, internal.ColorReset)

	if len(arenas) == 0 {
		renderer.WriteColor(builder, internal.ColorMuted)
		builder.WriteString("  (no allocators recorded)\n")
		renderer.WriteColor(builder, internal.ColorReset)
		return
	}

	for _, arena := range arenas {
		statusColor := internal.ColorPass
		statusLabel := "OK"
		if arena.Leaking {
			statusColor = internal.ColorFail
			statusLabel = "LEAKING"
		}
		renderer.WriteColor(builder, statusColor)
		builder.WriteString(fmt.Sprintf("  %-10s", statusLabel))
		renderer.WriteColor(builder, internal.ColorReset)
		builder.WriteString(fmt.Sprintf(" %-28s %s\n", arena.Name, formatMemoryDiagnosticAddress(arena.Address)))
		renderer.WriteColor(builder, internal.ColorMuted)
		age := arena.AgeAtCapture
		if age == 0 && !arena.CreatedAt.IsZero() {
			age = time.Since(arena.CreatedAt)
		}
		builder.WriteString(fmt.Sprintf("             age=%s live=%d/%s peak=%d/%s ever=%d/%s\n",
			age.Round(time.Millisecond).String(),
			arena.LiveAllocations, formatting.FormatMemoryBytes(arena.LiveBytes),
			arena.PeakLiveAllocations, formatting.FormatMemoryBytes(arena.PeakLiveBytes),
			arena.EverAllocations, formatting.FormatMemoryBytes(arena.EverBytes)))
		if arena.CurrentArenaDataCapBytes > 0 {
			arenaLine := fmt.Sprintf("             arena=%s", formatting.FormatMemoryBytes(arena.CurrentArenaDataCapBytes))
			if arena.CurrentArenaTotalBytes > 0 {
				arenaLine += fmt.Sprintf(" mmap=%s", formatting.FormatMemoryBytes(arena.CurrentArenaTotalBytes))
			}
			if arena.PeakArenaDataCapBytes > arena.CurrentArenaDataCapBytes {
				arenaLine += fmt.Sprintf(" peak=%s", formatting.FormatMemoryBytes(arena.PeakArenaDataCapBytes))
			}
			if arena.LiveBytes > 0 {
				utilPct := float64(arena.LiveBytes) * 100 / float64(arena.CurrentArenaDataCapBytes)
				arenaLine += fmt.Sprintf(" util=%.1f%%", utilPct)
			}
			builder.WriteString(arenaLine + "\n")
		}
		if len(arena.CapacitySegments) > 1 {
			for _, segment := range arena.CapacitySegments {
				endLabel := "capture"
				if !segment.EndedAt.IsZero() {
					endLabel = segment.EndedAt.Format("15:04:05.000")
				}
				segmentLine := fmt.Sprintf("             %s – %s  %s",
					segment.StartedAt.Format("15:04:05.000"),
					endLabel,
					formatting.FormatMemoryBytes(segment.DataCapBytes))
				if segment.GrownFromBytes > 0 && segment.DataCapBytes > segment.GrownFromBytes {
					segmentLine += fmt.Sprintf(" (+%s)",
						formatting.FormatMemoryBytes(segment.DataCapBytes-segment.GrownFromBytes))
				}
				builder.WriteString(segmentLine + "\n")
			}
		}
		if len(arena.FilteredCreatorStack) > 0 {
			builder.WriteString("             created: ")
			builder.WriteString(strings.Join(arena.FilteredCreatorStack, " → "))
			builder.WriteString("\n")
		}
		renderer.WriteColor(builder, internal.ColorReset)
	}
	builder.WriteString("\n")
}

func renderLeakGroups(renderer *internal.Renderer, builder *strings.Builder, groups []memforge.MemforgeAllocationStackGroup) {
	renderer.WriteColor(builder, internal.ColorHighlight)
	builder.WriteString("Leak Groups (by filtered stack)\n")
	renderer.WriteColor(builder, internal.ColorReset)

	for _, group := range groups {
		builder.WriteString(fmt.Sprintf("  %d alloc %s via %s\n",
			group.AllocationCount,
			formatting.FormatMemoryBytes(group.TotalBytes),
			group.SampleAllocator))
		if len(group.FilteredStack) > 0 {
			renderer.WriteColor(builder, internal.ColorMuted)
			builder.WriteString("    ")
			builder.WriteString(strings.Join(group.FilteredStack, " → "))
			builder.WriteString("\n")
			renderer.WriteColor(builder, internal.ColorReset)
		}
	}
	builder.WriteString("\n")
}

func renderGranularTimeline(renderer *internal.Renderer, builder *strings.Builder, events []memforge.MemforgeTimelineEvent) {
	renderer.WriteColor(builder, internal.ColorHighlight)
	builder.WriteString("Granular Timeline\n")
	renderer.WriteColor(builder, internal.ColorReset)

	for _, evt := range events {
		renderGranularTimelineEvent(renderer, builder, evt)
	}
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

func renderGranularTimelineEvent(renderer *internal.Renderer, builder *strings.Builder, evt memforge.MemforgeTimelineEvent) {
	renderer.WriteColor(builder, internal.ColorHighlight)
	builder.WriteString(fmt.Sprintf("#%-6d ", evt.Seq))
	renderer.WriteColor(builder, internal.ColorReset)
	builder.WriteString(fmt.Sprintf("%s %-28s ", evt.Timestamp.Format(time.RFC3339Nano), evt.Kind))
	builder.WriteString(fmt.Sprintf("%s (%s)\n", evt.AllocatorName, formatMemoryDiagnosticAddress(evt.AllocatorAddress)))

	switch evt.Kind {
	case memforge.MemforgeTimelineEventAllocation, memforge.MemforgeTimelineEventFreeManual:
		builder.WriteString(fmt.Sprintf("  addr=%s size=%s",
			formatMemoryDiagnosticAddress(evt.AllocationAddress),
			formatting.FormatMemoryBytes(evt.SizeBytes)))
		if evt.Kind == memforge.MemforgeTimelineEventFreeManual {
			builder.WriteString(fmt.Sprintf(" orig=#%d@%s", evt.OriginalSeq, evt.OriginalCreatedAt.Format(time.RFC3339Nano)))
		}
		builder.WriteString("\n")
	case memforge.MemforgeTimelineEventFreeRegionalReset, memforge.MemforgeTimelineEventFreeRegionalDestroy:
		builder.WriteString(fmt.Sprintf("  freed=%d allocation(s)\n", len(evt.FreedAllocations)))
		for _, freed := range evt.FreedAllocations {
			builder.WriteString(fmt.Sprintf("    - %s %s orig=#%d@%s\n",
				formatMemoryDiagnosticAddress(freed.AllocationAddress),
				formatting.FormatMemoryBytes(freed.SizeBytes),
				freed.OriginalSeq,
				freed.OriginalCreatedAt.Format(time.RFC3339Nano)))
		}
	}

	if strings.TrimSpace(evt.Stack) != "" {
		renderer.WriteColor(builder, internal.ColorMuted)
		builder.WriteString("  ")
		builder.WriteString(evt.Stack)
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

func exportMemoryTimelineJSONL(configPath, configuredPath string, snapshot memforge.MemforgeMemoryTimelineSnapshot) error {
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

	return memforge.MemforgeMemoryTimelineSnapshotWriteJSONL(file, snapshot)
}

func resolveMemoryTimelineUserPath(configPath, path string, fromConfig bool) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", fmt.Errorf("memory timeline path is empty")
	}
	if filepath.IsAbs(trimmed) {
		return trimmed, nil
	}
	if fromConfig {
		return resolveMemoryTimelinePath(configPath, trimmed)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to resolve working directory: %w", err)
	}
	return filepath.Join(cwd, trimmed), nil
}

func runAnalyzeTimelineCommand(renderer *internal.Renderer, builder *strings.Builder, cfg *ShieldConfiguration, configPath string, args []string) bool {
	fromConfig := len(args) == 0
	path := strings.TrimSpace(cfg.Runtime.MemoryTimelinePath)
	if len(args) > 0 {
		path = strings.TrimSpace(args[0])
	}
	if path == "" {
		renderer.WriteColor(builder, internal.ColorFail)
		builder.WriteString("No timeline path provided and runtime.memory_timeline_path is empty.\n")
		renderer.WriteColor(builder, internal.ColorReset)
		return false
	}

	resolved, err := resolveMemoryTimelineUserPath(configPath, path, fromConfig)
	if err != nil {
		renderer.WriteColor(builder, internal.ColorFail)
		builder.WriteString(fmt.Sprintf("Failed to resolve timeline path: %v\n", err))
		renderer.WriteColor(builder, internal.ColorReset)
		return false
	}

	file, err := os.Open(resolved)
	if err != nil {
		renderer.WriteColor(builder, internal.ColorFail)
		builder.WriteString(fmt.Sprintf("Failed to open timeline file: %v\n", err))
		renderer.WriteColor(builder, internal.ColorReset)
		return false
	}
	defer file.Close()

	snapshot, err := memforge.MemforgeMemoryTimelineSnapshotReadJSONL(file)
	if err != nil {
		renderer.WriteColor(builder, internal.ColorFail)
		builder.WriteString(fmt.Sprintf("Failed to read timeline JSONL: %v\n", err))
		renderer.WriteColor(builder, internal.ColorReset)
		return false
	}

	analysis := memoryTimelineAnalyzeFromConfig(cfg, snapshot, "jsonl", resolved)
	renderMemoryTimelineAnalysis(renderer, builder, analysis, memoryTimelineViewsFromConfig(cfg))
	return false
}
