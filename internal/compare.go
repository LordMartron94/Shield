package internal

import (
	"blaze"
	"blaze/elementwise"
	"fmt"
	"foundation/formatting"
	"memarch"
	"memcore"
	"memforge"
	"memstruct"
	"slices"
	"sort"
	"statarch/core"
	"statarch/descriptive"
	"strings"
)

type ShieldCompareSeverity uint8

const (
	ShieldCompareSeverityInfo ShieldCompareSeverity = iota + 1
	ShieldCompareSeverityWarn
	ShieldCompareSeverityRegression
)

type ShieldCompareVerdict uint8

const (
	ShieldCompareVerdictNoRegression ShieldCompareVerdict = iota + 1
	ShieldCompareVerdictRegression
	ShieldCompareVerdictMixed
)

type ShieldCompareOptions struct {
	SlowPctThreshold    float64
	SlowAbsThresholdNs  int64
	TopSlowRegressionsN int
}

type ShieldComparePathChange struct {
	Path string
}

type ShieldCompareTimingDelta struct {
	Path             string
	BaselineNs       int64
	CandidateNs      int64
	DeltaNs          int64
	DeltaPct         float64
	IsSlowRegression bool
}

type ShieldCompareStructural struct {
	UnitsAdded   []ShieldComparePathChange
	UnitsRemoved []ShieldComparePathChange
	AtomsAdded   []ShieldComparePathChange
	AtomsRemoved []ShieldComparePathChange
	CasesAdded   []ShieldComparePathChange
	CasesRemoved []ShieldComparePathChange
}

type ShieldCompareOutcomes struct {
	NewFailingCases      []string
	ResolvedFailingCases []string
	NewSkippedCases      []string
	UnskippedCases       []string
	UnitsFailedDelta     int
	AtomsFailedDelta     int
	CasesFailedDelta     int
	UnitsSkippedDelta    int
	AtomsSkippedDelta    int
	CasesSkippedDelta    int
}

type ShieldCompareTiming struct {
	RunDeltaNs          int64
	RunDeltaPct         float64
	AtomSlowRegressions []ShieldCompareTimingDelta
	CaseSlowRegressions []ShieldCompareTimingDelta
}

type ShieldCompareResult struct {
	BaselinePath  string
	CandidatePath string
	Warnings      []string
	Verdict       ShieldCompareVerdict
	Structural    ShieldCompareStructural
	Outcomes      ShieldCompareOutcomes
	Timing        ShieldCompareTiming
}

type shieldCompareUnitNode struct {
	Path    string
	Failed  bool
	Skipped bool
}

type shieldCompareAtomNode struct {
	Path      string
	Failed    bool
	Skipped   bool
	ElapsedNs int64
}

type shieldCompareCaseNode struct {
	Path      string
	Failed    bool
	Skipped   bool
	ElapsedNs int64
}

type shieldCompareIndex struct {
	units map[string]shieldCompareUnitNode
	atoms map[string]shieldCompareAtomNode
	cases map[string]shieldCompareCaseNode
}

func ShieldCompareOptionsDefault() ShieldCompareOptions {
	return ShieldCompareOptions{
		SlowPctThreshold:    10.0,
		SlowAbsThresholdNs:  int64(1e6),
		TopSlowRegressionsN: 10,
	}
}

