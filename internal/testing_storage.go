package internal

import (
	"database/sql"
	"essence"
	"fmt"
	"persistence"
	"time"
)

const zonePathStorageSeparator = "."

const (
	testResultsTableName  = "test_results"
	guardResultsTableName = "guard_results"

	// Scenario Columns
	colScenarioID       = "result_id"
	colScenarioName     = "scenario_name"
	colScenarioZonePath = "zone_path"
	colTimestamp        = "timestamp"
	colPassed           = "passed"
	colDurationWall     = "duration_wall"
	colDurationSummed   = "duration_summed"
	colSeed             = "seed"
	colFuzzingPattern   = "fuzzing_pattern"
	colMaxIterations    = "max_iterations"
	colMaxDuration      = "max_duration"
	colUseDuration      = "use_duration"
	colProviderID       = "provider_id"

	// Guard Columns
	colGuardID              = "guard_result_id"
	colGuardTestID          = "test_result_id"
	colGuardName            = "guard_name"
	colGuardPassed          = "passed"
	colGuardDuration        = "duration"
	colGuardFailedSeed      = "failed_seed"
	colGuardFailedIteration = "failed_iteration"
	colGuardFailureReason   = "failure_reason"
)

// --------------------------------------------------------------- ENTITIES

type testResultEntity struct {
	ResultID         string
	ScenarioName     string
	ScenarioZonePath string
	Timestamp        int64 // Unix milliseconds
	Passed           int   // 0 or 1
	DurationWall     int64 // Nanoseconds
	DurationSummed   int64 // Nanoseconds
	Seed             string
	FuzzingPattern   int
	MaxIterations    int64
	MaxDuration      int64 // Nanoseconds
	UseDuration      int   // 0 or 1
	ProviderID       string
}

type guardResultEntity struct {
	GuardResultID   string
	TestResultID    string
	GuardName       string
	Passed          int   // 0 or 1
	Duration        int64 // Nanoseconds
	FailedSeed      string
	FailedIteration int64
	FailureReason   string
}

// --------------------------------------------------------------- DATABASE

type TestResultDatabase struct {
	engine *persistence.SQLite3Repo
}

func TestResultDatabaseCreate(dbFilePath string) *TestResultDatabase {
	repoConfig := persistence.SQLite3RepoConfigurationCreate(dbFilePath)

	testResultsTable := createTestResultsTableDef()
	guardResultsTable := createGuardResultsTableDef()

	repoConfig.WithTables(
		testResultsTable.ToAnyTable(),
		guardResultsTable.ToAnyTable(),
	)

	repoEngine := persistence.SQLite3RepoCreate(repoConfig)

	return &TestResultDatabase{
		engine: repoEngine,
	}
}

func TestResultDatabaseInitialize(db *TestResultDatabase) error {
	if persistence.SQLite3RepoIsOpen(db.engine) {
		return nil
	}

	if err := persistence.SQLite3RepoInitialize(db.engine); err != nil {
		return fmt.Errorf("error initializing connection: %w", err)
	}

	return nil
}

func TestResultDatabaseClose(db *TestResultDatabase) error {
	if !persistence.SQLite3RepoIsOpen(db.engine) {
		return nil
	}

	if err := persistence.SQLite3RepoClose(db.engine); err != nil {
		return fmt.Errorf("error closing connection: %w", err)
	}

	return nil
}

func testResultDatabaseEnsureOpen(db *TestResultDatabase) error {
	if err := TestResultDatabaseInitialize(db); err != nil {
		return fmt.Errorf("error ensuring connection open: %w", err)
	}

	return nil
}

