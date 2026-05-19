//go:build !memforge_debug

package runner

func memoryTimelineCollect() MemoryTimelineResult {
	return MemoryTimelineResult{
		Provider: "memforge",
	}
}
