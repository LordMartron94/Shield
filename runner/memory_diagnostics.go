package runner

import (
	"fmt"
	"foundation/formatting"
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
