package runner

import (
	"fmt"
	"memforge"
	"strings"
)

const (
	memoryTimelineViewSummary  = "summary"
	memoryTimelineViewLeaks    = "leaks"
	memoryTimelineViewTimeline = "timeline"
)

func shieldStackFilterFromConfig(cfg *ShieldConfiguration) memforge.MemforgeStackFilter {
	if cfg == nil {
		return memforge.MemforgeStackFilter{}
	}
	return memforge.MemforgeStackFilter{
		IgnorePrefixes: append([]string(nil), cfg.Runtime.MemoryStackIgnorePrefixes...),
		IgnoreContains: append([]string(nil), cfg.Runtime.MemoryStackIgnoreContains...),
		MaxDepth:       cfg.Runtime.MemoryStackMaxDepth,
		BoundaryMode:   cfg.Runtime.MemoryStackBoundaryMode,
	}
}

func memoryTimelineViewsFromConfig(cfg *ShieldConfiguration) []string {
	if cfg == nil || len(cfg.Runtime.MemoryTimelineViews) == 0 {
		return []string{memoryTimelineViewSummary}
	}
	return normalizeMemoryTimelineViews(cfg.Runtime.MemoryTimelineViews)
}

func normalizeMemoryTimelineViews(views []string) []string {
	seen := make(map[string]bool)
	normalized := make([]string, 0, len(views))
	for _, view := range views {
		trimmed := strings.TrimSpace(strings.ToLower(view))
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		normalized = append(normalized, trimmed)
	}
	if len(normalized) == 0 {
		return []string{memoryTimelineViewSummary}
	}
	return normalized
}

func validateMemoryTimelineViews(views []string) error {
	if len(views) == 0 {
		return nil
	}
	for _, view := range views {
		trimmed := strings.TrimSpace(strings.ToLower(view))
		if trimmed == "" {
			continue
		}
		switch trimmed {
		case memoryTimelineViewSummary, memoryTimelineViewLeaks, memoryTimelineViewTimeline:
			continue
		default:
			return fmt.Errorf("unknown runtime.memory_timeline_views entry '%s' (expected summary, leaks, or timeline)", view)
		}
	}
	return nil
}

func memoryTimelineViewsInclude(views []string, view string) bool {
	for _, candidate := range views {
		if candidate == view {
			return true
		}
	}
	return false
}
