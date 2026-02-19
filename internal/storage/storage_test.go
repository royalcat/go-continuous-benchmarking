package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/royalcat/go-continuous-benchmarking/internal/model"
)

func TestNew_CreatesDirectories(t *testing.T) {
	dir := t.TempDir()
	baseDir := filepath.Join(dir, "bench")

	s, err := New(baseDir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if s == nil {
		t.Fatal("New() returned nil storage")
	}

	// Check data/ subdirectory was created
	dataDir := filepath.Join(baseDir, "data")
	info, err := os.Stat(dataDir)
	if err != nil {
		t.Fatalf("data directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("data path is not a directory")
	}
}

func TestReadBranches_EmptyWhenNoFile(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	branches, err := s.ReadBranches()
	if err != nil {
		t.Fatalf("ReadBranches() error: %v", err)
	}
	if branches != nil {
		t.Fatalf("expected nil, got %v", branches)
	}
}

func TestWriteAndReadBranches(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	want := []string{"develop", "feature/foo", "main"}
	if err := s.WriteBranches(want); err != nil {
		t.Fatalf("WriteBranches() error: %v", err)
	}

	got, err := s.ReadBranches()
	if err != nil {
		t.Fatalf("ReadBranches() error: %v", err)
	}

	if len(got) != len(want) {
		t.Fatalf("branch count: got %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("branch[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestEnsureBranch_AddsNewBranch(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	added, err := s.EnsureBranch("main")
	if err != nil {
		t.Fatalf("EnsureBranch() error: %v", err)
	}
	if !added {
		t.Error("expected branch to be newly added")
	}

	branches, err := s.ReadBranches()
	if err != nil {
		t.Fatalf("ReadBranches() error: %v", err)
	}
	if len(branches) != 1 || branches[0] != "main" {
		t.Errorf("unexpected branches: %v", branches)
	}
}

func TestEnsureBranch_DoesNotDuplicate(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if _, err := s.EnsureBranch("main"); err != nil {
		t.Fatalf("first EnsureBranch() error: %v", err)
	}

	added, err := s.EnsureBranch("main")
	if err != nil {
		t.Fatalf("second EnsureBranch() error: %v", err)
	}
	if added {
		t.Error("expected branch to NOT be newly added on second call")
	}

	branches, err := s.ReadBranches()
	if err != nil {
		t.Fatalf("ReadBranches() error: %v", err)
	}
	if len(branches) != 1 {
		t.Errorf("expected 1 branch, got %d: %v", len(branches), branches)
	}
}

func TestEnsureBranch_SortedOrder(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	for _, b := range []string{"zebra", "alpha", "middle"} {
		if _, err := s.EnsureBranch(b); err != nil {
			t.Fatalf("EnsureBranch(%q) error: %v", b, err)
		}
	}

	branches, err := s.ReadBranches()
	if err != nil {
		t.Fatalf("ReadBranches() error: %v", err)
	}

	expected := []string{"alpha", "middle", "zebra"}
	if len(branches) != len(expected) {
		t.Fatalf("branch count: got %d, want %d", len(branches), len(expected))
	}
	for i := range expected {
		if branches[i] != expected[i] {
			t.Errorf("branch[%d]: got %q, want %q", i, branches[i], expected[i])
		}
	}
}

func TestEnsureBranch_ReleasesAlwaysFirst(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	// Add regular branches first, then a semver tag.
	for _, b := range []string{"main", "develop"} {
		if _, err := s.EnsureBranch(b); err != nil {
			t.Fatalf("EnsureBranch(%q) error: %v", b, err)
		}
	}

	// A semver tag should register "releases" instead.
	added, err := s.EnsureBranch("v1.0.0")
	if err != nil {
		t.Fatalf("EnsureBranch(v1.0.0) error: %v", err)
	}
	if !added {
		t.Error("expected 'releases' to be newly added")
	}

	branches, err := s.ReadBranches()
	if err != nil {
		t.Fatalf("ReadBranches() error: %v", err)
	}

	// "releases" must be first; individual tag "v1.0.0" must not appear.
	expected := []string{"releases", "develop", "main"}
	if !reflect.DeepEqual(branches, expected) {
		t.Errorf("branches = %v, want %v", branches, expected)
	}

	// Adding another semver tag should not duplicate "releases".
	added2, err := s.EnsureBranch("v2.0.0")
	if err != nil {
		t.Fatalf("EnsureBranch(v2.0.0) error: %v", err)
	}
	if added2 {
		t.Error("expected 'releases' to NOT be newly added on second semver tag")
	}

	branches2, err := s.ReadBranches()
	if err != nil {
		t.Fatalf("ReadBranches() error: %v", err)
	}
	if !reflect.DeepEqual(branches2, expected) {
		t.Errorf("branches after second tag = %v, want %v", branches2, expected)
	}
}

func TestReadBranchData_EmptyWhenNoFile(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	data, err := s.ReadBranchData("nonexistent")
	if err != nil {
		t.Fatalf("ReadBranchData() error: %v", err)
	}
	if data != nil {
		t.Fatalf("expected nil, got %v", data)
	}
}

func TestWriteAndReadBranchData(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	entries := model.BranchData{
		{
			Commit: model.Commit{
				SHA:     "abc123",
				Message: "test commit",
				Author:  "tester",
				Date:    "2024-01-01T00:00:00Z",
				URL:     "https://example.com/commit/abc123",
			},
			Date: 1704067200000,
			Params: model.RunParams{
				CPU:       "Intel Xeon",
				GOOS:      "linux",
				GOARCH:    "amd64",
				GoVersion: "go1.22.0",
				CGO:       true,
			},
			Benchmarks: []model.BenchmarkResult{
				{Name: "BenchmarkFoo", Value: 1234.5, Unit: "ns/op", Extra: "1000 times\n8 procs"},
			},
		},
	}

	if err := s.WriteBranchData("main", entries); err != nil {
		t.Fatalf("WriteBranchData() error: %v", err)
	}

	got, err := s.ReadBranchData("main")
	if err != nil {
		t.Fatalf("ReadBranchData() error: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(got))
	}
	if got[0].Commit.SHA != "abc123" {
		t.Errorf("commit SHA: got %q, want %q", got[0].Commit.SHA, "abc123")
	}
	if got[0].Params.CPU != "Intel Xeon" {
		t.Errorf("CPU: got %q, want %q", got[0].Params.CPU, "Intel Xeon")
	}
	if got[0].Params.GOOS != "linux" {
		t.Errorf("GOOS: got %q, want %q", got[0].Params.GOOS, "linux")
	}
	if got[0].Params.GOARCH != "amd64" {
		t.Errorf("GOARCH: got %q, want %q", got[0].Params.GOARCH, "amd64")
	}
	if got[0].Params.GoVersion != "go1.22.0" {
		t.Errorf("GoVersion: got %q, want %q", got[0].Params.GoVersion, "go1.22.0")
	}
	if got[0].Params.CGO != true {
		t.Errorf("CGO: got %v, want true", got[0].Params.CGO)
	}
	if len(got[0].Benchmarks) != 1 {
		t.Fatalf("expected 1 benchmark, got %d", len(got[0].Benchmarks))
	}
	if got[0].Benchmarks[0].Value != 1234.5 {
		t.Errorf("benchmark value: got %f, want %f", got[0].Benchmarks[0].Value, 1234.5)
	}
}

func TestAppendEntry(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	entry1 := model.BenchmarkEntry{
		Commit: model.Commit{SHA: "aaa111", Message: "first", Date: "2024-01-01T00:00:00Z"},
		Date:   1704067200000,
		Params: model.RunParams{CPU: "cpu1", GOOS: "linux", GOARCH: "amd64", GoVersion: "go1.22.0"},
		Benchmarks: []model.BenchmarkResult{
			{Name: "BenchFoo", Value: 100, Unit: "ns/op"},
		},
	}
	entry2 := model.BenchmarkEntry{
		Commit: model.Commit{SHA: "bbb222", Message: "second", Date: "2024-01-02T00:00:00Z"},
		Date:   1704153600000,
		Params: model.RunParams{CPU: "cpu1", GOOS: "linux", GOARCH: "amd64", GoVersion: "go1.22.0"},
		Benchmarks: []model.BenchmarkResult{
			{Name: "BenchFoo", Value: 90, Unit: "ns/op"},
		},
	}

	if err := s.AppendEntry("main", entry1, 0); err != nil {
		t.Fatalf("AppendEntry(1) error: %v", err)
	}
	if err := s.AppendEntry("main", entry2, 0); err != nil {
		t.Fatalf("AppendEntry(2) error: %v", err)
	}

	// Verify data
	data, err := s.ReadBranchData("main")
	if err != nil {
		t.Fatalf("ReadBranchData() error: %v", err)
	}
	if len(data) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(data))
	}
	if data[0].Commit.SHA != "aaa111" {
		t.Errorf("first entry SHA: got %q, want %q", data[0].Commit.SHA, "aaa111")
	}
	if data[1].Commit.SHA != "bbb222" {
		t.Errorf("second entry SHA: got %q, want %q", data[1].Commit.SHA, "bbb222")
	}

	// Verify branch was registered
	branches, err := s.ReadBranches()
	if err != nil {
		t.Fatalf("ReadBranches() error: %v", err)
	}
	if len(branches) != 1 || branches[0] != "main" {
		t.Errorf("unexpected branches: %v", branches)
	}
}

func TestAppendEntry_MaxItems(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	for i := 0; i < 5; i++ {
		entry := model.BenchmarkEntry{
			Commit: model.Commit{
				SHA:  string(rune('a'+i)) + "00",
				Date: "2024-01-0" + string(rune('1'+i)) + "T00:00:00Z",
			},
			Date:   int64(i * 1000),
			Params: model.RunParams{CPU: "cpu1", GOOS: "linux", GOARCH: "amd64"},
			Benchmarks: []model.BenchmarkResult{
				{Name: "Bench", Value: float64(i * 100), Unit: "ns/op"},
			},
		}
		if err := s.AppendEntry("main", entry, 3); err != nil {
			t.Fatalf("AppendEntry(%d) error: %v", i, err)
		}
	}

	data, err := s.ReadBranchData("main")
	if err != nil {
		t.Fatalf("ReadBranchData() error: %v", err)
	}
	if len(data) != 3 {
		t.Fatalf("expected 3 entries (max-items), got %d", len(data))
	}

	// The oldest entries should have been trimmed; the last 3 should remain.
	if data[0].Date != 2000 {
		t.Errorf("first entry date: got %d, want 2000", data[0].Date)
	}
	if data[2].Date != 4000 {
		t.Errorf("last entry date: got %d, want 4000", data[2].Date)
	}
}

func TestAppendEntry_MultipleBranches(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	entryA := model.BenchmarkEntry{
		Commit:     model.Commit{SHA: "aaa", Date: "2024-01-01T00:00:00Z"},
		Date:       1704067200000,
		Params:     model.RunParams{CPU: "cpu1", GOOS: "linux", GOARCH: "amd64"},
		Benchmarks: []model.BenchmarkResult{{Name: "B", Value: 1, Unit: "ns/op"}},
	}
	entryB := model.BenchmarkEntry{
		Commit:     model.Commit{SHA: "bbb", Date: "2024-01-02T00:00:00Z"},
		Date:       1704153600000,
		Params:     model.RunParams{CPU: "cpu1", GOOS: "linux", GOARCH: "amd64"},
		Benchmarks: []model.BenchmarkResult{{Name: "B", Value: 2, Unit: "ns/op"}},
	}

	if err := s.AppendEntry("main", entryA, 0); err != nil {
		t.Fatalf("AppendEntry(main) error: %v", err)
	}
	if err := s.AppendEntry("develop", entryB, 0); err != nil {
		t.Fatalf("AppendEntry(develop) error: %v", err)
	}

	branches, err := s.ReadBranches()
	if err != nil {
		t.Fatalf("ReadBranches() error: %v", err)
	}
	if len(branches) != 2 {
		t.Fatalf("expected 2 branches, got %d: %v", len(branches), branches)
	}

	mainData, err := s.ReadBranchData("main")
	if err != nil {
		t.Fatalf("ReadBranchData(main) error: %v", err)
	}
	if len(mainData) != 1 || mainData[0].Commit.SHA != "aaa" {
		t.Errorf("unexpected main data: %v", mainData)
	}

	devData, err := s.ReadBranchData("develop")
	if err != nil {
		t.Fatalf("ReadBranchData(develop) error: %v", err)
	}
	if len(devData) != 1 || devData[0].Commit.SHA != "bbb" {
		t.Errorf("unexpected develop data: %v", devData)
	}
}

func TestBranchDataPath_Sanitization(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	entry := model.BenchmarkEntry{
		Commit:     model.Commit{SHA: "ccc", Date: "2024-01-01T00:00:00Z"},
		Date:       1704067200000,
		Params:     model.RunParams{CPU: "cpu1", GOOS: "linux", GOARCH: "amd64"},
		Benchmarks: []model.BenchmarkResult{{Name: "B", Value: 1, Unit: "ns/op"}},
	}

	branch := "feature/my-branch"
	if err := s.AppendEntry(branch, entry, 0); err != nil {
		t.Fatalf("AppendEntry() error: %v", err)
	}

	// The file should be named with sanitized branch name
	expectedFile := filepath.Join(dir, "data", "feature_my-branch.json")
	if _, err := os.Stat(expectedFile); err != nil {
		t.Fatalf("expected sanitized file at %s: %v", expectedFile, err)
	}

	// But we should be able to read it back with the original branch name
	data, err := s.ReadBranchData(branch)
	if err != nil {
		t.Fatalf("ReadBranchData() error: %v", err)
	}
	if len(data) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(data))
	}
}

func TestWriteAndReadMetadata(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	repoURL := "https://github.com/test/repo"
	goModule := "github.com/test/repo"
	if err := s.WriteMetadata(repoURL, goModule); err != nil {
		t.Fatalf("WriteMetadata() error: %v", err)
	}

	meta, err := s.ReadMetadata()
	if err != nil {
		t.Fatalf("ReadMetadata() error: %v", err)
	}
	if meta.RepoURL != repoURL {
		t.Errorf("RepoURL: got %q, want %q", meta.RepoURL, repoURL)
	}
	if meta.GoModule != goModule {
		t.Errorf("GoModule: got %q, want %q", meta.GoModule, goModule)
	}
	if meta.LastUpdate <= 0 {
		t.Errorf("LastUpdate should be positive, got %d", meta.LastUpdate)
	}
}

func TestReadMetadata_EmptyWhenNoFile(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	meta, err := s.ReadMetadata()
	if err != nil {
		t.Fatalf("ReadMetadata() error: %v", err)
	}
	if meta.RepoURL != "" {
		t.Errorf("expected empty RepoURL, got %q", meta.RepoURL)
	}
}

func TestBranchFileName(t *testing.T) {
	tests := []struct {
		branch string
		want   string
	}{
		{"main", "main.json"},
		{"develop", "develop.json"},
		{"feature/foo", "feature_foo.json"},
		{"release/v1.0", "release_v1.0.json"},
		{"has:colons", "has_colons.json"},
		{"has*stars", "has_stars.json"},
	}

	for _, tt := range tests {
		got := BranchFileName(tt.branch)
		if got != tt.want {
			t.Errorf("BranchFileName(%q) = %q, want %q", tt.branch, got, tt.want)
		}
	}
}

func TestBranchData_JSONRoundTrip(t *testing.T) {
	original := model.BranchData{
		{
			Commit: model.Commit{
				SHA:     "deadbeef",
				Message: "A commit with \"quotes\" and\nnewlines",
				Author:  "user",
				Date:    "2024-06-15T10:00:00Z",
				URL:     "https://github.com/test/repo/commit/deadbeef",
			},
			Date: 1718445600000,
			Params: model.RunParams{
				CPU:       "Intel Xeon",
				GOOS:      "linux",
				GOARCH:    "amd64",
				GoVersion: "go1.22.0",
				CGO:       true,
			},
			Benchmarks: []model.BenchmarkResult{
				{Name: "BenchmarkA", Value: 1234.567, Unit: "ns/op", Extra: "100 times\n4 procs"},
				{Name: "BenchmarkA - B/op", Value: 256, Unit: "B/op", Extra: "100 times\n4 procs"},
			},
		},
	}

	data, err := json.MarshalIndent(original, "", "  ")
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded model.BranchData
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if len(decoded) != len(original) {
		t.Fatalf("length mismatch: got %d, want %d", len(decoded), len(original))
	}

	got := decoded[0]
	want := original[0]

	if got.Commit.SHA != want.Commit.SHA {
		t.Errorf("SHA: got %q, want %q", got.Commit.SHA, want.Commit.SHA)
	}
	if got.Commit.Message != want.Commit.Message {
		t.Errorf("Message: got %q, want %q", got.Commit.Message, want.Commit.Message)
	}
	if got.Date != want.Date {
		t.Errorf("Date: got %d, want %d", got.Date, want.Date)
	}
	if got.Params != want.Params {
		t.Errorf("Params: got %+v, want %+v", got.Params, want.Params)
	}
	if len(got.Benchmarks) != len(want.Benchmarks) {
		t.Fatalf("Benchmarks length: got %d, want %d", len(got.Benchmarks), len(want.Benchmarks))
	}
	for i := range want.Benchmarks {
		if got.Benchmarks[i].Name != want.Benchmarks[i].Name {
			t.Errorf("Benchmark[%d].Name: got %q, want %q", i, got.Benchmarks[i].Name, want.Benchmarks[i].Name)
		}
		if got.Benchmarks[i].Value != want.Benchmarks[i].Value {
			t.Errorf("Benchmark[%d].Value: got %f, want %f", i, got.Benchmarks[i].Value, want.Benchmarks[i].Value)
		}
		if got.Benchmarks[i].Unit != want.Benchmarks[i].Unit {
			t.Errorf("Benchmark[%d].Unit: got %q, want %q", i, got.Benchmarks[i].Unit, want.Benchmarks[i].Unit)
		}
		if got.Benchmarks[i].Extra != want.Benchmarks[i].Extra {
			t.Errorf("Benchmark[%d].Extra: got %q, want %q", i, got.Benchmarks[i].Extra, want.Benchmarks[i].Extra)
		}
	}
}

// ---------------------------------------------------------------------------
// Deduplication tests
// ---------------------------------------------------------------------------

func TestAppendEntry_ReplacesExistingWithSameKey(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	params := model.RunParams{
		CPU:       "Intel Xeon",
		GOOS:      "linux",
		GOARCH:    "amd64",
		GoVersion: "go1.22.0",
		CGO:       true,
	}

	// First entry for commit abc with identical params.
	entry1 := model.BenchmarkEntry{
		Commit: model.Commit{SHA: "abc123", Message: "first run", Date: "2024-01-01T00:00:00Z"},
		Date:   1704067200000,
		Params: params,
		Benchmarks: []model.BenchmarkResult{
			{Name: "BenchFoo", Value: 100, Unit: "ns/op"},
		},
	}

	// Second entry for the SAME commit+params — should replace.
	entry2 := model.BenchmarkEntry{
		Commit: model.Commit{SHA: "abc123", Message: "re-run", Date: "2024-01-01T00:00:00Z"},
		Date:   1704067200000,
		Params: params,
		Benchmarks: []model.BenchmarkResult{
			{Name: "BenchFoo", Value: 42, Unit: "ns/op"},
		},
	}

	if err := s.AppendEntry("main", entry1, 0); err != nil {
		t.Fatalf("AppendEntry(1) error: %v", err)
	}
	if err := s.AppendEntry("main", entry2, 0); err != nil {
		t.Fatalf("AppendEntry(2) error: %v", err)
	}

	data, err := s.ReadBranchData("main")
	if err != nil {
		t.Fatalf("ReadBranchData() error: %v", err)
	}

	if len(data) != 1 {
		t.Fatalf("expected 1 entry after replacement, got %d", len(data))
	}
	if data[0].Benchmarks[0].Value != 42 {
		t.Errorf("expected replaced value 42, got %f", data[0].Benchmarks[0].Value)
	}
	if data[0].Commit.Message != "re-run" {
		t.Errorf("expected replaced message %q, got %q", "re-run", data[0].Commit.Message)
	}
}

func TestAppendEntry_DifferentCPU_NoReplace(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	entry1 := model.BenchmarkEntry{
		Commit: model.Commit{SHA: "abc123", Date: "2024-01-01T00:00:00Z"},
		Date:   1704067200000,
		Params: model.RunParams{CPU: "Intel Xeon", GOOS: "linux", GOARCH: "amd64", GoVersion: "go1.22.0", CGO: true},
		Benchmarks: []model.BenchmarkResult{
			{Name: "BenchFoo", Value: 100, Unit: "ns/op"},
		},
	}

	entry2 := model.BenchmarkEntry{
		Commit: model.Commit{SHA: "abc123", Date: "2024-01-01T00:00:00Z"},
		Date:   1704067200000,
		Params: model.RunParams{CPU: "AMD Ryzen", GOOS: "linux", GOARCH: "amd64", GoVersion: "go1.22.0", CGO: true},
		Benchmarks: []model.BenchmarkResult{
			{Name: "BenchFoo", Value: 80, Unit: "ns/op"},
		},
	}

	if err := s.AppendEntry("main", entry1, 0); err != nil {
		t.Fatalf("AppendEntry(1) error: %v", err)
	}
	if err := s.AppendEntry("main", entry2, 0); err != nil {
		t.Fatalf("AppendEntry(2) error: %v", err)
	}

	data, err := s.ReadBranchData("main")
	if err != nil {
		t.Fatalf("ReadBranchData() error: %v", err)
	}

	if len(data) != 2 {
		t.Fatalf("expected 2 entries (different CPU), got %d", len(data))
	}
}

func TestAppendEntry_DifferentCGO_NoReplace(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	entry1 := model.BenchmarkEntry{
		Commit: model.Commit{SHA: "abc123", Date: "2024-01-01T00:00:00Z"},
		Date:   1704067200000,
		Params: model.RunParams{CPU: "Intel Xeon", GOOS: "linux", GOARCH: "amd64", GoVersion: "go1.22.0", CGO: true},
		Benchmarks: []model.BenchmarkResult{
			{Name: "BenchFoo", Value: 100, Unit: "ns/op"},
		},
	}

	entry2 := model.BenchmarkEntry{
		Commit: model.Commit{SHA: "abc123", Date: "2024-01-01T00:00:00Z"},
		Date:   1704067200000,
		Params: model.RunParams{CPU: "Intel Xeon", GOOS: "linux", GOARCH: "amd64", GoVersion: "go1.22.0", CGO: false},
		Benchmarks: []model.BenchmarkResult{
			{Name: "BenchFoo", Value: 120, Unit: "ns/op"},
		},
	}

	if err := s.AppendEntry("main", entry1, 0); err != nil {
		t.Fatalf("AppendEntry(1) error: %v", err)
	}
	if err := s.AppendEntry("main", entry2, 0); err != nil {
		t.Fatalf("AppendEntry(2) error: %v", err)
	}

	data, err := s.ReadBranchData("main")
	if err != nil {
		t.Fatalf("ReadBranchData() error: %v", err)
	}

	if len(data) != 2 {
		t.Fatalf("expected 2 entries (different CGO), got %d", len(data))
	}
}

func TestAppendEntry_DifferentGOOS_NoReplace(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	entry1 := model.BenchmarkEntry{
		Commit: model.Commit{SHA: "abc123", Date: "2024-01-01T00:00:00Z"},
		Date:   1704067200000,
		Params: model.RunParams{CPU: "Intel Xeon", GOOS: "linux", GOARCH: "amd64", GoVersion: "go1.22.0", CGO: true},
		Benchmarks: []model.BenchmarkResult{
			{Name: "BenchFoo", Value: 100, Unit: "ns/op"},
		},
	}

	entry2 := model.BenchmarkEntry{
		Commit: model.Commit{SHA: "abc123", Date: "2024-01-01T00:00:00Z"},
		Date:   1704067200000,
		Params: model.RunParams{CPU: "Intel Xeon", GOOS: "darwin", GOARCH: "amd64", GoVersion: "go1.22.0", CGO: true},
		Benchmarks: []model.BenchmarkResult{
			{Name: "BenchFoo", Value: 110, Unit: "ns/op"},
		},
	}

	if err := s.AppendEntry("main", entry1, 0); err != nil {
		t.Fatalf("AppendEntry(1) error: %v", err)
	}
	if err := s.AppendEntry("main", entry2, 0); err != nil {
		t.Fatalf("AppendEntry(2) error: %v", err)
	}

	data, err := s.ReadBranchData("main")
	if err != nil {
		t.Fatalf("ReadBranchData() error: %v", err)
	}

	if len(data) != 2 {
		t.Fatalf("expected 2 entries (different GOOS), got %d", len(data))
	}
}

func TestAppendEntry_DifferentGOARCH_NoReplace(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	entry1 := model.BenchmarkEntry{
		Commit: model.Commit{SHA: "abc123", Date: "2024-01-01T00:00:00Z"},
		Date:   1704067200000,
		Params: model.RunParams{CPU: "Intel Xeon", GOOS: "linux", GOARCH: "amd64", GoVersion: "go1.22.0", CGO: true},
		Benchmarks: []model.BenchmarkResult{
			{Name: "BenchFoo", Value: 100, Unit: "ns/op"},
		},
	}

	entry2 := model.BenchmarkEntry{
		Commit: model.Commit{SHA: "abc123", Date: "2024-01-01T00:00:00Z"},
		Date:   1704067200000,
		Params: model.RunParams{CPU: "Intel Xeon", GOOS: "linux", GOARCH: "arm64", GoVersion: "go1.22.0", CGO: true},
		Benchmarks: []model.BenchmarkResult{
			{Name: "BenchFoo", Value: 130, Unit: "ns/op"},
		},
	}

	if err := s.AppendEntry("main", entry1, 0); err != nil {
		t.Fatalf("AppendEntry(1) error: %v", err)
	}
	if err := s.AppendEntry("main", entry2, 0); err != nil {
		t.Fatalf("AppendEntry(2) error: %v", err)
	}

	data, err := s.ReadBranchData("main")
	if err != nil {
		t.Fatalf("ReadBranchData() error: %v", err)
	}

	if len(data) != 2 {
		t.Fatalf("expected 2 entries (different GOARCH), got %d", len(data))
	}
}

func TestAppendEntry_DifferentGoVersion_NoReplace(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	entry1 := model.BenchmarkEntry{
		Commit: model.Commit{SHA: "abc123", Date: "2024-01-01T00:00:00Z"},
		Date:   1704067200000,
		Params: model.RunParams{CPU: "Intel Xeon", GOOS: "linux", GOARCH: "amd64", GoVersion: "go1.21.0", CGO: true},
		Benchmarks: []model.BenchmarkResult{
			{Name: "BenchFoo", Value: 100, Unit: "ns/op"},
		},
	}

	entry2 := model.BenchmarkEntry{
		Commit: model.Commit{SHA: "abc123", Date: "2024-01-01T00:00:00Z"},
		Date:   1704067200000,
		Params: model.RunParams{CPU: "Intel Xeon", GOOS: "linux", GOARCH: "amd64", GoVersion: "go1.22.0", CGO: true},
		Benchmarks: []model.BenchmarkResult{
			{Name: "BenchFoo", Value: 95, Unit: "ns/op"},
		},
	}

	if err := s.AppendEntry("main", entry1, 0); err != nil {
		t.Fatalf("AppendEntry(1) error: %v", err)
	}
	if err := s.AppendEntry("main", entry2, 0); err != nil {
		t.Fatalf("AppendEntry(2) error: %v", err)
	}

	data, err := s.ReadBranchData("main")
	if err != nil {
		t.Fatalf("ReadBranchData() error: %v", err)
	}

	if len(data) != 2 {
		t.Fatalf("expected 2 entries (different GoVersion), got %d", len(data))
	}
}

func TestAppendEntry_DifferentCommit_NoReplace(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	params := model.RunParams{CPU: "Intel Xeon", GOOS: "linux", GOARCH: "amd64", GoVersion: "go1.22.0", CGO: true}

	entry1 := model.BenchmarkEntry{
		Commit: model.Commit{SHA: "abc123", Date: "2024-01-01T00:00:00Z"},
		Date:   1704067200000,
		Params: params,
		Benchmarks: []model.BenchmarkResult{
			{Name: "BenchFoo", Value: 100, Unit: "ns/op"},
		},
	}

	entry2 := model.BenchmarkEntry{
		Commit: model.Commit{SHA: "def456", Date: "2024-01-02T00:00:00Z"},
		Date:   1704153600000,
		Params: params,
		Benchmarks: []model.BenchmarkResult{
			{Name: "BenchFoo", Value: 90, Unit: "ns/op"},
		},
	}

	if err := s.AppendEntry("main", entry1, 0); err != nil {
		t.Fatalf("AppendEntry(1) error: %v", err)
	}
	if err := s.AppendEntry("main", entry2, 0); err != nil {
		t.Fatalf("AppendEntry(2) error: %v", err)
	}

	data, err := s.ReadBranchData("main")
	if err != nil {
		t.Fatalf("ReadBranchData() error: %v", err)
	}

	if len(data) != 2 {
		t.Fatalf("expected 2 entries (different commit), got %d", len(data))
	}
}

func TestAppendEntries_BatchReplace(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	params := model.RunParams{CPU: "cpu1", GOOS: "linux", GOARCH: "amd64", GoVersion: "go1.22.0", CGO: true}

	// Seed with initial entries.
	initial := []model.BenchmarkEntry{
		{
			Commit:     model.Commit{SHA: "aaa", Date: "2024-01-01T00:00:00Z"},
			Date:       1704067200000,
			Params:     params,
			Benchmarks: []model.BenchmarkResult{{Name: "B", Value: 1, Unit: "ns/op"}},
		},
		{
			Commit:     model.Commit{SHA: "bbb", Date: "2024-01-02T00:00:00Z"},
			Date:       1704153600000,
			Params:     params,
			Benchmarks: []model.BenchmarkResult{{Name: "B", Value: 2, Unit: "ns/op"}},
		},
	}
	if err := s.AppendEntries("main", initial, 0); err != nil {
		t.Fatalf("initial AppendEntries error: %v", err)
	}

	// Replace the first entry and add a new third one.
	updates := []model.BenchmarkEntry{
		{
			Commit:     model.Commit{SHA: "aaa", Date: "2024-01-01T00:00:00Z"},
			Date:       1704067200000,
			Params:     params,
			Benchmarks: []model.BenchmarkResult{{Name: "B", Value: 999, Unit: "ns/op"}},
		},
		{
			Commit:     model.Commit{SHA: "ccc", Date: "2024-01-03T00:00:00Z"},
			Date:       1704240000000,
			Params:     params,
			Benchmarks: []model.BenchmarkResult{{Name: "B", Value: 3, Unit: "ns/op"}},
		},
	}
	if err := s.AppendEntries("main", updates, 0); err != nil {
		t.Fatalf("update AppendEntries error: %v", err)
	}

	data, err := s.ReadBranchData("main")
	if err != nil {
		t.Fatalf("ReadBranchData() error: %v", err)
	}

	if len(data) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(data))
	}

	// Verify the replaced entry has the new value.
	if data[0].Commit.SHA != "aaa" {
		t.Errorf("first entry SHA: got %q, want %q", data[0].Commit.SHA, "aaa")
	}
	if data[0].Benchmarks[0].Value != 999 {
		t.Errorf("first entry value: got %f, want 999", data[0].Benchmarks[0].Value)
	}

	// Verify the untouched entry is still there.
	if data[1].Commit.SHA != "bbb" {
		t.Errorf("second entry SHA: got %q, want %q", data[1].Commit.SHA, "bbb")
	}
	if data[1].Benchmarks[0].Value != 2 {
		t.Errorf("second entry value: got %f, want 2", data[1].Benchmarks[0].Value)
	}

	// Verify the new entry was added.
	if data[2].Commit.SHA != "ccc" {
		t.Errorf("third entry SHA: got %q, want %q", data[2].Commit.SHA, "ccc")
	}
}

// ---------------------------------------------------------------------------
// Commit-date sorting tests
// ---------------------------------------------------------------------------

func TestAppendEntry_SortedByCommitDate(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	params := model.RunParams{CPU: "cpu1", GOOS: "linux", GOARCH: "amd64", GoVersion: "go1.22.0", CGO: true}

	// Insert entries out of chronological order.
	entries := []model.BenchmarkEntry{
		{
			Commit:     model.Commit{SHA: "ccc", Date: "2024-01-03T00:00:00Z"},
			Date:       1704240000000,
			Params:     params,
			Benchmarks: []model.BenchmarkResult{{Name: "B", Value: 3, Unit: "ns/op"}},
		},
		{
			Commit:     model.Commit{SHA: "aaa", Date: "2024-01-01T00:00:00Z"},
			Date:       1704067200000,
			Params:     params,
			Benchmarks: []model.BenchmarkResult{{Name: "B", Value: 1, Unit: "ns/op"}},
		},
		{
			Commit:     model.Commit{SHA: "bbb", Date: "2024-01-02T00:00:00Z"},
			Date:       1704153600000,
			Params:     params,
			Benchmarks: []model.BenchmarkResult{{Name: "B", Value: 2, Unit: "ns/op"}},
		},
	}

	if err := s.AppendEntries("main", entries, 0); err != nil {
		t.Fatalf("AppendEntries error: %v", err)
	}

	data, err := s.ReadBranchData("main")
	if err != nil {
		t.Fatalf("ReadBranchData() error: %v", err)
	}

	if len(data) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(data))
	}

	// Should be sorted chronologically by commit date.
	if data[0].Commit.SHA != "aaa" {
		t.Errorf("entry[0] SHA: got %q, want %q", data[0].Commit.SHA, "aaa")
	}
	if data[1].Commit.SHA != "bbb" {
		t.Errorf("entry[1] SHA: got %q, want %q", data[1].Commit.SHA, "bbb")
	}
	if data[2].Commit.SHA != "ccc" {
		t.Errorf("entry[2] SHA: got %q, want %q", data[2].Commit.SHA, "ccc")
	}
}