func ShieldCompareReportDocuments(
	baseline ShieldReportDocument,
	candidate ShieldReportDocument,
	options ShieldCompareOptions,
) ShieldCompareResult {
	options = shieldCompareOptionsNormalize(options)
	result := ShieldCompareResult{
		Warnings: make([]string, 0),
		Verdict:  ShieldCompareVerdictNoRegression,
	}

	if baseline.SchemaVersion < 2 || candidate.SchemaVersion < 2 {
		result.Warnings = append(result.Warnings, fmt.Sprintf(
			"incompatible schema for deep compare: baseline=%d candidate=%d",
			baseline.SchemaVersion, candidate.SchemaVersion))
		return result
	}

	if baseline.SchemaVersion != candidate.SchemaVersion {
		result.Warnings = append(result.Warnings, fmt.Sprintf(
			"schema mismatch: baseline=%d candidate=%d (partial compare may still be useful)",
			baseline.SchemaVersion, candidate.SchemaVersion))
	}

	baselineIndex := shieldCompareIndexBuild(baseline)
	candidateIndex := shieldCompareIndexBuild(candidate)

	result.Structural = shieldCompareStructuralBuild(baselineIndex, candidateIndex)
	result.Outcomes = shieldCompareOutcomesBuild(baseline, candidate, baselineIndex, candidateIndex)
	result.Timing = shieldCompareTimingBuild(baseline, candidate, baselineIndex, candidateIndex, options)
	result.Verdict = shieldCompareVerdictBuild(result)

	return result
}

func ShieldCompareResultFormatTXT(result ShieldCompareResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Shield compare report\n")
	if result.BaselinePath != "" {
		fmt.Fprintf(&b, "Baseline: %s\n", result.BaselinePath)
	}
	if result.CandidatePath != "" {
		fmt.Fprintf(&b, "Candidate: %s\n", result.CandidatePath)
	}
	fmt.Fprintf(&b, "Verdict: %s\n\n", shieldCompareVerdictString(result.Verdict))

	if len(result.Warnings) > 0 {
		b.WriteString("Warnings\n")
		for _, warning := range result.Warnings {
			fmt.Fprintf(&b, "  - %s\n", warning)
		}
		b.WriteString("\n")
	}

	b.WriteString("Outcome deltas\n")
	fmt.Fprintf(&b, "  Failed: units=%+d atoms=%+d cases=%+d\n",
		result.Outcomes.UnitsFailedDelta, result.Outcomes.AtomsFailedDelta, result.Outcomes.CasesFailedDelta)
	fmt.Fprintf(&b, "  Skipped: units=%+d atoms=%+d cases=%+d\n\n",
		result.Outcomes.UnitsSkippedDelta, result.Outcomes.AtomsSkippedDelta, result.Outcomes.CasesSkippedDelta)

	if len(result.Outcomes.NewFailingCases) > 0 {
		b.WriteString("New failing cases\n")
		for _, path := range result.Outcomes.NewFailingCases {
			fmt.Fprintf(&b, "  - %s\n", path)
		}
		b.WriteString("\n")
	}

	if len(result.Structural.UnitsAdded)+len(result.Structural.UnitsRemoved)+
		len(result.Structural.AtomsAdded)+len(result.Structural.AtomsRemoved)+
		len(result.Structural.CasesAdded)+len(result.Structural.CasesRemoved) > 0 {
		b.WriteString("Structural changes\n")
		shieldCompareWritePathChanges(&b, "Units added", result.Structural.UnitsAdded)
		shieldCompareWritePathChanges(&b, "Units removed", result.Structural.UnitsRemoved)
		shieldCompareWritePathChanges(&b, "Atoms added", result.Structural.AtomsAdded)
		shieldCompareWritePathChanges(&b, "Atoms removed", result.Structural.AtomsRemoved)
		shieldCompareWritePathChanges(&b, "Cases added", result.Structural.CasesAdded)
		shieldCompareWritePathChanges(&b, "Cases removed", result.Structural.CasesRemoved)
		b.WriteString("\n")
	}

	b.WriteString("Timing\n")
	fmt.Fprintf(&b, "  Run delta: %s (%+d ns, %+0.2f%%)\n",
		formatting.FormatDurationNSF64(float64(result.Timing.RunDeltaNs)),
		result.Timing.RunDeltaNs,
		result.Timing.RunDeltaPct)
	if len(result.Timing.AtomSlowRegressions) > 0 {
		b.WriteString("  Slow atom regressions\n")
		for _, delta := range result.Timing.AtomSlowRegressions {
			fmt.Fprintf(&b, "    - %s delta=%s (%+0.2f%%)\n",
				delta.Path,
				formatting.FormatDurationNSF64(float64(delta.DeltaNs)),
				delta.DeltaPct)
		}
	}
	if len(result.Timing.CaseSlowRegressions) > 0 {
		b.WriteString("  Slow case regressions\n")
		for _, delta := range result.Timing.CaseSlowRegressions {
			fmt.Fprintf(&b, "    - %s delta=%s (%+0.2f%%)\n",
				delta.Path,
				formatting.FormatDurationNSF64(float64(delta.DeltaNs)),
				delta.DeltaPct)
		}
	}

	return b.String()
}

