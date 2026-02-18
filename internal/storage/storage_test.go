package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
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
		Commit: model.Commit{SHA: "aaa111", Message: "first"},
		Date:   1000,
		Benchmarks: []model.BenchmarkResult{
			{Name: "BenchFoo", Value: 100, Unit: "ns/op"},
		},
	}
	entry2 := model.BenchmarkEntry{
		Commit: model.Commit{SHA: "bbb222", Message: "second"},
		Date:   2000,
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
			Commit: model.Commit{SHA: string(rune('a'+i)) + "00"},
			Date:   int64(i * 1000),
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
		Commit:     model.Commit{SHA: "aaa"},
		Date:       1000,
		Benchmarks: []model.BenchmarkResult{{Name: "B", Value: 1, Unit: "ns/op"}},
	}
	entryB := model.BenchmarkEntry{
		Commit:     model.Commit{SHA: "bbb"},
		Date:       2000,
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
		Commit:     model.Commit{SHA: "ccc"},
		Date:       3000,
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
	if err := s.WriteMetadata(repoURL); err != nil {
		t.Fatalf("WriteMetadata() error: %v", err)
	}

	meta, err := s.ReadMetadata()
	if err != nil {
		t.Fatalf("ReadMetadata() error: %v", err)
	}
	if meta.RepoURL != repoURL {
		t.Errorf("RepoURL: got %q, want %q", meta.RepoURL, repoURL)
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
