package main

import (
	"bufio"
	"fmt"
	"foundation/system"
	"os"
	"path/filepath"
	"shield"
	"sort"
	"strings"
	"time"
)

const (
	shieldCliResultPrefix      = "shield-run-"
	shieldCliBaselineFileName  = ".shield-baseline"
	shieldCliDefaultResultsDir = "results/tests"
)

type shieldCliResultFileMeta struct {
	Path      string
	Name      string
	WrittenAt string
	RunFailed *bool
	ElapsedNs int64
	ModTime   time.Time
}

func shieldCliResultDirResolve(defaultDir string) string {
	env := strings.TrimSpace(os.Getenv("SHIELD_RESULTS_DIR"))
	if env != "" {
		return env
	}

	return defaultDir
}

func shieldCliResultDirResolveFromBase(baseDir string, defaultDir string) (string, error) {
	rawDir := shieldCliResultDirResolve(defaultDir)
	if filepath.IsAbs(rawDir) {
		return filepath.Clean(rawDir), nil
	}

	if strings.TrimSpace(baseDir) != "" {
		return filepath.Clean(system.PathJoin(baseDir, rawDir)), nil
	}

	return system.PathResolveWorkspace(rawDir)
}

func shieldCliResultFilesList(resultsDir string) ([]shieldCliResultFileMeta, error) {
	entries, err := os.ReadDir(resultsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []shieldCliResultFileMeta{}, nil
		}
		return nil, err
	}

	items := make([]shieldCliResultFileMeta, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasPrefix(name, shieldCliResultPrefix) {
			continue
		}

		ext := filepath.Ext(name)
		if ext != ".json" && ext != ".txt" {
			continue
		}

		path := filepath.Join(resultsDir, name)
		info, err := entry.Info()
		if err != nil {
			continue
		}

		meta := shieldCliResultFileMeta{
			Path:    path,
			Name:    name,
			ModTime: info.ModTime(),
		}

		if ext == ".json" {
			doc, err := shieldCliResultFileReadMeta(path)
			if err == nil {
				meta.WrittenAt = doc.WrittenAt
				meta.ElapsedNs = doc.ElapsedNs
				runFailed := doc.RunFailed
				meta.RunFailed = &runFailed
			}
		}

		items = append(items, meta)
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].ModTime.After(items[j].ModTime)
	})

	return items, nil
}

func shieldCliResultFileReadMeta(path string) (shield.ReportDocument, error) {
	return shield.ReportDocumentRead(path)
}

func shieldCliResultFileSelect(resultsDir, selector string) (shieldCliResultFileMeta, error) {
	results, err := shieldCliResultFilesList(resultsDir)
	if err != nil {
		return shieldCliResultFileMeta{}, err
	}
	if len(results) == 0 {
		return shieldCliResultFileMeta{}, fmt.Errorf("no results found in %s", resultsDir)
	}

	if selector == "latest" {
		return results[0], nil
	}

	if index, ok := shieldCliIndexParse(selector); ok {
		if index > len(results) {
			return shieldCliResultFileMeta{}, fmt.Errorf("index out of range")
		}
		return results[index-1], nil
	}

	path := selector
	if !filepath.IsAbs(path) {
		path = filepath.Join(resultsDir, selector)
	}

	info, err := os.Stat(path)
	if err != nil {
		return shieldCliResultFileMeta{}, err
	}
	if info.IsDir() {
		return shieldCliResultFileMeta{}, fmt.Errorf("selector points to directory: %s", path)
	}

	return shieldCliResultFileMeta{
		Path:    path,
		Name:    filepath.Base(path),
		ModTime: info.ModTime(),
	}, nil
}

func shieldCliResultFilesDeleteAll(resultsDir string) (int, error) {
	results, err := shieldCliResultFilesList(resultsDir)
	if err != nil {
		return 0, err
	}

	removed := 0
	for _, result := range results {
		if err := os.Remove(result.Path); err == nil {
			removed++
		}
	}

	_ = shieldCliBaselineClear(resultsDir)

	return removed, nil
}

func shieldCliDeleteConfirm(path string) bool {
	fmt.Printf("delete '%s'? [y/N]: ", path)

	reader := bufio.NewReader(os.Stdin)
	response, _ := reader.ReadString('\n')

	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
}

func shieldCliBaselineFilePath(resultsDir string) string {
	return filepath.Join(resultsDir, shieldCliBaselineFileName)
}

func shieldCliBaselineRead(resultsDir string) (string, error) {
	payload, err := os.ReadFile(shieldCliBaselineFilePath(resultsDir))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}

		return "", err
	}

	return strings.TrimSpace(string(payload)), nil
}

func shieldCliBaselineWrite(resultsDir, path string) error {
	if err := os.MkdirAll(resultsDir, 0o750); err != nil {
		return err
	}

	return os.WriteFile(shieldCliBaselineFilePath(resultsDir), []byte(path+"\n"), 0o640)
}

func shieldCliBaselineClear(resultsDir string) error {
	err := os.Remove(shieldCliBaselineFilePath(resultsDir))
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}
