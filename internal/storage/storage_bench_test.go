package storage

import (
	"fmt"
	"testing"

	"github.com/royalcat/go-continuous-benchmarking/internal/model"
)

func makeEntry(sha string, nBenchmarks int) model.BenchmarkEntry {
	benches := make([]model.BenchmarkResult, 0, nBenchmarks)
	for i := 0; i < nBenchmarks; i++ {
		benches = append(benches, model.BenchmarkResult{
			Name:  fmt.Sprintf("BenchmarkFunc%d", i),
			Value: float64(1000 + i*13),
			Unit:  "ns/op",
			Extra: fmt.Sprintf("%d times\n8 procs", 1000000/(i+1)),
		})
	}
	return model.BenchmarkEntry{
		Commit: model.Commit{
			SHA:     sha,
			Message: "commit " + sha,
			Author:  "tester",
			Date:    "2024-06-15T10:00:00Z",
			URL:     "https://github.com/test/repo/commit/" + sha,
		},
		Date:       1718445600000,
		Benchmarks: benches,
	}
}

func seedStorage(b *testing.B, s *Storage, branch string, n int, benchesPerEntry int) {
	b.Helper()
	for i := 0; i < n; i++ {
		entry := makeEntry(fmt.Sprintf("%040x", i), benchesPerEntry)
		if err := s.AppendEntry(branch, entry, 0); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAppendEntry_EmptyStorage(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		b.StopTimer()
		dir := b.TempDir()
		s, err := New(dir)
		if err != nil {
			b.Fatal(err)
		}
		entry := makeEntry("abc123", 5)
		b.StartTimer()

		if err := s.AppendEntry("main", entry, 0); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAppendEntry_Existing10(b *testing.B) {
	benchmarkAppendToExisting(b, 10, 5)
}

func BenchmarkAppendEntry_Existing100(b *testing.B) {
	benchmarkAppendToExisting(b, 100, 5)
}

func BenchmarkAppendEntry_Existing1000(b *testing.B) {
	benchmarkAppendToExisting(b, 1000, 5)
}

func benchmarkAppendToExisting(b *testing.B, existingEntries int, benchesPerEntry int) {
	b.Helper()
	b.ReportAllocs()
	for b.Loop() {
		b.StopTimer()
		dir := b.TempDir()
		s, err := New(dir)
		if err != nil {
			b.Fatal(err)
		}
		seedStorage(b, s, "main", existingEntries, benchesPerEntry)
		entry := makeEntry("newcommit", benchesPerEntry)
		b.StartTimer()

		if err := s.AppendEntry("main", entry, 0); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAppendEntry_WithMaxItems(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		b.StopTimer()
		dir := b.TempDir()
		s, err := New(dir)
		if err != nil {
			b.Fatal(err)
		}
		seedStorage(b, s, "main", 200, 5)
		entry := makeEntry("newcommit", 5)
		b.StartTimer()

		if err := s.AppendEntry("main", entry, 100); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkReadBranchData_10(b *testing.B) {
	benchmarkReadBranchData(b, 10, 5)
}

func BenchmarkReadBranchData_100(b *testing.B) {
	benchmarkReadBranchData(b, 100, 5)
}

func BenchmarkReadBranchData_1000(b *testing.B) {
	benchmarkReadBranchData(b, 1000, 5)
}

func benchmarkReadBranchData(b *testing.B, entries int, benchesPerEntry int) {
	b.Helper()
	dir := b.TempDir()
	s, err := New(dir)
	if err != nil {
		b.Fatal(err)
	}
	seedStorage(b, s, "main", entries, benchesPerEntry)

	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		_, err := s.ReadBranchData("main")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkWriteBranchData_10(b *testing.B) {
	benchmarkWriteBranchData(b, 10, 5)
}

func BenchmarkWriteBranchData_100(b *testing.B) {
	benchmarkWriteBranchData(b, 100, 5)
}

func BenchmarkWriteBranchData_1000(b *testing.B) {
	benchmarkWriteBranchData(b, 1000, 5)
}

func benchmarkWriteBranchData(b *testing.B, entries int, benchesPerEntry int) {
	b.Helper()
	dir := b.TempDir()
	s, err := New(dir)
	if err != nil {
		b.Fatal(err)
	}

	data := make(model.BranchData, 0, entries)
	for i := 0; i < entries; i++ {
		data = append(data, makeEntry(fmt.Sprintf("%040x", i), benchesPerEntry))
	}

	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		if err := s.WriteBranchData("main", data); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEnsureBranch_New(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		b.StopTimer()
		dir := b.TempDir()
		s, err := New(dir)
		if err != nil {
			b.Fatal(err)
		}
		b.StartTimer()

		_, err = s.EnsureBranch("main")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEnsureBranch_Existing(b *testing.B) {
	dir := b.TempDir()
	s, err := New(dir)
	if err != nil {
		b.Fatal(err)
	}
	// Pre-populate with branches
	for i := 0; i < 50; i++ {
		if _, err := s.EnsureBranch(fmt.Sprintf("branch-%03d", i)); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		// Check a branch that already exists (middle of the list)
		_, err := s.EnsureBranch("branch-025")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkReadBranches_50(b *testing.B) {
	dir := b.TempDir()
	s, err := New(dir)
	if err != nil {
		b.Fatal(err)
	}
	for i := 0; i < 50; i++ {
		if _, err := s.EnsureBranch(fmt.Sprintf("branch-%03d", i)); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		_, err := s.ReadBranches()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkWriteMetadata(b *testing.B) {
	dir := b.TempDir()
	s, err := New(dir)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		if err := s.WriteMetadata("https://github.com/test/repo", "github.com/test/repo"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkReadMetadata(b *testing.B) {
	dir := b.TempDir()
	s, err := New(dir)
	if err != nil {
		b.Fatal(err)
	}
	if err := s.WriteMetadata("https://github.com/test/repo", "github.com/test/repo"); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		_, err := s.ReadMetadata()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSanitizeBranchName_Simple(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = sanitizeBranchName("main")
	}
}

func BenchmarkSanitizeBranchName_Complex(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = sanitizeBranchName("feature/my-cool-branch:v2.0*beta?release")
	}
}

func BenchmarkBranchFileName(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = BranchFileName("feature/release/v1.2.3")
	}
}

func BenchmarkAppendEntry_LargeBenchmarks(b *testing.B) {
	// Entry with many benchmark results (simulating a big test suite)
	b.ReportAllocs()
	for b.Loop() {
		b.StopTimer()
		dir := b.TempDir()
		s, err := New(dir)
		if err != nil {
			b.Fatal(err)
		}
		entry := makeEntry("abc123", 100)
		b.StartTimer()

		if err := s.AppendEntry("main", entry, 0); err != nil {
			b.Fatal(err)
		}
	}
}