func createTestResultsTableDef() persistence.SQLite3TableConfiguration[testResultEntity] {
	return persistence.SQLite3TableConfigurationCreate(
		testResultsTableName,
		persistence.SQLite3SchemaFieldCreateManual(
			colScenarioID, persistence.SQLiteDataTypeText,
			func(t *testResultEntity) any { return t.ResultID },
			func(t *testResultEntity) any { return &t.ResultID },
			persistence.SQLite3SchemaFieldOptionsPrimaryKey(),
		),
		persistence.SQLite3SchemaFieldCreateManual(
			colScenarioName, persistence.SQLiteDataTypeText,
			func(t *testResultEntity) any { return t.ScenarioName },
			func(t *testResultEntity) any { return &t.ScenarioName },
			persistence.SQLite3SchemaFieldOptionsFilterable(),
		),
		persistence.SQLite3SchemaFieldCreateManual(
			colScenarioZonePath, persistence.SQLiteDataTypeText,
			func(t *testResultEntity) any { return t.ScenarioZonePath },
			func(t *testResultEntity) any { return &t.ScenarioZonePath },
			persistence.SQLite3SchemaFieldOptionsFilterable(),
		),
		persistence.SQLite3SchemaFieldCreateManual(
			colTimestamp, persistence.SQLiteDataTypeInteger,
			func(t *testResultEntity) any { return t.Timestamp },
			func(t *testResultEntity) any { return &t.Timestamp },
			persistence.SQLite3SchemaFieldOptionsFilterable(),
		),
		persistence.SQLite3SchemaFieldCreateManual(
			colPassed, persistence.SQLiteDataTypeInteger,
			func(t *testResultEntity) any { return t.Passed },
			func(t *testResultEntity) any { return &t.Passed },
			persistence.SQLite3SchemaFieldOptionsFilterable(),
		),
		persistence.SQLite3SchemaFieldCreateManual(
			colDurationWall, persistence.SQLiteDataTypeInteger,
			func(t *testResultEntity) any { return t.DurationWall },
			func(t *testResultEntity) any { return &t.DurationWall },
			persistence.SQLite3SchemaFieldOptions{},
		),
		persistence.SQLite3SchemaFieldCreateManual(
			colDurationSummed, persistence.SQLiteDataTypeInteger,
			func(t *testResultEntity) any { return t.DurationSummed },
			func(t *testResultEntity) any { return &t.DurationSummed },
			persistence.SQLite3SchemaFieldOptions{},
		),
		persistence.SQLite3SchemaFieldCreateManual(
			colSeed, persistence.SQLiteDataTypeText,
			func(t *testResultEntity) any { return t.Seed },
			func(t *testResultEntity) any { return &t.Seed },
			persistence.SQLite3SchemaFieldOptions{},
		),
		persistence.SQLite3SchemaFieldCreateManual(
			colFuzzingPattern, persistence.SQLiteDataTypeInteger,
			func(t *testResultEntity) any { return t.FuzzingPattern },
			func(t *testResultEntity) any { return &t.FuzzingPattern },
			persistence.SQLite3SchemaFieldOptions{},
		),
		persistence.SQLite3SchemaFieldCreateManual(
			colMaxIterations, persistence.SQLiteDataTypeInteger,
			func(t *testResultEntity) any { return t.MaxIterations },
			func(t *testResultEntity) any { return &t.MaxIterations },
			persistence.SQLite3SchemaFieldOptions{},
		),
		persistence.SQLite3SchemaFieldCreateManual(
			colMaxDuration, persistence.SQLiteDataTypeInteger,
			func(t *testResultEntity) any { return t.MaxDuration },
			func(t *testResultEntity) any { return &t.MaxDuration },
			persistence.SQLite3SchemaFieldOptions{},
		),
		persistence.SQLite3SchemaFieldCreateManual(
			colUseDuration, persistence.SQLiteDataTypeInteger,
			func(t *testResultEntity) any { return t.UseDuration },
			func(t *testResultEntity) any { return &t.UseDuration },
			persistence.SQLite3SchemaFieldOptions{},
		),
		persistence.SQLite3SchemaFieldCreateManual(
			colProviderID, persistence.SQLiteDataTypeText,
			func(t *testResultEntity) any { return t.ProviderID },
			func(t *testResultEntity) any { return &t.ProviderID },
			persistence.SQLite3SchemaFieldOptions{},
		),
	)
}

