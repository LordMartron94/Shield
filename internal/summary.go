package internal

import (
	"echo"
	"fmt"
	"foundation/formatting"
	"math"
	"memarch"
	"memcore"
	"memforge"
	"memstruct"
	"statarch/core"
	"statarch/descriptive"
	"time"
)

type shieldRunTelemetry struct {
	atomDurationsNs []float64
}

// ShieldRunAggregates holds tree-walk counters for a completed run (exported for ReportPersistAdapter).
type ShieldRunAggregates struct {
	UnitsVisited            int
	UnitsSkippedBlacklist   int
	UnitsSkippedSetup       int
	AtomValidationFailures  int
	AtomPanics              int
	AtomSetupFailures       int
	SubUnitLoopEarlyStops   int
	UnitsFailedGate         int
}

// ShieldRunMetrics is the snapshot passed to echo summary, disk writers, and custom adapters.
type ShieldRunMetrics struct {
	WallTime   time.Time
	Aggregates ShieldRunAggregates

	AtomsTimedN int

	DurationStatsOK    bool
	DurationStatsError string
	MeanNs             float64
	StddevPopNs        float64
	MinNs              float64
	MaxNs              float64

	DurationSumNs       float64
	DurationMedianNs    float64
	DurationQ1Ns        float64
	DurationQ3Ns        float64
	DurationIQRNs       float64
	DurationP95Ns       float64
	DurationP99Ns       float64
	DurationCoeffVarPop float64
}

const shieldDurationMeanEpsilonNs = 1e-9

func shieldRunReportAggregate(report ShieldRunReport) ShieldRunAggregates {
	var a ShieldRunAggregates

	for _, tl := range report.TopLevel {
		shieldAggregatesAddUnitReport(&a, tl.Report)

		if tl.Failed {
			a.UnitsFailedGate++
		}
	}

	return a
}

func shieldAggregatesAddUnitReport(a *ShieldRunAggregates, r ShieldUnitRunReport) {
	a.UnitsVisited++

	if r.SkippedDueToBlacklist {
		a.UnitsSkippedBlacklist++

		return
	}

	if r.SkippedDueToSetup {
		a.UnitsSkippedSetup++

		return
	}

	a.AtomValidationFailures += r.AtomValidationFailures
	a.AtomPanics += r.AtomPanics
	a.AtomSetupFailures += r.AtomSetupFailureCount

	if r.TerminatedSubUnitLoopEarly {
		a.SubUnitLoopEarlyStops++
	}

	for _, ch := range r.DirectChildren {
		if ch.Failed {
			a.UnitsFailedGate++
		}

		shieldAggregatesAddUnitReport(a, ch.Report)
	}
}

func shieldRunMetricsBuild(report ShieldRunReport, tel *shieldRunTelemetry) ShieldRunMetrics {
	m := ShieldRunMetrics{
		WallTime:   time.Now().UTC(),
		Aggregates: shieldRunReportAggregate(report),
	}

	if tel == nil {
		return m
	}

	n := len(tel.atomDurationsNs)
	m.AtomsTimedN = n

	if n == 0 {
		return m
	}

	allocator := memforge.FixedLinearAllocatorCreate(uint64(memcore.MegaByte))
	defer memforge.FixedLinearAllocatorDestroy(allocator)

	allocFn := func(sizeBytes, alignment uint64) memcore.MarkRaw {
		return memforge.FixedLinearAllocatorMalloc(allocator, sizeBytes, alignment)
	}

	vecMark, _ := memarch.MemArchVectorCreate[float64](allocFn, uint64(n))

	err := memstruct.VectorSetFromSliceRange(vecMark, tel.atomDurationsNs, 0, uint64(n), 0)
	if err != nil {
		m.DurationStatsError = err.Error()

		return m
	}

	analysis := core.StatArchAnalysisCreate[float64](vecMark, allocFn)

	m.MeanNs = descriptive.StatArchDescriptiveVectorMeanF64(analysis)

	if n >= 2 {
		m.StddevPopNs = descriptive.StatArchDescriptiveVectorStandardDeviationF64(analysis, false)
	}

	m.DurationSumNs = descriptive.StatArchDescriptiveVectorSumF64(analysis)

	five := descriptive.StatArchDescriptiveVectorFiveNumberSummaryF64(analysis)
	m.MinNs = five.Min
	m.MaxNs = five.Max
	m.DurationMedianNs = five.Median
	m.DurationQ1Ns = five.Q1
	m.DurationQ3Ns = five.Q3
	m.DurationIQRNs = five.Q3 - five.Q1

	m.DurationP95Ns = descriptive.StatArchDescriptiveVectorPercentileF64(analysis, 95)
	m.DurationP99Ns = descriptive.StatArchDescriptiveVectorPercentileF64(analysis, 99)

	if n >= 2 && math.Abs(m.MeanNs) > shieldDurationMeanEpsilonNs {
		m.DurationCoeffVarPop = descriptive.StatArchDescriptiveVectorCoefficientVariantF64(analysis, false)
	}

	m.DurationStatsOK = true

	return m
}