func TestAppendEntry_SortedAfterReplace(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	params := model.RunParams{CPU: "cpu1", GOOS: "linux", GOARCH: "amd64"}

	// Insert in correct order initially.
	for _, e := range []model.BenchmarkEntry{
		{
			Commit:     model.Commit{SHA: "aaa", Date: "2024-01-01T00:00:00Z"},
			Date:       1704067200000,
			Params:     params,
			Benchmarks: []model.BenchmarkResult{{Name: "B", Value: 1, Unit: "ns/op"}},
		},
		{
			Commit:     model.Commit{SHA: "bbb", Date: "2024-01-02T00:00:00Z"},
			Date:       1704153600000,
			Params:     params,
			Benchmarks: []model.BenchmarkResult{{Name: "B", Value: 2, Unit: "ns/op"}},
		},
		{
			Commit:     model.Commit{SHA: "ccc", Date: "2024-01-03T00:00:00Z"},
			Date:       1704240000000,
			Params:     params,
			Benchmarks: []model.BenchmarkResult{{Name: "B", Value: 3, Unit: "ns/op"}},
		},
	} {
		if err := s.AppendEntry("main", e, 0); err != nil {
			t.Fatalf("AppendEntry error: %v", err)
		}
	}

	// Now replace bbb with updated value. Order should be preserved.
	replacement := model.BenchmarkEntry{
		Commit:     model.Commit{SHA: "bbb", Date: "2024-01-02T00:00:00Z"},
		Date:       1704153600000,
		Params:     params,
		Benchmarks: []model.BenchmarkResult{{Name: "B", Value: 999, Unit: "ns/op"}},
	}
	if err := s.AppendEntry("main", replacement, 0); err != nil {
		t.Fatalf("replace AppendEntry error: %v", err)
	}

	data, err := s.ReadBranchData("main")
	if err != nil {
		t.Fatalf("ReadBranchData() error: %v", err)
	}

	if len(data) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(data))
	}

	// Verify order is maintained.
	expectedSHAs := []string{"aaa", "bbb", "ccc"}
	for i, sha := range expectedSHAs {
		if data[i].Commit.SHA != sha {
			t.Errorf("entry[%d] SHA: got %q, want %q", i, data[i].Commit.SHA, sha)
		}
	}

	// Verify replacement took effect.
	if data[1].Benchmarks[0].Value != 999 {
		t.Errorf("replaced entry value: got %f, want 999", data[1].Benchmarks[0].Value)
	}
}

