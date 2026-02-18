package model

// BenchmarkResult represents a single benchmark measurement.
type BenchmarkResult struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
	Extra string  `json:"extra,omitempty"`
}

// Commit represents the git commit associated with a benchmark run.
type Commit struct {
	SHA     string `json:"sha"`
	Message string `json:"message"`
	Author  string `json:"author"`
	Date    string `json:"date"`
	URL     string `json:"url"`
}

// BenchmarkEntry represents a single benchmark run (one commit's results).
type BenchmarkEntry struct {
	Commit     Commit            `json:"commit"`
	Date       int64             `json:"date"`
	Benchmarks []BenchmarkResult `json:"benchmarks"`
}

// BranchData is a slice of benchmark entries for a given branch,
// ordered chronologically by commit date.
type BranchData []BenchmarkEntry
