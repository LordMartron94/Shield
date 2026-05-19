//go:build !memforge_debug

package runner

func memoryDiagnosticsCollect() MemoryDiagnosticResult {
	return MemoryDiagnosticResult{
		Provider: "memforge",
	}
}