func createGuardResultsTableDef() persistence.SQLite3TableConfiguration[guardResultEntity] {
	return persistence.SQLite3TableConfigurationCreate(
		guardResultsTableName,
		persistence.SQLite3SchemaFieldCreateManual(
			colGuardID, persistence.SQLiteDataTypeText,
			func(t *guardResultEntity) any { return t.GuardResultID },
			func(t *guardResultEntity) any { return &t.GuardResultID },
			persistence.SQLite3SchemaFieldOptionsPrimaryKey(),
		),
		persistence.SQLite3SchemaFieldCreateManual(
			colGuardTestID, persistence.SQLiteDataTypeText,
			func(t *guardResultEntity) any { return t.TestResultID },
			func(t *guardResultEntity) any { return &t.TestResultID },
			persistence.SQLite3SchemaFieldOptionsBuilderCreate().
				WithFilterable(true).
				WithIndexable(true).
				WithForeignKey(&persistence.SQLite3ForeignKey{
					Table:    testResultsTableName,
					Column:   colScenarioID,
					OnDelete: "CASCADE",
				}).Build(),
		),
		persistence.SQLite3SchemaFieldCreateManual(
			colGuardName, persistence.SQLiteDataTypeText,
			func(t *guardResultEntity) any { return t.GuardName },
			func(t *guardResultEntity) any { return &t.GuardName },
			persistence.SQLite3SchemaFieldOptionsFilterable(),
		),
		persistence.SQLite3SchemaFieldCreateManual(
			colGuardPassed, persistence.SQLiteDataTypeInteger,
			func(t *guardResultEntity) any { return t.Passed },
			func(t *guardResultEntity) any { return &t.Passed },
			persistence.SQLite3SchemaFieldOptionsFilterable(),
		),
		persistence.SQLite3SchemaFieldCreateManual(
			colGuardDuration, persistence.SQLiteDataTypeInteger,
			func(t *guardResultEntity) any { return t.Duration },
			func(t *guardResultEntity) any { return &t.Duration },
			persistence.SQLite3SchemaFieldOptions{},
		),
		persistence.SQLite3SchemaFieldCreateManual(
			colGuardFailedSeed, persistence.SQLiteDataTypeText,
			func(t *guardResultEntity) any { return t.FailedSeed },
			func(t *guardResultEntity) any { return &t.FailedSeed },
			persistence.SQLite3SchemaFieldOptions{},
		),
		persistence.SQLite3SchemaFieldCreateManual(
			colGuardFailedIteration, persistence.SQLiteDataTypeInteger,
			func(t *guardResultEntity) any { return t.FailedIteration },
			func(t *guardResultEntity) any { return &t.FailedIteration },
			persistence.SQLite3SchemaFieldOptions{},
		),
		persistence.SQLite3SchemaFieldCreateManual(
			colGuardFailureReason, persistence.SQLiteDataTypeText,
			func(t *guardResultEntity) any { return t.FailureReason },
			func(t *guardResultEntity) any { return &t.FailureReason },
			persistence.SQLite3SchemaFieldOptions{},
		),
	)
}

// --------------------------------------------------------------- OPERATIONS

func TestResultDatabaseScenarioResultAdd(db *TestResultDatabase, scenarioResult ScenarioRunResult) error {
	if err := testResultDatabaseEnsureOpen(db); err != nil {
		return err
	}

	scenarioID, _ := essence.UUIDv7GenerateRandom()

	scenarioEntity := mapScenarioToEntity(scenarioResult, scenarioID.String())
	guardEntities := mapGuardsToEntities(scenarioID.String(), scenarioResult.GuardResults())

	guardPointers := make([]*guardResultEntity, len(guardEntities))
	for i := range guardEntities {
		guardPointers[i] = &guardEntities[i]
	}

	return persistence.SQLite3RepoTransaction(db.engine, func(tx *sql.Tx) error {
		if err := persistence.SQLite3RepoInsertTx(tx, db.engine, testResultsTableName, &scenarioEntity); err != nil {
			return err
		}

		if err := persistence.SQLite3RepoBatchInsertTx(tx, db.engine, guardResultsTableName, guardPointers); err != nil {
			return err
		}

		return nil
	})
}

func TestResultDatabaseScenarioResultFindByID(db *TestResultDatabase, id string) (*ScenarioRunResult, error) {
	if err := testResultDatabaseEnsureOpen(db); err != nil {
		return nil, err
	}

	scenarioEntity, err := persistence.SQLite3RepoGet[testResultEntity](db.engine, testResultsTableName, id)
	if err != nil {
		return nil, fmt.Errorf("scenario %s not found: %w", id, err)
	}

	guardEntities, err := persistence.SQLite3RepoFindByField[guardResultEntity](
		db.engine, guardResultsTableName, colGuardTestID, id,
	)
	if err != nil {
		return nil, fmt.Errorf("failed fetching guards for scenario %s: %w", id, err)
	}

	return mapEntityToScenario(scenarioEntity, guardEntities), nil
}

