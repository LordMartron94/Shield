package internal

import (
	"blaze/reduce"
	"echo"
	"fmt"
	"foundation/formatting"
	"memarch"
	"memcore"
	"memforge"
	"memstruct"
	"statarch/core"
	"statarch/descriptive"
)

type shieldRunTelemetry struct {
	atomDurationsNs []float64
}

type shieldRunAggregates struct {
	unitsVisited            int
	unitsSkippedBlacklist   int
	unitsSkippedSetup       int
	atomValidationFailures  int
	atomPanics              int
	atomSetupFailures       int
	subUnitLoopEarlyStops   int
	unitsFailedGate         int
}

func shieldRunReportAggregate(report ShieldRunReport) shieldRunAggregates {
	var a shieldRunAggregates

	for _, tl := range report.TopLevel {
		shieldAggregatesAddUnitReport(&a, tl.Report)

		if tl.Failed {
			a.unitsFailedGate++
		}
	}

	return a
}

func shieldAggregatesAddUnitReport(a *shieldRunAggregates, r ShieldUnitRunReport) {
	a.unitsVisited++

	if r.SkippedDueToBlacklist {
		a.unitsSkippedBlacklist++

		return
	}

	if r.SkippedDueToSetup {
		a.unitsSkippedSetup++

		return
	}

	a.atomValidationFailures += r.AtomValidationFailures
	a.atomPanics += r.AtomPanics
	a.atomSetupFailures += r.AtomSetupFailureCount

	if r.TerminatedSubUnitLoopEarly {
		a.subUnitLoopEarlyStops++
	}

	for _, ch := range r.DirectChildren {
		if ch.Failed {
			a.unitsFailedGate++
		}

		shieldAggregatesAddUnitReport(a, ch.Report)
	}
}

func shieldRunSummaryEmit(report ShieldRunReport, tel *shieldRunTelemetry) {
	agg := shieldRunReportAggregate(report)

	elapsedNs := float64(report.Elapsed.Nanoseconds())

	logger := echo.On(shieldSystemID).
		Field("summary", true).
		Field("run_failed", report.Failed).
		Field("elapsed_ns", report.Elapsed.Nanoseconds()).
		Field("elapsed", formatting.FormatDurationNSF64(elapsedNs)).
		Field("units_visited", agg.unitsVisited).
		Field("units_skipped_blacklist", agg.unitsSkippedBlacklist).
		Field("units_skipped_setup", agg.unitsSkippedSetup).
		Field("atom_validation_failures", agg.atomValidationFailures).
		Field("atom_panics", agg.atomPanics).
		Field("atom_setup_failures", agg.atomSetupFailures).
		Field("subunit_loop_early_stops", agg.subUnitLoopEarlyStops).
		Field("units_failed_gate", agg.unitsFailedGate)

	n := 0
	if tel != nil {
		n = len(tel.atomDurationsNs)
	}

	logger = logger.Field("atoms_timed", n)

	if n == 0 {
		logger.Info("Shield run summary")

		return
	}

	allocator := memforge.FixedLinearAllocatorCreate(uint64(memcore.MegaByte))
	defer memforge.FixedLinearAllocatorDestroy(allocator)

	allocFn := func(sizeBytes, alignment uint64) memcore.MarkRaw {
		return memforge.FixedLinearAllocatorMalloc(allocator, sizeBytes, alignment)
	}

	vecMark, _ := memarch.MemArchVectorCreate[float64](allocFn, uint64(n))

	err := memstruct.VectorSetFromSliceRange(vecMark, tel.atomDurationsNs, 0, uint64(n), 0)
	if err != nil {
		logger.Field("summary_stats_error", err.Error()).Info("Shield run summary")

		return
	}

	analysis := core.StatArchAnalysisCreate[float64](vecMark, allocFn)

	mean := descriptive.StatArchDescriptiveVectorMeanF64(analysis)
	minV, maxV := reduce.BlazeReduceVectorMinMax[float64](vecMark)

	var stddev float64

	if n >= 2 {
		stddev = descriptive.StatArchDescriptiveVectorStandardDeviationF64(analysis, false)
	}

	logger.
		Field("atom_duration_mean_ns", mean).
		Field("atom_duration_stddev_pop_ns", stddev).
		Field("atom_duration_min_ns", float64(minV)).
		Field("atom_duration_max_ns", float64(maxV)).
		Field("atom_duration_mean", formatting.FormatDurationNSF64(mean)).
		Field("atom_duration_min", formatting.FormatDurationNSF64(float64(minV))).
		Field("atom_duration_max", formatting.FormatDurationNSF64(float64(maxV))).
		Info(fmt.Sprintf(
			"Shield run summary (atom timing mean=%s min=%s max=%s)",
			formatting.FormatDurationNSF64(mean),
			formatting.FormatDurationNSF64(float64(minV)),
			formatting.FormatDurationNSF64(float64(maxV)),
		))
}