func shieldRunSummaryEmit(report ShieldRunReport, metrics ShieldRunMetrics) {
	elapsedNs := float64(report.Elapsed.Nanoseconds())

	logger := echo.On(shieldSystemID).
		Field("summary", true).
		Field("run_failed", report.Failed).
		Field("elapsed_ns", report.Elapsed.Nanoseconds()).
		Field("elapsed", formatting.FormatDurationNSF64(elapsedNs)).
		Field("units_visited", metrics.Aggregates.UnitsVisited).
		Field("units_skipped_blacklist", metrics.Aggregates.UnitsSkippedBlacklist).
		Field("units_skipped_setup", metrics.Aggregates.UnitsSkippedSetup).
		Field("atom_validation_failures", metrics.Aggregates.AtomValidationFailures).
		Field("atom_panics", metrics.Aggregates.AtomPanics).
		Field("atom_setup_failures", metrics.Aggregates.AtomSetupFailures).
		Field("subunit_loop_early_stops", metrics.Aggregates.SubUnitLoopEarlyStops).
		Field("units_failed_gate", metrics.Aggregates.UnitsFailedGate).
		Field("atoms_timed", metrics.AtomsTimedN)

	if metrics.AtomsTimedN == 0 {
		logger.Info("Shield run summary")

		return
	}

	if metrics.DurationStatsError != "" {
		logger.Field("summary_stats_error", metrics.DurationStatsError).Info("Shield run summary")

		return
	}

	if !metrics.DurationStatsOK {
		logger.Info("Shield run summary")

		return
	}

	logger = logger.
		Field("atom_duration_mean_ns", metrics.MeanNs).
		Field("atom_duration_stddev_pop_ns", metrics.StddevPopNs).
		Field("atom_duration_min_ns", metrics.MinNs).
		Field("atom_duration_max_ns", metrics.MaxNs).
		Field("atom_duration_sum_ns", metrics.DurationSumNs).
		Field("atom_duration_median_ns", metrics.DurationMedianNs).
		Field("atom_duration_p95_ns", metrics.DurationP95Ns).
		Field("atom_duration_p99_ns", metrics.DurationP99Ns).
		Field("atom_duration_iqr_ns", metrics.DurationIQRNs).
		Field("atom_duration_mean", formatting.FormatDurationNSF64(metrics.MeanNs)).
		Field("atom_duration_min", formatting.FormatDurationNSF64(metrics.MinNs)).
		Field("atom_duration_max", formatting.FormatDurationNSF64(metrics.MaxNs)).
		Field("atom_duration_median", formatting.FormatDurationNSF64(metrics.DurationMedianNs)).
		Field("atom_duration_p95", formatting.FormatDurationNSF64(metrics.DurationP95Ns)).
		Field("atom_duration_p99", formatting.FormatDurationNSF64(metrics.DurationP99Ns))

	if metrics.AtomsTimedN >= 2 && math.Abs(metrics.MeanNs) > shieldDurationMeanEpsilonNs {
		logger = logger.Field("atom_duration_coeff_var_pop", metrics.DurationCoeffVarPop)
	}

	logger.Info(fmt.Sprintf(
		"Shield run summary (atom timing mean=%s median=%s p95=%s)",
		formatting.FormatDurationNSF64(metrics.MeanNs),
		formatting.FormatDurationNSF64(metrics.DurationMedianNs),
		formatting.FormatDurationNSF64(metrics.DurationP95Ns),
	))
}