func TestAppendEntry_SortedInsertionOutOfOrder(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	params := model.RunParams{CPU: "cpu1", GOOS: "linux", GOARCH: "amd64"}

	// Append entries one at a time, out of order, to simulate late-arriving data.
	e3 := model.BenchmarkEntry{
		Commit:     model.Commit{SHA: "ccc", Date: "2024-03-01T00:00:00Z"},
		Date:       1709251200000,
		Params:     params,
		Benchmarks: []model.BenchmarkResult{{Name: "B", Value: 3, Unit: "ns/op"}},
	}
	e1 := model.BenchmarkEntry{
		Commit:     model.Commit{SHA: "aaa", Date: "2024-01-01T00:00:00Z"},
		Date:       1704067200000,
		Params:     params,
		Benchmarks: []model.BenchmarkResult{{Name: "B", Value: 1, Unit: "ns/op"}},
	}
	e2 := model.BenchmarkEntry{
		Commit:     model.Commit{SHA: "bbb", Date: "2024-02-01T00:00:00Z"},
		Date:       1706745600000,
		Params:     params,
		Benchmarks: []model.BenchmarkResult{{Name: "B", Value: 2, Unit: "ns/op"}},
	}

	for _, e := range []model.BenchmarkEntry{e3, e1, e2} {
		if err := s.AppendEntry("main", e, 0); err != nil {
			t.Fatalf("AppendEntry error: %v", err)
		}
	}

	data, err := s.ReadBranchData("main")
	if err != nil {
		t.Fatalf("ReadBranchData() error: %v", err)
	}

	if len(data) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(data))
	}

	expectedSHAs := []string{"aaa", "bbb", "ccc"}
	for i, sha := range expectedSHAs {
		if data[i].Commit.SHA != sha {
			t.Errorf("entry[%d] SHA: got %q, want %q", i, data[i].Commit.SHA, sha)
		}
	}
}

