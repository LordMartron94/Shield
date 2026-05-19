//go:build memforge_debug

package runner

import "memforge"

func memoryTimelineCollect() memforge.MemforgeMemoryTimelineSnapshot {
	return memforge.MemforgeMemoryTimelineSnapshotGet()
}