func shieldCompareOptionsNormalize(options ShieldCompareOptions) ShieldCompareOptions {
	normalized := options
	if normalized.SlowPctThreshold <= 0 {
		normalized.SlowPctThreshold = 10.0
	}
	if normalized.SlowAbsThresholdNs <= 0 {
		normalized.SlowAbsThresholdNs = int64(1e6)
	}
	if normalized.TopSlowRegressionsN <= 0 {
		normalized.TopSlowRegressionsN = 10
	}
	return normalized
}

func shieldCompareIndexBuild(doc ShieldReportDocument) shieldCompareIndex {
	index := shieldCompareIndex{
		units: make(map[string]shieldCompareUnitNode),
		atoms: make(map[string]shieldCompareAtomNode),
		cases: make(map[string]shieldCompareCaseNode),
	}

	for _, top := range doc.Tree {
		shieldCompareIndexUnitAppend(&index, "", top.Name, top.Failed, top.Report)
	}

	return index
}

func shieldCompareIndexUnitAppend(index *shieldCompareIndex, prefix string, name string, failed bool, report ShieldReportUnitReport) {
	unitPath := name
	if prefix != "" {
		unitPath = prefix + "/" + name
	}
	index.units[unitPath] = shieldCompareUnitNode{
		Path:    unitPath,
		Failed:  failed,
		Skipped: report.SkippedDueToBlacklist || report.SkippedDueToSetup,
	}

	for _, atom := range report.Atoms {
		atomPath := unitPath + "." + atom.Name
		index.atoms[atomPath] = shieldCompareAtomNode{
			Path:      atomPath,
			Failed:    atom.Failed,
			Skipped:   atom.SkippedDueToBlacklist || atom.SkippedDueToSetup,
			ElapsedNs: atom.ElapsedNs,
		}

		for _, shieldCase := range atom.Cases {
			casePath := atomPath + "." + shieldCase.Name
			index.cases[casePath] = shieldCompareCaseNode{
				Path:      casePath,
				Failed:    shieldCase.Failed,
				Skipped:   shieldCase.Skipped,
				ElapsedNs: shieldCase.ElapsedNs,
			}
		}
	}

	for _, ch := range report.DirectChildren {
		shieldCompareIndexUnitAppend(index, unitPath, ch.Name, ch.Failed, ch.Report)
	}
}

func shieldCompareStructuralBuild(
	baseline shieldCompareIndex,
	candidate shieldCompareIndex,
) ShieldCompareStructural {
	return ShieldCompareStructural{
		UnitsAdded:   shieldComparePathDiff(baseline.units, candidate.units),
		UnitsRemoved: shieldComparePathDiff(candidate.units, baseline.units),
		AtomsAdded:   shieldComparePathDiff(baseline.atoms, candidate.atoms),
		AtomsRemoved: shieldComparePathDiff(candidate.atoms, baseline.atoms),
		CasesAdded:   shieldComparePathDiff(baseline.cases, candidate.cases),
		CasesRemoved: shieldComparePathDiff(candidate.cases, baseline.cases),
	}
}