func TestAppendEntry_MaxItemsAfterReplace(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	params := model.RunParams{CPU: "cpu1", GOOS: "linux", GOARCH: "amd64"}

	// Seed with 3 entries.
	for i, sha := range []string{"aaa", "bbb", "ccc"} {
		e := model.BenchmarkEntry{
			Commit:     model.Commit{SHA: sha, Date: "2024-01-0" + string(rune('1'+i)) + "T00:00:00Z"},
			Date:       int64(1704067200000 + i*86400000),
			Params:     params,
			Benchmarks: []model.BenchmarkResult{{Name: "B", Value: float64(i + 1), Unit: "ns/op"}},
		}
		if err := s.AppendEntry("main", e, 0); err != nil {
			t.Fatalf("seed AppendEntry error: %v", err)
		}
	}

	// Replace bbb (same key), with maxItems=2.
	replacement := model.BenchmarkEntry{
		Commit:     model.Commit{SHA: "bbb", Date: "2024-01-02T00:00:00Z"},
		Date:       1704153600000,
		Params:     params,
		Benchmarks: []model.BenchmarkResult{{Name: "B", Value: 999, Unit: "ns/op"}},
	}
	if err := s.AppendEntry("main", replacement, 2); err != nil {
		t.Fatalf("replace AppendEntry error: %v", err)
	}

	data, err := s.ReadBranchData("main")
	if err != nil {
		t.Fatalf("ReadBranchData() error: %v", err)
	}

	// After replacement we have 3 entries, maxItems=2 trims to last 2.
	if len(data) != 2 {
		t.Fatalf("expected 2 entries (maxItems), got %d", len(data))
	}

	if data[0].Commit.SHA != "bbb" {
		t.Errorf("entry[0] SHA: got %q, want %q", data[0].Commit.SHA, "bbb")
	}
	if data[1].Commit.SHA != "ccc" {
		t.Errorf("entry[1] SHA: got %q, want %q", data[1].Commit.SHA, "ccc")
	}
}