func TestResultDatabaseScenarioResultFindByName(db *TestResultDatabase, name string) ([]*ScenarioRunResult, error) {
	if err := testResultDatabaseEnsureOpen(db); err != nil {
		return nil, err
	}

	scenarioEntities, err := persistence.SQLite3RepoFindByField[testResultEntity](db.engine, testResultsTableName, colScenarioName, name)
	if err != nil {
		return nil, fmt.Errorf("issue getting scenario results: %w", err)
	}

	results := make([]*ScenarioRunResult, len(scenarioEntities))
	for i, scenarioEntity := range scenarioEntities {
		guardEntities, guardErr := persistence.SQLite3RepoFindByField[guardResultEntity](
			db.engine, guardResultsTableName, colGuardTestID, scenarioEntity.ResultID,
		)
		if guardErr != nil {
			return nil, fmt.Errorf("failed fetching guards for scenario %s: %w", scenarioEntity.ResultID, guardErr)
		}

		results[i] = mapEntityToScenario(scenarioEntity, guardEntities)
	}

	return results, nil
}

// --------------------------------------------------------------- PRIVATE HELPERS

func mapEntityToScenario(scenario *testResultEntity, guards []*guardResultEntity) *ScenarioRunResult {
	seedUUID, _ := essence.UUIDFromString(scenario.Seed)

	domainGuards := make([]GuardEvaluationResult, len(guards))
	for i, g := range guards {
		failedSeedUUID, _ := essence.UUIDFromString(g.FailedSeed)

		domainGuards[i] = GuardEvaluationResult{
			guardName:       g.GuardName,
			passed:          g.Passed == 1,
			duration:        time.Duration(g.Duration),
			failedSeed:      failedSeedUUID,
			failedIteration: uint64(g.FailedIteration),
			failureReason:   g.FailureReason,
		}
	}

	return &ScenarioRunResult{
		scenarioName:        scenario.ScenarioName,
		zonePath:            ZonePathCreateFromString(scenario.ScenarioZonePath, zonePathStorageSeparator),
		totalDurationWall:   time.Duration(scenario.DurationWall),
		totalDurationSummed: time.Duration(scenario.DurationSummed),
		passed:              scenario.Passed == 1,
		guardResults:        domainGuards,
		runConfig: SnapshotConfig{
			Seed:           seedUUID,
			FuzzingPattern: FuzzingPattern(scenario.FuzzingPattern),
			MaxIterations:  uint64(scenario.MaxIterations),
			MaxDuration:    time.Duration(scenario.MaxDuration),
			UseDuration:    scenario.UseDuration == 1,
			ProviderID:     scenario.ProviderID,
		},
	}
}

func mapScenarioToEntity(scenario ScenarioRunResult, scenarioID string) testResultEntity {
	cfg := scenario.SnapshotConfig()

	passedInt, useDurationInt := 0, 0
	if scenario.Passed() {
		passedInt = 1
	}
	if cfg.UseDuration {
		useDurationInt = 1
	}

	return testResultEntity{
		ResultID:         scenarioID,
		ScenarioName:     scenario.Name(),
		ScenarioZonePath: scenario.zonePath.Render(zonePathStorageSeparator),
		Timestamp:        time.Now().UnixMilli(),
		Passed:           passedInt,
		DurationWall:     scenario.WallDuration().Nanoseconds(),
		DurationSummed:   scenario.SummedDuration().Nanoseconds(),
		Seed:             cfg.Seed.String(),
		FuzzingPattern:   int(cfg.FuzzingPattern),
		MaxIterations:    int64(cfg.MaxIterations),
		MaxDuration:      cfg.MaxDuration.Nanoseconds(),
		UseDuration:      useDurationInt,
		ProviderID:       cfg.ProviderID,
	}
}

func mapGuardsToEntities(scenarioID string, guards []GuardEvaluationResult) []guardResultEntity {
	entities := make([]guardResultEntity, len(guards))

	for i, g := range guards {
		guardID, _ := essence.UUIDv7GenerateRandom()

		passedInt := 0
		if g.Passed() {
			passedInt = 1
		}

		entities[i] = guardResultEntity{
			GuardResultID:   guardID.String(),
			TestResultID:    scenarioID,
			GuardName:       g.Name(),
			Passed:          passedInt,
			Duration:        g.Duration().Nanoseconds(),
			FailedSeed:      g.failedSeed.String(),
			FailedIteration: int64(g.failedIteration),
			FailureReason:   g.FailureReason(),
		}
	}

	return entities
}
