//go:build memforge_debug

package runner

import (
	"memforge"
	"strings"
	"time"
)

func memoryTimelineCollect() MemoryTimelineResult {
	snapshot := memforge.MemforgeMemoryTimelineSnapshotGet()
	if !snapshot.Available {
		return MemoryTimelineResult{
			Provider: "memforge",
		}
	}

	events := make([]MemoryTimelineEvent, len(snapshot.Events))
	for i, evt := range snapshot.Events {
		events[i] = memoryTimelineEventFromMemforge(evt)
	}

	return MemoryTimelineResult{
		Available:  true,
		Provider:   "memforge",
		CapturedAt: snapshot.CapturedAt,
		EventCount: snapshot.EventCount,
		Events:     events,
	}
}

func memoryTimelineEventFromMemforge(evt memforge.MemforgeTimelineEvent) MemoryTimelineEvent {
	out := MemoryTimelineEvent{
		Seq:               evt.Seq,
		Timestamp:         evt.Timestamp.Format(time.RFC3339Nano),
		Kind:              string(evt.Kind),
		AllocatorAddress:  formatMemoryDiagnosticAddress(evt.AllocatorAddress),
		AllocatorName:     evt.AllocatorName,
		Stack:             timelineStackSplit(evt.Stack),
		AllocationAddress: formatMemoryDiagnosticAddress(evt.AllocationAddress),
		SizeBytes:         evt.SizeBytes,
		OriginalSeq:       evt.OriginalSeq,
	}
	if !evt.OriginalCreatedAt.IsZero() {
		out.OriginalCreatedAt = evt.OriginalCreatedAt.Format(time.RFC3339Nano)
	}
	if len(evt.FreedAllocations) > 0 {
		out.Freed = make([]MemoryTimelineFreedAllocation, len(evt.FreedAllocations))
		for i, freed := range evt.FreedAllocations {
			entry := MemoryTimelineFreedAllocation{
				AllocationAddress: formatMemoryDiagnosticAddress(freed.AllocationAddress),
				SizeBytes:         freed.SizeBytes,
				OriginalSeq:       freed.OriginalSeq,
			}
			if !freed.OriginalCreatedAt.IsZero() {
				entry.OriginalCreatedAt = freed.OriginalCreatedAt.Format(time.RFC3339Nano)
			}
			out.Freed[i] = entry
		}
	}
	return out
}

func timelineStackSplit(stack string) []string {
	trimmed := strings.TrimSpace(stack)
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, " → ")
}
