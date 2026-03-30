package internal

import (
	"echo"
	"encoding/json"
	"fmt"
	"foundation/formatting"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const shieldReportSchemaVersion = 2

// ShieldReportDocumentSchemaVersion is exported for clients documenting persisted JSON.
const ShieldReportDocumentSchemaVersion = shieldReportSchemaVersion

// ShieldReportPersistenceMode selects built-in report artifacts when no custom adapter is set.
type ShieldReportPersistenceMode uint8

const (
	ShieldReportPersistenceModeJSON ShieldReportPersistenceMode = iota + 1
	ShieldReportPersistenceModeTXT
	ShieldReportPersistenceModeJSONAndTXT
)

// ShieldReportPersistAdapter writes a run snapshot; return primary path for RunReport.WrittenReportPath.
// Advanced users register via ShieldReportPersistenceSetAdapter; when non-nil, Mode is ignored.
type ShieldReportPersistAdapter func(outputDir string, report ShieldRunReport, metrics ShieldRunMetrics) (primaryPath string, err error)

type ShieldReportPersistence struct {
	outputDir string
	enabled   bool
	mode      ShieldReportPersistenceMode
	adapter   ShieldReportPersistAdapter
}

func ShieldReportPersistenceCreate(outputDir string) *ShieldReportPersistence {
	return &ShieldReportPersistence{
		outputDir: outputDir,
		enabled:   false,
		mode:      ShieldReportPersistenceModeJSON,
		adapter:   nil,
	}
}

func ShieldReportPersistenceSetEnabled(p *ShieldReportPersistence, value bool) {
	if p == nil {
		return
	}

	p.enabled = value
}

func ShieldReportPersistenceSetMode(p *ShieldReportPersistence, mode ShieldReportPersistenceMode) {
	if p == nil {
		return
	}

	switch mode {
	case ShieldReportPersistenceModeJSON, ShieldReportPersistenceModeTXT, ShieldReportPersistenceModeJSONAndTXT:
		p.mode = mode
	default:
		p.mode = ShieldReportPersistenceModeJSON
	}
}

func ShieldReportPersistenceSetAdapter(p *ShieldReportPersistence, adapter ShieldReportPersistAdapter) {
	if p == nil {
		return
	}

	p.adapter = adapter
}

type ShieldReportDurationSummary struct {
	AtomsTimed  int     `json:"atoms_timed"`
	MeanNs      float64 `json:"mean_ns"`
	StddevPopNs float64 `json:"stddev_pop_ns"`
	MinNs       float64 `json:"min_ns"`
	MaxNs       float64 `json:"max_ns"`
	SumNs       float64 `json:"sum_ns"`
	MedianNs    float64 `json:"median_ns"`
	Q1Ns        float64 `json:"q1_ns"`
	Q3Ns        float64 `json:"q3_ns"`
	IQRNs       float64 `json:"iqr_ns"`
	P95Ns       float64 `json:"p95_ns"`
	P99Ns       float64 `json:"p99_ns"`
	CoeffVarPop float64 `json:"coeff_var_pop,omitempty"`
	MeanHuman   string  `json:"mean_human"`
	MinHuman    string  `json:"min_human"`
	MaxHuman    string  `json:"max_human"`
	MedianHuman string  `json:"median_human"`
	P95Human    string  `json:"p95_human"`
	P99Human    string  `json:"p99_human"`
	StatsError  string  `json:"stats_error,omitempty"`
}

type ShieldReportSummary struct {
	UnitsVisited           int                          `json:"units_visited"`
	UnitsSkippedBlacklist  int                          `json:"units_skipped_blacklist"`
	UnitsSkippedSetup      int                          `json:"units_skipped_setup"`
	AtomValidationFailures int                          `json:"atom_validation_failures"`
	AtomPanics             int                          `json:"atom_panics"`
	AtomSetupFailures      int                          `json:"atom_setup_failures"`
	SubUnitLoopEarlyStops  int                          `json:"subunit_loop_early_stops"`
	UnitsFailedGate        int                          `json:"units_failed_gate"`
	Duration               *ShieldReportDurationSummary `json:"duration,omitempty"`
}

type ShieldReportUnitReport struct {
	Name                       string                  `json:"name"`
	SkippedDueToBlacklist      bool                    `json:"skipped_due_to_blacklist"`
	SkippedDueToSetup          bool                    `json:"skipped_due_to_setup"`
	AtomSetupFailureCount      int                     `json:"atom_setup_failure_count"`
	AtomValidationFailures     int                     `json:"atom_validation_failures"`
	AtomPanics                 int                     `json:"atom_panics"`
	TerminatedSubUnitLoopEarly bool                    `json:"terminated_subunit_loop_early"`
	Atoms                      []ShieldReportAtomNode  `json:"atoms"`
	DirectChildren             []ShieldReportChildNode `json:"direct_children"`
}

type ShieldReportAtomNode struct {
	Name                  string `json:"name"`
	Failed                bool   `json:"failed"`
	SkippedDueToBlacklist bool   `json:"skipped_due_to_blacklist"`
	SkippedDueToSetup     bool   `json:"skipped_due_to_setup"`
	ValidationFailures    int    `json:"validation_failures"`
	Panics                int    `json:"panics"`
	ElapsedNs             int64  `json:"elapsed_ns"`
	ElapsedHuman          string `json:"elapsed_human"`
}

type ShieldReportChildNode struct {
	Name   string                 `json:"name"`
	Failed bool                   `json:"failed"`
	Report ShieldReportUnitReport `json:"report"`
}

type ShieldReportTopNode struct {
	Name   string                 `json:"name"`
	Failed bool                   `json:"failed"`
	Report ShieldReportUnitReport `json:"report"`
}

type ShieldReportDocument struct {
	SchemaVersion int                   `json:"schema_version"`
	WrittenAt     string                `json:"written_at"`
	RunFailed     bool                  `json:"run_failed"`
	ElapsedNs     int64                 `json:"elapsed_ns"`
	ElapsedHuman  string                `json:"elapsed_human"`
	Summary       ShieldReportSummary   `json:"summary"`
	Tree          []ShieldReportTopNode `json:"tree"`
}

func shieldReportUnitReportMapFromRun(r ShieldUnitRunReport) ShieldReportUnitReport {
	out := ShieldReportUnitReport{
		Name:                       r.Name,
		SkippedDueToBlacklist:      r.SkippedDueToBlacklist,
		SkippedDueToSetup:          r.SkippedDueToSetup,
		AtomSetupFailureCount:      r.AtomSetupFailureCount,
		AtomValidationFailures:     r.AtomValidationFailures,
		AtomPanics:                 r.AtomPanics,
		TerminatedSubUnitLoopEarly: r.TerminatedSubUnitLoopEarly,
		Atoms:                      make([]ShieldReportAtomNode, 0, len(r.Atoms)),
		DirectChildren:             make([]ShieldReportChildNode, 0, len(r.DirectChildren)),
	}

	for _, atom := range r.Atoms {
		elapsedNs := atom.Elapsed.Nanoseconds()
		out.Atoms = append(out.Atoms, ShieldReportAtomNode{
			Name:                  atom.Name,
			Failed:                atom.Failed,
			SkippedDueToBlacklist: atom.SkippedDueToBlacklist,
			SkippedDueToSetup:     atom.SkippedDueToSetup,
			ValidationFailures:    atom.ValidationFailures,
			Panics:                atom.Panics,
			ElapsedNs:             elapsedNs,
			ElapsedHuman:          formatting.FormatDurationNSF64(float64(elapsedNs)),
		})
	}

	for _, ch := range r.DirectChildren {
		out.DirectChildren = append(out.DirectChildren, ShieldReportChildNode{
			Name:   ch.Name,
			Failed: ch.Failed,
			Report: shieldReportUnitReportMapFromRun(ch.Report),
		})
	}

	return out
}

func ShieldReportDocumentMapFromRun(report ShieldRunReport, metrics ShieldRunMetrics) ShieldReportDocument {
	doc := ShieldReportDocument{
		SchemaVersion: shieldReportSchemaVersion,
		WrittenAt:     metrics.WallTime.Format("2006-01-02T15:04:05.999999999Z07:00"),
		RunFailed:     report.Failed,
		ElapsedNs:     report.Elapsed.Nanoseconds(),
		ElapsedHuman:  formatting.FormatDurationNSF64(float64(report.Elapsed.Nanoseconds())),
		Summary: ShieldReportSummary{
			UnitsVisited:           metrics.Aggregates.UnitsVisited,
			UnitsSkippedBlacklist:  metrics.Aggregates.UnitsSkippedBlacklist,
			UnitsSkippedSetup:      metrics.Aggregates.UnitsSkippedSetup,
			AtomValidationFailures: metrics.Aggregates.AtomValidationFailures,
			AtomPanics:             metrics.Aggregates.AtomPanics,
			AtomSetupFailures:      metrics.Aggregates.AtomSetupFailures,
			SubUnitLoopEarlyStops:  metrics.Aggregates.SubUnitLoopEarlyStops,
			UnitsFailedGate:        metrics.Aggregates.UnitsFailedGate,
		},
		Tree: make([]ShieldReportTopNode, 0, len(report.TopLevel)),
	}

	if metrics.AtomsTimedN > 0 {
		ds := &ShieldReportDurationSummary{
			AtomsTimed:  metrics.AtomsTimedN,
			MeanNs:      metrics.MeanNs,
			StddevPopNs: metrics.StddevPopNs,
			MinNs:       metrics.MinNs,
			MaxNs:       metrics.MaxNs,
		}

		if metrics.DurationStatsError != "" {
			ds.StatsError = metrics.DurationStatsError
		} else if metrics.DurationStatsOK {
			ds.SumNs = metrics.DurationSumNs
			ds.MedianNs = metrics.DurationMedianNs
			ds.Q1Ns = metrics.DurationQ1Ns
			ds.Q3Ns = metrics.DurationQ3Ns
			ds.IQRNs = metrics.DurationIQRNs
			ds.P95Ns = metrics.DurationP95Ns
			ds.P99Ns = metrics.DurationP99Ns
			if metrics.AtomsTimedN >= 2 && math.Abs(metrics.MeanNs) > shieldDurationMeanEpsilonNs {
				ds.CoeffVarPop = metrics.DurationCoeffVarPop
			}
			ds.MeanHuman = formatting.FormatDurationNSF64(metrics.MeanNs)
			ds.MinHuman = formatting.FormatDurationNSF64(metrics.MinNs)
			ds.MaxHuman = formatting.FormatDurationNSF64(metrics.MaxNs)
			ds.MedianHuman = formatting.FormatDurationNSF64(metrics.DurationMedianNs)
			ds.P95Human = formatting.FormatDurationNSF64(metrics.DurationP95Ns)
			ds.P99Human = formatting.FormatDurationNSF64(metrics.DurationP99Ns)
		}

		doc.Summary.Duration = ds
	}

	for _, tl := range report.TopLevel {
		doc.Tree = append(doc.Tree, ShieldReportTopNode{
			Name:   tl.Name,
			Failed: tl.Failed,
			Report: shieldReportUnitReportMapFromRun(tl.Report),
		})
	}

	return doc
}

func shieldReportFileStem(ts time.Time) string {
	return fmt.Sprintf("shield-run-%s.%09d",
		ts.Format("20060102-150405"),
		ts.Nanosecond())
}

func shieldRunReportWrite(persist *ShieldReportPersistence, report ShieldRunReport, metrics ShieldRunMetrics) string {
	if persist == nil || !persist.enabled {
		return ""
	}

	dir := filepath.Clean(persist.outputDir)
	if dir == "" || dir == "." {
		echo.On(shieldSystemID).Error("shield report persistence: output directory is empty")

		return ""
	}

	if err := os.MkdirAll(dir, 0o750); err != nil {
		echo.On(shieldSystemID).Error(fmt.Sprintf("shield report persistence: mkdir: %v", err))

		return ""
	}

	if persist.adapter != nil {
		path, err := persist.adapter(dir, report, metrics)
		if err != nil {
			echo.On(shieldSystemID).Error(fmt.Sprintf("shield report persistence: adapter: %v", err))

			return ""
		}

		return path
	}

	doc := ShieldReportDocumentMapFromRun(report, metrics)

	stem := shieldReportFileStem(metrics.WallTime)

	var primaryPath string

	switch persist.mode {
	case ShieldReportPersistenceModeJSON:
		path, err := shieldReportPersistWriteJSON(dir, stem, doc)
		if err != nil {
			echo.On(shieldSystemID).Error(fmt.Sprintf("shield report persistence: write json: %v", err))

			return ""
		}

		primaryPath = path

	case ShieldReportPersistenceModeTXT:
		path, err := shieldReportPersistWriteTXT(dir, stem, doc)
		if err != nil {
			echo.On(shieldSystemID).Error(fmt.Sprintf("shield report persistence: write txt: %v", err))

			return ""
		}

		primaryPath = path

	case ShieldReportPersistenceModeJSONAndTXT:
		pathJ, err := shieldReportPersistWriteJSON(dir, stem, doc)
		if err != nil {
			echo.On(shieldSystemID).Error(fmt.Sprintf("shield report persistence: write json: %v", err))

			return ""
		}

		primaryPath = pathJ

		pathT, err := shieldReportPersistWriteTXT(dir, stem, doc)
		if err != nil {
			echo.On(shieldSystemID).Error(fmt.Sprintf("shield report persistence: write txt: %v", err))

			return primaryPath
		}

		_ = pathT

	default:
		path, err := shieldReportPersistWriteJSON(dir, stem, doc)
		if err != nil {
			echo.On(shieldSystemID).Error(fmt.Sprintf("shield report persistence: write json: %v", err))

			return ""
		}

		primaryPath = path
	}

	return primaryPath
}

func shieldReportPersistWriteJSON(dir, stem string, doc ShieldReportDocument) (string, error) {
	base := stem + ".json"

	path, f, err := shieldReportOpenExclusive(dir, base)
	if err != nil {
		return "", err
	}

	defer f.Close()

	payload, err := shieldReportDocumentSerializeJSON(doc)
	if err != nil {
		return "", err
	}

	if _, err := f.Write(payload); err != nil {
		return "", err
	}

	return path, nil
}

func shieldReportPersistWriteTXT(dir, stem string, doc ShieldReportDocument) (string, error) {
	base := stem + ".txt"

	path, f, err := shieldReportOpenExclusive(dir, base)
	if err != nil {
		return "", err
	}

	defer f.Close()

	_, err = f.WriteString(shieldReportDocumentFormatTXT(doc))

	return path, err
}

func shieldReportOpenExclusive(dir, base string) (path string, f *os.File, err error) {
	for attempt := 0; attempt < 1000; attempt++ {
		name := base
		if attempt > 0 {
			ext := filepath.Ext(base)
			stem := strings.TrimSuffix(base, ext)
			name = fmt.Sprintf("%s_%d%s", stem, attempt, ext)
		}

		path = filepath.Join(dir, name)

		f, err = os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
		if err == nil {
			return path, f, nil
		}

		if !os.IsExist(err) {
			return "", nil, err
		}
	}

	return "", nil, fmt.Errorf("could not create unique file under %s", dir)
}

func shieldReportDocumentFormatTXT(doc ShieldReportDocument) string {
	var b strings.Builder

	shieldReportWriteTXTHeader(&b, doc)
	shieldReportWriteTXTVerdictBlock(&b, doc)
	shieldReportWriteTXTSummary(&b, doc.Summary)
	shieldReportWriteTXTDuration(&b, doc.Summary.Duration)
	shieldReportWriteTXTTree(&b, doc.Tree)

	return b.String()
}

func shieldReportWriteTXTHeader(b *strings.Builder, doc ShieldReportDocument) {
	runResult := "PASSED"
	if doc.RunFailed {
		runResult = "FAILED"
	}

	fmt.Fprintf(b, "Shield run report (schema %d)\n", doc.SchemaVersion)
	fmt.Fprintf(b, "Written: %s\n", doc.WrittenAt)
	fmt.Fprintf(b, "Result: %s\n", runResult)
	fmt.Fprintf(b, "Run failed: %v\n", doc.RunFailed)
	fmt.Fprintf(b, "Elapsed: %s (%d ns)\n\n", doc.ElapsedHuman, doc.ElapsedNs)
}

func shieldReportWriteTXTVerdictBlock(b *strings.Builder, doc ShieldReportDocument) {
	fmt.Fprintf(b, "[==========] Shield run complete (%s)\n", doc.ElapsedHuman)

	if doc.RunFailed {
		fmt.Fprintf(b, "[  FAILED  ] %d units failed.\n\n", doc.Summary.UnitsFailedGate)
		return
	}

	b.WriteString("[  PASSED  ] All units passed.\n\n")
}

func shieldReportWriteTXTSummary(b *strings.Builder, s ShieldReportSummary) {
	fmt.Fprintf(b, "Summary\n")
	fmt.Fprintf(b, "  Units visited: %d\n", s.UnitsVisited)
	fmt.Fprintf(b, "  Units skipped (blacklist): %d\n", s.UnitsSkippedBlacklist)
	fmt.Fprintf(b, "  Units skipped (setup): %d\n", s.UnitsSkippedSetup)
	fmt.Fprintf(b, "  Atom validation failures: %d\n", s.AtomValidationFailures)
	fmt.Fprintf(b, "  Atom panics: %d\n", s.AtomPanics)
	fmt.Fprintf(b, "  Atom setup failures: %d\n", s.AtomSetupFailures)
	fmt.Fprintf(b, "  Sub-unit loop early stops: %d\n", s.SubUnitLoopEarlyStops)
	fmt.Fprintf(b, "  Units failed (gate): %d\n", s.UnitsFailedGate)
}

func shieldReportWriteTXTDuration(b *strings.Builder, d *ShieldReportDurationSummary) {
	if d == nil {
		return
	}

	fmt.Fprintf(b, "\nAtom duration (timed atoms: %d)\n", d.AtomsTimed)
	if d.StatsError != "" {
		fmt.Fprintf(b, "  Stats error: %s\n", d.StatsError)
		return
	}

	shieldReportWriteTXTDurationStatsOK(b, d)
}

func shieldReportWriteTXTDurationStatsOK(b *strings.Builder, d *ShieldReportDurationSummary) {
	fmt.Fprintf(b, "  Sum: %.0f ns\n", d.SumNs)
	fmt.Fprintf(b, "  Mean: %s (%.0f ns)\n", d.MeanHuman, d.MeanNs)
	fmt.Fprintf(b, "  Stddev (pop): %.0f ns\n", d.StddevPopNs)
	fmt.Fprintf(b, "  Min: %s (%.0f ns)\n", d.MinHuman, d.MinNs)
	fmt.Fprintf(b, "  Q1: %.0f ns\n", d.Q1Ns)
	fmt.Fprintf(b, "  Median: %s (%.0f ns)\n", d.MedianHuman, d.MedianNs)
	fmt.Fprintf(b, "  Q3: %.0f ns\n", d.Q3Ns)
	fmt.Fprintf(b, "  IQR: %.0f ns\n", d.IQRNs)
	fmt.Fprintf(b, "  P95: %s (%.0f ns)\n", d.P95Human, d.P95Ns)
	fmt.Fprintf(b, "  P99: %s (%.0f ns)\n", d.P99Human, d.P99Ns)
	fmt.Fprintf(b, "  Max: %s (%.0f ns)\n", d.MaxHuman, d.MaxNs)
	if d.AtomsTimed >= 2 && math.Abs(d.MeanNs) > shieldDurationMeanEpsilonNs {
		fmt.Fprintf(b, "  Coeff var (pop): %g\n", d.CoeffVarPop)
	}
}

func shieldReportWriteTXTTree(b *strings.Builder, tree []ShieldReportTopNode) {
	b.WriteString("\nTop-level outcomes\n")
	for _, top := range tree {
		status := "PASSED"
		if top.Failed {
			status = "FAILED"
		}

		fmt.Fprintf(b, "  - %s: %s\n", top.Name, status)
	}

	b.WriteString("\nUnit tree (includes atom rows)\n")
	for _, top := range tree {
		shieldReportWriteTXTUnit(b, top.Name, top.Failed, top.Report, 0)
	}
}

func shieldReportWriteTXTUnit(b *strings.Builder, name string, failed bool, r ShieldReportUnitReport, depth int) {
	ind := strings.Repeat("  ", depth)
	status := "PASSED"
	if failed {
		status = "FAILED"
	}

	skipState := "none"
	if r.SkippedDueToBlacklist {
		skipState = "blacklist"
	} else if r.SkippedDueToSetup {
		skipState = "setup"
	}

	fmt.Fprintf(b, "%s- %s (status=%s)\n", ind, name, status)
	fmt.Fprintf(b, "%s  skip=%s atom_val_fail=%d atom_panic=%d atom_setup_fail=%d subunit_early=%v\n",
		ind, skipState, r.AtomValidationFailures, r.AtomPanics,
		r.AtomSetupFailureCount, r.TerminatedSubUnitLoopEarly)

	for _, atom := range r.Atoms {
		atomStatus := "PASSED"
		if atom.Failed {
			atomStatus = "FAILED"
		}

		atomSkip := "none"
		if atom.SkippedDueToBlacklist {
			atomSkip = "blacklist"
		} else if atom.SkippedDueToSetup {
			atomSkip = "setup"
		}

		fmt.Fprintf(b, "%s  * atom %s (status=%s)\n", ind, atom.Name, atomStatus)
		fmt.Fprintf(b, "%s    skip=%s val_fail=%d panic=%d elapsed=%s (%d ns)\n",
			ind, atomSkip, atom.ValidationFailures, atom.Panics, atom.ElapsedHuman, atom.ElapsedNs)
	}

	for _, ch := range r.DirectChildren {
		shieldReportWriteTXTUnit(b, ch.Name, ch.Failed, ch.Report, depth+1)
	}
}

func shieldReportDocumentSerializeJSON(doc ShieldReportDocument) ([]byte, error) {
	return json.MarshalIndent(doc, "", "  ")
}

func ShieldReportDocumentRead(path string) (ShieldReportDocument, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return ShieldReportDocument{}, err
	}

	var doc ShieldReportDocument
	if err := json.Unmarshal(payload, &doc); err != nil {
		return ShieldReportDocument{}, err
	}

	return doc, nil
}
