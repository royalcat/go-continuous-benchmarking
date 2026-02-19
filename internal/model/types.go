package model

import "fmt"

// BenchmarkResult represents a single benchmark measurement.
type BenchmarkResult struct {
	Name    string  `json:"name"`
	Value   float64 `json:"value"`
	Unit    string  `json:"unit"`
	Extra   string  `json:"extra,omitempty"`
	Package string  `json:"package,omitempty"`
	Procs   int     `json:"procs,omitempty"`
}

// Commit represents the git commit associated with a benchmark run.
type Commit struct {
	SHA     string `json:"sha"`
	Message string `json:"message"`
	Author  string `json:"author"`
	Date    string `json:"date"`
	URL     string `json:"url"`
}

// BenchmarkEntry represents a single benchmark run (one commit's results
// from a specific host/configuration).
type BenchmarkEntry struct {
	Commit     Commit            `json:"commit"`
	Date       int64             `json:"date"`
	CPU        string            `json:"cpu,omitempty"`
	GOOS       string            `json:"goos,omitempty"`
	GOARCH     string            `json:"goarch,omitempty"`
	CGO        bool              `json:"cgo"`
	Benchmarks []BenchmarkResult `json:"benchmarks"`
}

// EntryKey returns a composite key that uniquely identifies a benchmark run
// by its commit SHA, CPU model, and CGO status. Entries with the same key
// represent the same logical run and newer results should replace older ones.
func (e BenchmarkEntry) EntryKey() string {
	return fmt.Sprintf("%s|%s|%v", e.Commit.SHA, e.CPU, e.CGO)
}

// BranchData is a slice of benchmark entries for a given branch,
// ordered chronologically by commit date.
type BranchData []BenchmarkEntry
