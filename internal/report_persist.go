package internal

import (
	"encoding/json"
	"echo"
	"fmt"
	"foundation/formatting"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const shieldReportSchemaVersion = 1

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

type shieldReportDurationSummary struct {
	AtomsTimed      int     `json:"atoms_timed"`
	MeanNs          float64 `json:"mean_ns"`
	StddevPopNs     float64 `json:"stddev_pop_ns"`
	MinNs           float64 `json:"min_ns"`
	MaxNs           float64 `json:"max_ns"`
	MeanHuman       string  `json:"mean_human"`
	MinHuman        string  `json:"min_human"`
	MaxHuman        string  `json:"max_human"`
	StatsError      string  `json:"stats_error,omitempty"`
}

type shieldReportSummaryJSON struct {
	UnitsVisited            int                          `json:"units_visited"`
	UnitsSkippedBlacklist   int                          `json:"units_skipped_blacklist"`
	UnitsSkippedSetup       int                          `json:"units_skipped_setup"`
	AtomValidationFailures  int                          `json:"atom_validation_failures"`
	AtomPanics              int                          `json:"atom_panics"`
	AtomSetupFailures       int                          `json:"atom_setup_failures"`
	SubUnitLoopEarlyStops   int                          `json:"subunit_loop_early_stops"`
	UnitsFailedGate         int                          `json:"units_failed_gate"`
	Duration                *shieldReportDurationSummary `json:"duration,omitempty"`
}

type shieldReportUnitReportJSON struct {
	Name                       string                       `json:"name"`
	SkippedDueToBlacklist      bool                         `json:"skipped_due_to_blacklist"`
	SkippedDueToSetup        bool                         `json:"skipped_due_to_setup"`
	AtomSetupFailureCount      int                          `json:"atom_setup_failure_count"`
	AtomValidationFailures     int                          `json:"atom_validation_failures"`
	AtomPanics                 int                          `json:"atom_panics"`
	TerminatedSubUnitLoopEarly bool                         `json:"terminated_subunit_loop_early"`
	DirectChildren             []shieldReportChildNodeJSON  `json:"direct_children"`
}

type shieldReportChildNodeJSON struct {
	Name   string                       `json:"name"`
	Failed bool                         `json:"failed"`
	Report shieldReportUnitReportJSON   `json:"report"`
}

type shieldReportTopNodeJSON struct {
	Name   string                     `json:"name"`
	Failed bool                       `json:"failed"`
	Report shieldReportUnitReportJSON `json:"report"`
}

type shieldReportDocument struct {
	SchemaVersion int                       `json:"schema_version"`
	WrittenAt     string                    `json:"written_at"`
	RunFailed     bool                      `json:"run_failed"`
	ElapsedNs     int64                     `json:"elapsed_ns"`
	ElapsedHuman  string                    `json:"elapsed_human"`
	Summary       shieldReportSummaryJSON   `json:"summary"`
	Tree          []shieldReportTopNodeJSON `json:"tree"`
}

func shieldReportUnitReportToJSON(r ShieldUnitRunReport) shieldReportUnitReportJSON {
	out := shieldReportUnitReportJSON{
		Name:                       r.Name,
		SkippedDueToBlacklist:      r.SkippedDueToBlacklist,
		SkippedDueToSetup:        r.SkippedDueToSetup,
		AtomSetupFailureCount:      r.AtomSetupFailureCount,
		AtomValidationFailures:     r.AtomValidationFailures,
		AtomPanics:                 r.AtomPanics,
		TerminatedSubUnitLoopEarly: r.TerminatedSubUnitLoopEarly,
		DirectChildren:             make([]shieldReportChildNodeJSON, 0, len(r.DirectChildren)),
	}

	for _, ch := range r.DirectChildren {
		out.DirectChildren = append(out.DirectChildren, shieldReportChildNodeJSON{
			Name:   ch.Name,
			Failed: ch.Failed,
			Report: shieldReportUnitReportToJSON(ch.Report),
		})
	}

	return out
}

func shieldReportBuildDocument(report ShieldRunReport, metrics ShieldRunMetrics) shieldReportDocument {
	doc := shieldReportDocument{
		SchemaVersion: shieldReportSchemaVersion,
		WrittenAt:     metrics.WallTime.Format("2006-01-02T15:04:05.999999999Z07:00"),
		RunFailed:     report.Failed,
		ElapsedNs:     report.Elapsed.Nanoseconds(),
		ElapsedHuman:  formatting.FormatDurationNSF64(float64(report.Elapsed.Nanoseconds())),
		Summary: shieldReportSummaryJSON{
			UnitsVisited:            metrics.Aggregates.UnitsVisited,
			UnitsSkippedBlacklist:   metrics.Aggregates.UnitsSkippedBlacklist,
			UnitsSkippedSetup:       metrics.Aggregates.UnitsSkippedSetup,
			AtomValidationFailures:  metrics.Aggregates.AtomValidationFailures,
			AtomPanics:              metrics.Aggregates.AtomPanics,
			AtomSetupFailures:       metrics.Aggregates.AtomSetupFailures,
			SubUnitLoopEarlyStops:   metrics.Aggregates.SubUnitLoopEarlyStops,
			UnitsFailedGate:         metrics.Aggregates.UnitsFailedGate,
		},
		Tree: make([]shieldReportTopNodeJSON, 0, len(report.TopLevel)),
	}

	if metrics.AtomsTimedN > 0 {
		ds := &shieldReportDurationSummary{
			AtomsTimed:  metrics.AtomsTimedN,
			MeanNs:      metrics.MeanNs,
			StddevPopNs: metrics.StddevPopNs,
			MinNs:       metrics.MinNs,
			MaxNs:       metrics.MaxNs,
		}

		if metrics.DurationStatsError != "" {
			ds.StatsError = metrics.DurationStatsError
		} else if metrics.DurationStatsOK {
			ds.MeanHuman = formatting.FormatDurationNSF64(metrics.MeanNs)
			ds.MinHuman = formatting.FormatDurationNSF64(metrics.MinNs)
			ds.MaxHuman = formatting.FormatDurationNSF64(metrics.MaxNs)
		}

		doc.Summary.Duration = ds
	}

	for _, tl := range report.TopLevel {
		doc.Tree = append(doc.Tree, shieldReportTopNodeJSON{
			Name:   tl.Name,
			Failed: tl.Failed,
			Report: shieldReportUnitReportToJSON(tl.Report),
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

	doc := shieldReportBuildDocument(report, metrics)

	stem := shieldReportFileStem(metrics.WallTime)

	var primaryPath string

	switch persist.mode {
	case ShieldReportPersistenceModeJSON:
		path, err := shieldReportWriteJSON(dir, stem, doc)
		if err != nil {
			echo.On(shieldSystemID).Error(fmt.Sprintf("shield report persistence: write json: %v", err))

			return ""
		}

		primaryPath = path

	case ShieldReportPersistenceModeTXT:
		path, err := shieldReportWriteTXT(dir, stem, doc)
		if err != nil {
			echo.On(shieldSystemID).Error(fmt.Sprintf("shield report persistence: write txt: %v", err))

			return ""
		}

		primaryPath = path

	case ShieldReportPersistenceModeJSONAndTXT:
		pathJ, err := shieldReportWriteJSON(dir, stem, doc)
		if err != nil {
			echo.On(shieldSystemID).Error(fmt.Sprintf("shield report persistence: write json: %v", err))

			return ""
		}

		primaryPath = pathJ

		pathT, err := shieldReportWriteTXT(dir, stem, doc)
		if err != nil {
			echo.On(shieldSystemID).Error(fmt.Sprintf("shield report persistence: write txt: %v", err))

			return primaryPath
		}

		_ = pathT

	default:
		path, err := shieldReportWriteJSON(dir, stem, doc)
		if err != nil {
			echo.On(shieldSystemID).Error(fmt.Sprintf("shield report persistence: write json: %v", err))

			return ""
		}

		primaryPath = path
	}

	return primaryPath
}

func shieldReportWriteJSON(dir, stem string, doc shieldReportDocument) (string, error) {
	base := stem + ".json"

	path, f, err := shieldReportOpenExclusive(dir, base)
	if err != nil {
		return "", err
	}

	defer f.Close()

	payload, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", err
	}

	if _, err := f.Write(payload); err != nil {
		return "", err
	}

	return path, nil
}

func shieldReportWriteTXT(dir, stem string, doc shieldReportDocument) (string, error) {
	base := stem + ".txt"

	path, f, err := shieldReportOpenExclusive(dir, base)
	if err != nil {
		return "", err
	}

	defer f.Close()

	_, err = f.WriteString(shieldReportFormatTXT(doc))

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

func shieldReportFormatTXT(doc shieldReportDocument) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Shield run report (schema %d)\n", doc.SchemaVersion)
	fmt.Fprintf(&b, "Written: %s\n", doc.WrittenAt)
	fmt.Fprintf(&b, "Run failed: %v\n", doc.RunFailed)
	fmt.Fprintf(&b, "Elapsed: %s (%d ns)\n\n", doc.ElapsedHuman, doc.ElapsedNs)

	s := doc.Summary
	fmt.Fprintf(&b, "Summary\n")
	fmt.Fprintf(&b, "  Units visited: %d\n", s.UnitsVisited)
	fmt.Fprintf(&b, "  Units skipped (blacklist): %d\n", s.UnitsSkippedBlacklist)
	fmt.Fprintf(&b, "  Units skipped (setup): %d\n", s.UnitsSkippedSetup)
	fmt.Fprintf(&b, "  Atom validation failures: %d\n", s.AtomValidationFailures)
	fmt.Fprintf(&b, "  Atom panics: %d\n", s.AtomPanics)
	fmt.Fprintf(&b, "  Atom setup failures: %d\n", s.AtomSetupFailures)
	fmt.Fprintf(&b, "  Sub-unit loop early stops: %d\n", s.SubUnitLoopEarlyStops)
	fmt.Fprintf(&b, "  Units failed (gate): %d\n", s.UnitsFailedGate)

	if s.Duration != nil {
		fmt.Fprintf(&b, "\nAtom duration (timed atoms: %d)\n", s.Duration.AtomsTimed)
		if s.Duration.StatsError != "" {
			fmt.Fprintf(&b, "  Stats error: %s\n", s.Duration.StatsError)
		} else {
			fmt.Fprintf(&b, "  Mean: %s (%.0f ns)\n", s.Duration.MeanHuman, s.Duration.MeanNs)
			fmt.Fprintf(&b, "  Stddev (pop): %.0f ns\n", s.Duration.StddevPopNs)
			fmt.Fprintf(&b, "  Min: %s (%.0f ns)\n", s.Duration.MinHuman, s.Duration.MinNs)
			fmt.Fprintf(&b, "  Max: %s (%.0f ns)\n", s.Duration.MaxHuman, s.Duration.MaxNs)
		}
	}

	b.WriteString("\nUnit tree (aggregates only; per-atom rows not in v1)\n")
	for _, top := range doc.Tree {
		shieldReportWriteTXTUnit(&b, top.Name, top.Failed, top.Report, 0)
	}

	return b.String()
}

func shieldReportWriteTXTUnit(b *strings.Builder, name string, failed bool, r shieldReportUnitReportJSON, depth int) {
	ind := strings.Repeat("  ", depth)

	fmt.Fprintf(b, "%s- %s (failed=%v)\n", ind, name, failed)
	fmt.Fprintf(b, "%s  blacklist=%v setup_skip=%v atom_val_fail=%d atom_panic=%d atom_setup_fail=%d subunit_early=%v\n",
		ind, r.SkippedDueToBlacklist, r.SkippedDueToSetup, r.AtomValidationFailures, r.AtomPanics,
		r.AtomSetupFailureCount, r.TerminatedSubUnitLoopEarly)

	for _, ch := range r.DirectChildren {
		shieldReportWriteTXTUnit(b, ch.Name, ch.Failed, ch.Report, depth+1)
	}
}