// ---------------------------------------------------------------------------
// EntryKey tests
// ---------------------------------------------------------------------------

func TestEntryKey(t *testing.T) {
	e1 := model.BenchmarkEntry{
		Commit: model.Commit{SHA: "abc123"},
		Params: model.RunParams{CPU: "Intel Xeon", GOOS: "linux", GOARCH: "amd64", GoVersion: "go1.22.0", CGO: true},
	}
	e2 := model.BenchmarkEntry{
		Commit: model.Commit{SHA: "abc123"},
		Params: model.RunParams{CPU: "Intel Xeon", GOOS: "linux", GOARCH: "amd64", GoVersion: "go1.22.0", CGO: true},
	}
	// Different CPU
	e3 := model.BenchmarkEntry{
		Commit: model.Commit{SHA: "abc123"},
		Params: model.RunParams{CPU: "AMD Ryzen", GOOS: "linux", GOARCH: "amd64", GoVersion: "go1.22.0", CGO: true},
	}
	// Different CGO
	e4 := model.BenchmarkEntry{
		Commit: model.Commit{SHA: "abc123"},
		Params: model.RunParams{CPU: "Intel Xeon", GOOS: "linux", GOARCH: "amd64", GoVersion: "go1.22.0", CGO: false},
	}
	// Different commit
	e5 := model.BenchmarkEntry{
		Commit: model.Commit{SHA: "def456"},
		Params: model.RunParams{CPU: "Intel Xeon", GOOS: "linux", GOARCH: "amd64", GoVersion: "go1.22.0", CGO: true},
	}
	// Different GOOS
	e6 := model.BenchmarkEntry{
		Commit: model.Commit{SHA: "abc123"},
		Params: model.RunParams{CPU: "Intel Xeon", GOOS: "darwin", GOARCH: "amd64", GoVersion: "go1.22.0", CGO: true},
	}
	// Different GOARCH
	e7 := model.BenchmarkEntry{
		Commit: model.Commit{SHA: "abc123"},
		Params: model.RunParams{CPU: "Intel Xeon", GOOS: "linux", GOARCH: "arm64", GoVersion: "go1.22.0", CGO: true},
	}
	// Different GoVersion
	e8 := model.BenchmarkEntry{
		Commit: model.Commit{SHA: "abc123"},
		Params: model.RunParams{CPU: "Intel Xeon", GOOS: "linux", GOARCH: "amd64", GoVersion: "go1.21.0", CGO: true},
	}

	if e1.EntryKey() != e2.EntryKey() {
		t.Error("same commit+params should produce same key")
	}
	if e1.EntryKey() == e3.EntryKey() {
		t.Error("different CPU should produce different key")
	}
	if e1.EntryKey() == e4.EntryKey() {
		t.Error("different CGO should produce different key")
	}
	if e1.EntryKey() == e5.EntryKey() {
		t.Error("different commit should produce different key")
	}
	if e1.EntryKey() == e6.EntryKey() {
		t.Error("different GOOS should produce different key")
	}
	if e1.EntryKey() == e7.EntryKey() {
		t.Error("different GOARCH should produce different key")
	}
	if e1.EntryKey() == e8.EntryKey() {
		t.Error("different GoVersion should produce different key")
	}
}