func shieldComparePathDiff[T any](left map[string]T, right map[string]T) []ShieldComparePathChange {
	out := make([]ShieldComparePathChange, 0)
	for path := range right {
		if _, ok := left[path]; !ok {
			out = append(out, ShieldComparePathChange{Path: path})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func shieldCompareOutcomesBuild(
	baselineDoc ShieldReportDocument,
	candidateDoc ShieldReportDocument,
	baseline shieldCompareIndex,
	candidate shieldCompareIndex,
) ShieldCompareOutcomes {
	out := ShieldCompareOutcomes{
		NewFailingCases:      make([]string, 0),
		ResolvedFailingCases: make([]string, 0),
		NewSkippedCases:      make([]string, 0),
		UnskippedCases:       make([]string, 0),
		UnitsFailedDelta:     candidateDoc.Summary.UnitsFailed - baselineDoc.Summary.UnitsFailed,
		AtomsFailedDelta:     candidateDoc.Summary.AtomsFailed - baselineDoc.Summary.AtomsFailed,
		CasesFailedDelta:     candidateDoc.Summary.CasesFailed - baselineDoc.Summary.CasesFailed,
		UnitsSkippedDelta:    candidateDoc.Summary.UnitsSkipped - baselineDoc.Summary.UnitsSkipped,
		AtomsSkippedDelta:    candidateDoc.Summary.AtomsSkipped - baselineDoc.Summary.AtomsSkipped,
		CasesSkippedDelta:    candidateDoc.Summary.CasesSkipped - baselineDoc.Summary.CasesSkipped,
	}

	for path, now := range candidate.cases {
		old, ok := baseline.cases[path]
		if !ok {
			continue
		}
		if !old.Failed && now.Failed {
			out.NewFailingCases = append(out.NewFailingCases, path)
		}
		if old.Failed && !now.Failed {
			out.ResolvedFailingCases = append(out.ResolvedFailingCases, path)
		}
		if !old.Skipped && now.Skipped {
			out.NewSkippedCases = append(out.NewSkippedCases, path)
		}
		if old.Skipped && !now.Skipped {
			out.UnskippedCases = append(out.UnskippedCases, path)
		}
	}

	sort.Strings(out.NewFailingCases)
	sort.Strings(out.ResolvedFailingCases)
	sort.Strings(out.NewSkippedCases)
	sort.Strings(out.UnskippedCases)

	return out
}

func shieldCompareTimingBuild(
	baselineDoc ShieldReportDocument,
	candidateDoc ShieldReportDocument,
	baseline shieldCompareIndex,
	candidate shieldCompareIndex,
	options ShieldCompareOptions,
) ShieldCompareTiming {
	runDeltaNs, runDeltaPct := shieldCompareTimingDeltaScalarCompute(baselineDoc.ElapsedNs, candidateDoc.ElapsedNs)
	out := ShieldCompareTiming{
		RunDeltaNs:  runDeltaNs,
		RunDeltaPct: runDeltaPct,
	}

	atomPaths := make([]string, 0)
	atomBaseline := make([]int64, 0)
	atomCandidate := make([]int64, 0)
	atomDeltas := make([]ShieldCompareTimingDelta, 0)
	for path, now := range candidate.atoms {
		old, ok := baseline.atoms[path]
		if !ok {
			continue
		}
		atomPaths = append(atomPaths, path)
		atomBaseline = append(atomBaseline, old.ElapsedNs)
		atomCandidate = append(atomCandidate, now.ElapsedNs)
	}
	atomDeltaNs, atomDeltaPct := shieldCompareTimingDeltaSeriesCompute(atomBaseline, atomCandidate)
	for idx, path := range atomPaths {
		deltaNs := atomDeltaNs[idx]
		deltaPct := atomDeltaPct[idx]
		atomDeltas = append(atomDeltas, ShieldCompareTimingDelta{
			Path:             path,
			BaselineNs:       atomBaseline[idx],
			CandidateNs:      atomCandidate[idx],
			DeltaNs:          deltaNs,
			DeltaPct:         deltaPct,
			IsSlowRegression: deltaNs >= options.SlowAbsThresholdNs && deltaPct >= options.SlowPctThreshold,
		})
	}

	casePaths := make([]string, 0)
	caseBaseline := make([]int64, 0)
	caseCandidate := make([]int64, 0)
	caseDeltas := make([]ShieldCompareTimingDelta, 0)
	for path, now := range candidate.cases {
		old, ok := baseline.cases[path]
		if !ok {
			continue
		}
		casePaths = append(casePaths, path)
		caseBaseline = append(caseBaseline, old.ElapsedNs)
		caseCandidate = append(caseCandidate, now.ElapsedNs)
	}
	caseDeltaNs, caseDeltaPct := shieldCompareTimingDeltaSeriesCompute(caseBaseline, caseCandidate)
	for idx, path := range casePaths {
		deltaNs := caseDeltaNs[idx]
		deltaPct := caseDeltaPct[idx]
		caseDeltas = append(caseDeltas, ShieldCompareTimingDelta{
			Path:             path,
			BaselineNs:       caseBaseline[idx],
			CandidateNs:      caseCandidate[idx],
			DeltaNs:          deltaNs,
			DeltaPct:         deltaPct,
			IsSlowRegression: deltaNs >= options.SlowAbsThresholdNs && deltaPct >= options.SlowPctThreshold,
		})
	}

	sort.Slice(atomDeltas, func(i, j int) bool { return atomDeltas[i].DeltaNs > atomDeltas[j].DeltaNs })
	sort.Slice(caseDeltas, func(i, j int) bool { return caseDeltas[i].DeltaNs > caseDeltas[j].DeltaNs })

	out.AtomSlowRegressions = shieldCompareTopSlow(atomDeltas, options.TopSlowRegressionsN)
	out.CaseSlowRegressions = shieldCompareTopSlow(caseDeltas, options.TopSlowRegressionsN)

	return out
}

func shieldCompareTopSlow(all []ShieldCompareTimingDelta, topN int) []ShieldCompareTimingDelta {
	filtered := make([]ShieldCompareTimingDelta, 0)
	for _, item := range all {
		if item.IsSlowRegression {
			filtered = append(filtered, item)
		}
	}
	if len(filtered) <= topN {
		return filtered
	}
	return filtered[:topN]
}

func shieldCompareVerdictBuild(result ShieldCompareResult) ShieldCompareVerdict {
	hasReg := len(result.Outcomes.NewFailingCases) > 0 ||
		result.Outcomes.UnitsFailedDelta > 0 ||
		result.Outcomes.AtomsFailedDelta > 0 ||
		result.Outcomes.CasesFailedDelta > 0 ||
		len(result.Timing.AtomSlowRegressions) > 0 ||
		len(result.Timing.CaseSlowRegressions) > 0

	hasImprove := len(result.Outcomes.ResolvedFailingCases) > 0 ||
		result.Outcomes.UnitsFailedDelta < 0 ||
		result.Outcomes.AtomsFailedDelta < 0 ||
		result.Outcomes.CasesFailedDelta < 0

	if hasReg && hasImprove {
		return ShieldCompareVerdictMixed
	}
	if hasReg {
		return ShieldCompareVerdictRegression
	}
	return ShieldCompareVerdictNoRegression
}

func shieldCompareVerdictString(verdict ShieldCompareVerdict) string {
	switch verdict {
	case ShieldCompareVerdictRegression:
		return "REGRESSION"
	case ShieldCompareVerdictMixed:
		return "MIXED"
	default:
		return "NO_REGRESSION"
	}
}

func shieldCompareTimingDeltaScalarCompute(baselineNs int64, candidateNs int64) (deltaNs int64, deltaPct float64) {
	deltas, pct := shieldCompareTimingDeltaSeriesCompute([]int64{baselineNs}, []int64{candidateNs})
	return deltas[0], pct[0]
}

func shieldCompareTimingDeltaSeriesCompute(baselineNs []int64, candidateNs []int64) ([]int64, []float64) {
	n := len(baselineNs)
	if n == 0 || n != len(candidateNs) {
		return []int64{}, []float64{}
	}

	blaze.BlazeInitialize()

	allocator := memforge.FixedLinearAllocatorCreate(uint64(memcore.MegaByte))
	defer memforge.FixedLinearAllocatorDestroy(allocator)

	allocFn := func(sizeBytes, alignment uint64) memcore.MarkRaw {
		return memforge.FixedLinearAllocatorMalloc(allocator, sizeBytes, alignment)
	}

	baseF64 := make([]float64, n)
	candidateF64 := make([]float64, n)
	ones := make([]float64, n)
	hundred := make([]float64, n)

	for idx := range baselineNs {
		baseF64[idx] = float64(baselineNs[idx])
		candidateF64[idx] = float64(candidateNs[idx])
		ones[idx] = 1.0
		hundred[idx] = 100.0
	}

	baseVec, _ := memarch.MemArchVectorCreate[float64](allocFn, uint64(n))
	candidateVec, _ := memarch.MemArchVectorCreate[float64](allocFn, uint64(n))
	onesVec, _ := memarch.MemArchVectorCreate[float64](allocFn, uint64(n))
	hundredVec, _ := memarch.MemArchVectorCreate[float64](allocFn, uint64(n))
	deltaVec, _ := memarch.MemArchVectorCreate[float64](allocFn, uint64(n))
	ratioVec, _ := memarch.MemArchVectorCreate[float64](allocFn, uint64(n))
	ratioShiftVec, _ := memarch.MemArchVectorCreate[float64](allocFn, uint64(n))
	pctVec, _ := memarch.MemArchVectorCreate[float64](allocFn, uint64(n))

	memstruct.VectorSetFromSliceUnsafe(baseVec, baseF64)
	memstruct.VectorSetFromSliceUnsafe(candidateVec, candidateF64)
	memstruct.VectorSetFromSliceUnsafe(onesVec, ones)
	memstruct.VectorSetFromSliceUnsafe(hundredVec, hundred)

	elementwise.BlazeElementWiseVectorSubtractF64[float64, float64](candidateVec, baseVec, deltaVec)
	elementwise.BlazeElementWiseVectorDivideF64[float64, float64](candidateVec, baseVec, ratioVec)
	elementwise.BlazeElementWiseVectorSubtractF64[float64, float64](ratioVec, onesVec, ratioShiftVec)
	elementwise.BlazeElementWiseVectorMultiplyF64[float64, float64](ratioShiftVec, hundredVec, pctVec)

	analysis := core.StatArchAnalysisCreate[float64](pctVec, allocFn)
	_ = descriptive.StatArchDescriptiveVectorMeanF64(analysis)

	deltaF64 := make([]float64, 0, n)
	pctF64 := make([]float64, 0, n)
	memstruct.VectorUnaryReadOnlyExecute[float64](deltaVec, func(item float64) {
		deltaF64 = append(deltaF64, item)
	}, 1)
	memstruct.VectorUnaryReadOnlyExecute[float64](pctVec, func(item float64) {
		pctF64 = append(pctF64, item)
	}, 1)

	deltaNsOut := make([]int64, n)
	deltaPctOut := make([]float64, n)
	for idx := range deltaNsOut {
		deltaNsOut[idx] = int64(deltaF64[idx])
		if baselineNs[idx] == 0 {
			if candidateNs[idx] == 0 {
				deltaPctOut[idx] = 0
			} else {
				deltaPctOut[idx] = 100
			}
			continue
		}
		deltaPctOut[idx] = pctF64[idx]
	}

	return deltaNsOut, deltaPctOut
}

func shieldCompareWritePathChanges(b *strings.Builder, label string, values []ShieldComparePathChange) {
	if len(values) == 0 {
		return
	}
	fmt.Fprintf(b, "  %s (%d)\n", label, len(values))
	for _, value := range values {
		fmt.Fprintf(b, "    - %s\n", value.Path)
	}
}

func ShieldCompareResultHasRegression(result ShieldCompareResult) bool {
	return slices.Contains([]ShieldCompareVerdict{ShieldCompareVerdictRegression, ShieldCompareVerdictMixed}, result.Verdict)
}
