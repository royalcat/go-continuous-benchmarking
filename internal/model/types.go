package model

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

// RunParams holds the environment and configuration parameters that uniquely
// identify a benchmark run configuration. Two runs with the same commit and
// the same RunParams are considered the same logical run — newer results
// replace older ones.
type RunParams struct {
	CPU       string `json:"cpu,omitempty"`
	GOOS      string `json:"goos,omitempty"`
	GOARCH    string `json:"goarch,omitempty"`
	GoVersion string `json:"goVersion,omitempty"`
	CGO       bool   `json:"cgo"`
}

// BenchmarkEntry represents a single benchmark run (one commit's results
// from a specific host/configuration).
type BenchmarkEntry struct {
	Commit     Commit            `json:"commit"`
	Date       int64             `json:"date"`
	Params     RunParams         `json:"params"`
	Benchmarks []BenchmarkResult `json:"benchmarks"`
}

// EntryKey returns a composite key that uniquely identifies a benchmark run
// by its commit SHA and all run parameters. Entries with the same key
// represent the same logical run and newer results should replace older ones.
//
// RunParams is a simple comparable struct (no slices, maps, or pointers),
// so we use it directly as part of the map key.
func (e BenchmarkEntry) EntryKey() EntryKeyValue {
	return EntryKeyValue{
		SHA:    e.Commit.SHA,
		Params: e.Params,
	}
}

// EntryKeyValue is the composite key type used for deduplication.
// It is comparable and can be used as a map key directly.
type EntryKeyValue struct {
	SHA    string
	Params RunParams
}

// BranchData is a slice of benchmark entries for a given branch,
// ordered chronologically by commit date.
type BranchData []BenchmarkEntry