// ---------------------------------------------------------------------------
// Semver detection tests
// ---------------------------------------------------------------------------

func TestIsSemanticVersionTag(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"v1.0.0", true},
		{"v0.1.0", true},
		{"v12.34.56", true},
		{"v1.2.3-beta.1", true},
		{"v1.2.3-rc.1", true},
		{"1.0.0", true},
		{"0.1.0", true},
		{"1.2.3-alpha", true},
		{"main", false},
		{"develop", false},
		{"feature/foo", false},
		{"release/1.0", false},
		{"v1", false},
		{"v1.0", false},
		{"releases", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsSemanticVersionTag(tt.name)
			if got != tt.want {
				t.Errorf("IsSemanticVersionTag(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Releases virtual branch tests
// ---------------------------------------------------------------------------

func TestAppendEntries_SemverTag_CreatesReleasesData(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	entry := model.BenchmarkEntry{
		Commit: model.Commit{SHA: "aaa111", Date: "2024-01-01T00:00:00Z"},
		Date:   1704067200000,
		Params: model.RunParams{CPU: "TestCPU", GOOS: "linux", GOARCH: "amd64", GoVersion: "go1.22.0", CGO: true},
		Benchmarks: []model.BenchmarkResult{
			{Name: "BenchmarkFoo", Value: 100, Unit: "ns/op"},
		},
	}

	// Append as a semver tag.
	if err := s.AppendEntries("v1.0.0", []model.BenchmarkEntry{entry}, 0); err != nil {
		t.Fatalf("AppendEntries(v1.0.0) error: %v", err)
	}

	// The individual tag data file should exist.
	tagData, err := s.ReadBranchData("v1.0.0")
	if err != nil {
		t.Fatalf("ReadBranchData(v1.0.0) error: %v", err)
	}
	if len(tagData) != 1 {
		t.Fatalf("tag data: got %d entries, want 1", len(tagData))
	}

	// The combined "releases" data file should also exist with the same entry.
	relData, err := s.ReadBranchData(ReleasesVirtualBranch)
	if err != nil {
		t.Fatalf("ReadBranchData(releases) error: %v", err)
	}
	if len(relData) != 1 {
		t.Fatalf("releases data: got %d entries, want 1", len(relData))
	}
	if relData[0].Commit.SHA != "aaa111" {
		t.Errorf("releases entry SHA = %q, want %q", relData[0].Commit.SHA, "aaa111")
	}

	// branches.json should contain "releases" (not "v1.0.0") and it should be first.
	branches, err := s.ReadBranches()
	if err != nil {
		t.Fatalf("ReadBranches() error: %v", err)
	}
	if len(branches) != 1 || branches[0] != ReleasesVirtualBranch {
		t.Errorf("branches = %v, want [%q]", branches, ReleasesVirtualBranch)
	}
}

func TestAppendEntries_MultipleSemverTags_AggregateInReleases(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	entryV1 := model.BenchmarkEntry{
		Commit: model.Commit{SHA: "aaa111", Date: "2024-01-01T00:00:00Z"},
		Date:   1704067200000,
		Params: model.RunParams{CPU: "TestCPU", GOOS: "linux", GOARCH: "amd64", GoVersion: "go1.22.0", CGO: true},
		Benchmarks: []model.BenchmarkResult{
			{Name: "BenchmarkFoo", Value: 100, Unit: "ns/op"},
		},
	}

	entryV2 := model.BenchmarkEntry{
		Commit: model.Commit{SHA: "bbb222", Date: "2024-06-01T00:00:00Z"},
		Date:   1717200000000,
		Params: model.RunParams{CPU: "TestCPU", GOOS: "linux", GOARCH: "amd64", GoVersion: "go1.22.0", CGO: true},
		Benchmarks: []model.BenchmarkResult{
			{Name: "BenchmarkFoo", Value: 80, Unit: "ns/op"},
		},
	}

	if err := s.AppendEntries("v1.0.0", []model.BenchmarkEntry{entryV1}, 0); err != nil {
		t.Fatalf("AppendEntries(v1.0.0) error: %v", err)
	}
	if err := s.AppendEntries("v2.0.0", []model.BenchmarkEntry{entryV2}, 0); err != nil {
		t.Fatalf("AppendEntries(v2.0.0) error: %v", err)
	}

	// Combined releases should have both entries sorted by commit date.
	relData, err := s.ReadBranchData(ReleasesVirtualBranch)
	if err != nil {
		t.Fatalf("ReadBranchData(releases) error: %v", err)
	}
	if len(relData) != 2 {
		t.Fatalf("releases data: got %d entries, want 2", len(relData))
	}
	if relData[0].Commit.SHA != "aaa111" || relData[1].Commit.SHA != "bbb222" {
		t.Errorf("releases order: got [%s, %s], want [aaa111, bbb222]",
			relData[0].Commit.SHA, relData[1].Commit.SHA)
	}

	// branches.json should only have "releases" once.
	branches, err := s.ReadBranches()
	if err != nil {
		t.Fatalf("ReadBranches() error: %v", err)
	}
	if len(branches) != 1 || branches[0] != ReleasesVirtualBranch {
		t.Errorf("branches = %v, want [%q]", branches, ReleasesVirtualBranch)
	}
}

func TestAppendEntries_SemverTag_RecordsReleaseTags(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	entryV1 := model.BenchmarkEntry{
		Commit: model.Commit{SHA: "aaa111", Date: "2024-01-01T00:00:00Z"},
		Date:   1704067200000,
		Params: model.RunParams{CPU: "TestCPU", GOOS: "linux", GOARCH: "amd64"},
		Benchmarks: []model.BenchmarkResult{
			{Name: "BenchmarkFoo", Value: 100, Unit: "ns/op"},
		},
	}

	entryV2 := model.BenchmarkEntry{
		Commit: model.Commit{SHA: "bbb222", Date: "2024-06-01T00:00:00Z"},
		Date:   1717200000000,
		Params: model.RunParams{CPU: "TestCPU", GOOS: "linux", GOARCH: "amd64"},
		Benchmarks: []model.BenchmarkResult{
			{Name: "BenchmarkFoo", Value: 80, Unit: "ns/op"},
		},
	}

	if err := s.AppendEntries("v1.0.0", []model.BenchmarkEntry{entryV1}, 0); err != nil {
		t.Fatalf("AppendEntries(v1.0.0) error: %v", err)
	}
	if err := s.AppendEntries("v2.0.0", []model.BenchmarkEntry{entryV2}, 0); err != nil {
		t.Fatalf("AppendEntries(v2.0.0) error: %v", err)
	}

	// Read the release_tags.json map directly.
	tags, err := s.readReleaseTags()
	if err != nil {
		t.Fatalf("readReleaseTags() error: %v", err)
	}

	expected := map[string]string{
		"aaa111": "v1.0.0",
		"bbb222": "v2.0.0",
	}
	if !reflect.DeepEqual(tags, expected) {
		t.Errorf("release tags = %v, want %v", tags, expected)
	}

	// Verify the file actually exists on disk.
	rawData, err := os.ReadFile(s.releaseTagsPath())
	if err != nil {
		t.Fatalf("reading release_tags.json: %v", err)
	}
	var diskTags map[string]string
	if err := json.Unmarshal(rawData, &diskTags); err != nil {
		t.Fatalf("decoding release_tags.json: %v", err)
	}
	if !reflect.DeepEqual(diskTags, expected) {
		t.Errorf("on-disk release tags = %v, want %v", diskTags, expected)
	}
}

func TestAppendEntries_RegularBranch_NoReleasesFile(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	entry := model.BenchmarkEntry{
		Commit: model.Commit{SHA: "aaa111", Date: "2024-01-01T00:00:00Z"},
		Date:   1704067200000,
		Params: model.RunParams{CPU: "TestCPU", GOOS: "linux", GOARCH: "amd64"},
		Benchmarks: []model.BenchmarkResult{
			{Name: "BenchmarkFoo", Value: 100, Unit: "ns/op"},
		},
	}

	// Append as a regular branch (not semver).
	if err := s.AppendEntries("main", []model.BenchmarkEntry{entry}, 0); err != nil {
		t.Fatalf("AppendEntries(main) error: %v", err)
	}

	// The branch data should exist.
	data, err := s.ReadBranchData("main")
	if err != nil {
		t.Fatalf("ReadBranchData(main) error: %v", err)
	}
	if len(data) != 1 {
		t.Fatalf("main data: got %d entries, want 1", len(data))
	}

	// The "releases" data file should NOT exist.
	_, err = os.Stat(s.branchDataPath(ReleasesVirtualBranch))
	if err == nil {
		t.Error("releases data file should not exist for a regular branch")
	}

	// release_tags.json should NOT exist.
	_, err = os.Stat(s.releaseTagsPath())
	if err == nil {
		t.Error("release_tags.json should not exist for a regular branch")
	}

	// branches.json should contain "main", not "releases".
	branches, err := s.ReadBranches()
	if err != nil {
		t.Fatalf("ReadBranches() error: %v", err)
	}
	if len(branches) != 1 || branches[0] != "main" {
		t.Errorf("branches = %v, want [\"main\"]", branches)
	}
}

func TestAppendEntries_MixedBranchesAndTags(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	mainEntry := model.BenchmarkEntry{
		Commit: model.Commit{SHA: "aaa111", Date: "2024-01-01T00:00:00Z"},
		Date:   1704067200000,
		Params: model.RunParams{CPU: "TestCPU", GOOS: "linux", GOARCH: "amd64"},
		Benchmarks: []model.BenchmarkResult{
			{Name: "BenchmarkFoo", Value: 100, Unit: "ns/op"},
		},
	}
	tagEntry := model.BenchmarkEntry{
		Commit: model.Commit{SHA: "bbb222", Date: "2024-06-01T00:00:00Z"},
		Date:   1717200000000,
		Params: model.RunParams{CPU: "TestCPU", GOOS: "linux", GOARCH: "amd64"},
		Benchmarks: []model.BenchmarkResult{
			{Name: "BenchmarkFoo", Value: 80, Unit: "ns/op"},
		},
	}

	if err := s.AppendEntries("main", []model.BenchmarkEntry{mainEntry}, 0); err != nil {
		t.Fatalf("AppendEntries(main) error: %v", err)
	}
	if err := s.AppendEntries("v1.0.0", []model.BenchmarkEntry{tagEntry}, 0); err != nil {
		t.Fatalf("AppendEntries(v1.0.0) error: %v", err)
	}

	// branches.json should have "releases" first, then "main". No "v1.0.0".
	branches, err := s.ReadBranches()
	if err != nil {
		t.Fatalf("ReadBranches() error: %v", err)
	}
	expected := []string{ReleasesVirtualBranch, "main"}
	if !reflect.DeepEqual(branches, expected) {
		t.Errorf("branches = %v, want %v", branches, expected)
	}

	// main data should only have mainEntry.
	mainData, err := s.ReadBranchData("main")
	if err != nil {
		t.Fatalf("ReadBranchData(main) error: %v", err)
	}
	if len(mainData) != 1 || mainData[0].Commit.SHA != "aaa111" {
		t.Errorf("main data unexpected: %+v", mainData)
	}

	// releases data should only have tagEntry.
	relData, err := s.ReadBranchData(ReleasesVirtualBranch)
	if err != nil {
		t.Fatalf("ReadBranchData(releases) error: %v", err)
	}
	if len(relData) != 1 || relData[0].Commit.SHA != "bbb222" {
		t.Errorf("releases data unexpected: %+v", relData)
	}
}

func TestAppendEntries_SemverTag_ReplacesDuplicateInReleases(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	entry1 := model.BenchmarkEntry{
		Commit: model.Commit{SHA: "aaa111", Date: "2024-01-01T00:00:00Z"},
		Date:   1704067200000,
		Params: model.RunParams{CPU: "TestCPU", GOOS: "linux", GOARCH: "amd64"},
		Benchmarks: []model.BenchmarkResult{
			{Name: "BenchmarkFoo", Value: 100, Unit: "ns/op"},
		},
	}

	// Store v1.0.0 the first time.
	if err := s.AppendEntries("v1.0.0", []model.BenchmarkEntry{entry1}, 0); err != nil {
		t.Fatalf("first AppendEntries(v1.0.0) error: %v", err)
	}

	// Re-run the same tag (same commit + params) with updated value.
	entry1Updated := entry1
	entry1Updated.Benchmarks = []model.BenchmarkResult{
		{Name: "BenchmarkFoo", Value: 90, Unit: "ns/op"},
	}
	if err := s.AppendEntries("v1.0.0", []model.BenchmarkEntry{entry1Updated}, 0); err != nil {
		t.Fatalf("second AppendEntries(v1.0.0) error: %v", err)
	}

	// releases should have exactly 1 entry (replaced, not duplicated).
	relData, err := s.ReadBranchData(ReleasesVirtualBranch)
	if err != nil {
		t.Fatalf("ReadBranchData(releases) error: %v", err)
	}
	if len(relData) != 1 {
		t.Fatalf("releases data: got %d entries, want 1", len(relData))
	}
	if relData[0].Benchmarks[0].Value != 90 {
		t.Errorf("releases entry value = %v, want 90 (updated)", relData[0].Benchmarks[0].Value)
	}
}

func TestAppendEntries_SemverTag_MaxItemsApplied(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	tags := []struct {
		tag  string
		sha  string
		date string
	}{
		{"v1.0.0", "aaa", "2024-01-01T00:00:00Z"},
		{"v2.0.0", "bbb", "2024-02-01T00:00:00Z"},
		{"v3.0.0", "ccc", "2024-03-01T00:00:00Z"},
	}

	for _, tt := range tags {
		entry := model.BenchmarkEntry{
			Commit: model.Commit{SHA: tt.sha, Date: tt.date},
			Params: model.RunParams{CPU: "TestCPU", GOOS: "linux", GOARCH: "amd64"},
			Benchmarks: []model.BenchmarkResult{
				{Name: "BenchmarkFoo", Value: 100, Unit: "ns/op"},
			},
		}
		if err := s.AppendEntries(tt.tag, []model.BenchmarkEntry{entry}, 2); err != nil {
			t.Fatalf("AppendEntries(%s) error: %v", tt.tag, err)
		}
	}

	// With maxItems=2, releases should only keep the 2 newest.
	relData, err := s.ReadBranchData(ReleasesVirtualBranch)
	if err != nil {
		t.Fatalf("ReadBranchData(releases) error: %v", err)
	}
	if len(relData) != 2 {
		t.Fatalf("releases data: got %d entries, want 2", len(relData))
	}
	if relData[0].Commit.SHA != "bbb" || relData[1].Commit.SHA != "ccc" {
		t.Errorf("releases entries: got [%s, %s], want [bbb, ccc]",
			relData[0].Commit.SHA, relData[1].Commit.SHA)
	}
}
