package parse

import (
	"strings"
	"testing"

	"github.com/royalcat/go-continuous-benchmarking/internal/model"
)

func TestParseGoBenchOutput_Simple(t *testing.T) {
	input := `goos: linux
goarch: amd64
pkg: github.com/user/repo
cpu: Intel(R) Core(TM) i7-8700 CPU @ 3.20GHz
BenchmarkFib10-12        3000000               456.7 ns/op
BenchmarkFib20-12          30000             46573.2 ns/op
PASS
ok      github.com/user/repo    3.456s
`

	results, err := ParseGoBenchOutput(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	assertResult(t, results[0], model.BenchmarkResult{
		Name:    "BenchmarkFib10",
		Value:   456.7,
		Unit:    "ns/op",
		Extra:   "3000000 times\n12 procs",
		Package: "github.com/user/repo",
		Procs:   12,
	})

	assertResult(t, results[1], model.BenchmarkResult{
		Name:    "BenchmarkFib20",
		Value:   46573.2,
		Unit:    "ns/op",
		Extra:   "30000 times\n12 procs",
		Package: "github.com/user/repo",
		Procs:   12,
	})
}

func TestParseGoBenchOutput_MultipleMetrics(t *testing.T) {
	input := `goos: linux
goarch: amd64
pkg: github.com/user/repo
BenchmarkAlloc-8      10000        15000 ns/op        1024 B/op          5 allocs/op
PASS
ok      github.com/user/repo    1.234s
`

	results, err := ParseGoBenchOutput(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	assertResult(t, results[0], model.BenchmarkResult{
		Name:    "BenchmarkAlloc",
		Value:   15000,
		Unit:    "ns/op",
		Extra:   "10000 times\n8 procs",
		Package: "github.com/user/repo",
		Procs:   8,
	})

	assertResult(t, results[1], model.BenchmarkResult{
		Name:    "BenchmarkAlloc - B/op",
		Value:   1024,
		Unit:    "B/op",
		Extra:   "10000 times\n8 procs",
		Package: "github.com/user/repo",
		Procs:   8,
	})

	assertResult(t, results[2], model.BenchmarkResult{
		Name:    "BenchmarkAlloc - allocs/op",
		Value:   5,
		Unit:    "allocs/op",
		Extra:   "10000 times\n8 procs",
		Package: "github.com/user/repo",
		Procs:   8,
	})
}

func TestParseGoBenchOutput_NoProcs(t *testing.T) {
	input := `goos: linux
goarch: amd64
pkg: github.com/user/repo
BenchmarkSimple      5000000               300.0 ns/op
PASS
`

	results, err := ParseGoBenchOutput(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	assertResult(t, results[0], model.BenchmarkResult{
		Name:    "BenchmarkSimple",
		Value:   300.0,
		Unit:    "ns/op",
		Extra:   "5000000 times",
		Package: "github.com/user/repo",
		Procs:   0,
	})
}

func TestParseGoBenchOutput_MultiplePackages(t *testing.T) {
	input := `goos: linux
goarch: amd64
pkg: github.com/user/repo/pkga
BenchmarkA-4      1000000              1000 ns/op
pkg: github.com/user/repo/pkgb
BenchmarkB-4       500000              2000 ns/op
PASS
`

	results, err := ParseGoBenchOutput(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// With the new model, names should NOT include the package prefix.
	// Package is stored separately in the Package field.
	assertResult(t, results[0], model.BenchmarkResult{
		Name:    "BenchmarkA",
		Value:   1000,
		Unit:    "ns/op",
		Extra:   "1000000 times\n4 procs",
		Package: "github.com/user/repo/pkga",
		Procs:   4,
	})

	assertResult(t, results[1], model.BenchmarkResult{
		Name:    "BenchmarkB",
		Value:   2000,
		Unit:    "ns/op",
		Extra:   "500000 times\n4 procs",
		Package: "github.com/user/repo/pkgb",
		Procs:   4,
	})
}

func TestParseGoBenchOutput_Empty(t *testing.T) {
	input := `goos: linux
goarch: amd64
PASS
ok      github.com/user/repo    0.001s
`

	_, err := ParseGoBenchOutput(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for empty output, got nil")
	}
}

func TestParseGoBenchOutput_SubBenchmarks(t *testing.T) {
	input := `goos: linux
goarch: amd64
pkg: github.com/user/repo
BenchmarkParent/SubCase1-8        1000000              1100 ns/op
BenchmarkParent/SubCase2-8         500000              2200 ns/op
PASS
`

	results, err := ParseGoBenchOutput(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	assertResult(t, results[0], model.BenchmarkResult{
		Name:    "BenchmarkParent/SubCase1",
		Value:   1100,
		Unit:    "ns/op",
		Extra:   "1000000 times\n8 procs",
		Package: "github.com/user/repo",
		Procs:   8,
	})

	assertResult(t, results[1], model.BenchmarkResult{
		Name:    "BenchmarkParent/SubCase2",
		Value:   2200,
		Unit:    "ns/op",
		Extra:   "500000 times\n8 procs",
		Package: "github.com/user/repo",
		Procs:   8,
	})
}

func TestParseGoBenchOutput_LargeValues(t *testing.T) {
	input := `goos: linux
goarch: amd64
pkg: github.com/user/repo
BenchmarkHeavy-16               1        95258906556 ns/op
PASS
`

	results, err := ParseGoBenchOutput(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	assertResult(t, results[0], model.BenchmarkResult{
		Name:    "BenchmarkHeavy",
		Value:   95258906556,
		Unit:    "ns/op",
		Extra:   "1 times\n16 procs",
		Package: "github.com/user/repo",
		Procs:   16,
	})
}

func TestParseGoBenchOutput_MBPerSec(t *testing.T) {
	input := `goos: linux
goarch: amd64
pkg: github.com/user/repo
BenchmarkIO-8      10000        150000 ns/op      66.67 MB/s
PASS
`

	results, err := ParseGoBenchOutput(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	assertResult(t, results[0], model.BenchmarkResult{
		Name:    "BenchmarkIO",
		Value:   150000,
		Unit:    "ns/op",
		Extra:   "10000 times\n8 procs",
		Package: "github.com/user/repo",
		Procs:   8,
	})

	assertResult(t, results[1], model.BenchmarkResult{
		Name:    "BenchmarkIO - MB/s",
		Value:   66.67,
		Unit:    "MB/s",
		Extra:   "10000 times\n8 procs",
		Package: "github.com/user/repo",
		Procs:   8,
	})
}

func TestParseGoBenchOutput_WindowsLineEndings(t *testing.T) {
	input := "goos: windows\r\ngoarch: amd64\r\npkg: github.com/user/repo\r\nBenchmarkWin-8      1000000              500 ns/op\r\nPASS\r\n"

	results, err := ParseGoBenchOutput(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	assertResult(t, results[0], model.BenchmarkResult{
		Name:    "BenchmarkWin",
		Value:   500,
		Unit:    "ns/op",
		Extra:   "1000000 times\n8 procs",
		Package: "github.com/user/repo",
		Procs:   8,
	})
}

func TestParseGoBenchOutput_OnlyBenchmarkLines(t *testing.T) {
	// Sometimes users pipe only benchmark lines without header.
	input := `BenchmarkFoo-4      2000000               750 ns/op
BenchmarkBar-4      1000000              1500 ns/op
`

	results, err := ParseGoBenchOutput(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	if results[0].Name != "BenchmarkFoo" {
		t.Errorf("expected name BenchmarkFoo, got %s", results[0].Name)
	}
	if results[0].Package != "" {
		t.Errorf("expected empty package, got %s", results[0].Package)
	}
	if results[0].Procs != 4 {
		t.Errorf("expected procs 4, got %d", results[0].Procs)
	}

	if results[1].Name != "BenchmarkBar" {
		t.Errorf("expected name BenchmarkBar, got %s", results[1].Name)
	}
	if results[1].Package != "" {
		t.Errorf("expected empty package, got %s", results[1].Package)
	}
	if results[1].Procs != 4 {
		t.Errorf("expected procs 4, got %d", results[1].Procs)
	}
}

func TestParseGoBenchOutput_SinglePackageNoPrefix(t *testing.T) {
	// With a single package, names should NOT include the package prefix.
	input := `goos: linux
goarch: amd64
pkg: github.com/user/repo/mypkg
BenchmarkSingle-2      3000000               400 ns/op
PASS
`

	results, err := ParseGoBenchOutput(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].Name != "BenchmarkSingle" {
		t.Errorf("expected name without package prefix, got %s", results[0].Name)
	}
	if results[0].Package != "github.com/user/repo/mypkg" {
		t.Errorf("expected package github.com/user/repo/mypkg, got %s", results[0].Package)
	}
	if results[0].Procs != 2 {
		t.Errorf("expected procs 2, got %d", results[0].Procs)
	}
}

func TestParseGoBenchOutput_SpecialCharsInName(t *testing.T) {
	input := `goos: linux
goarch: amd64
pkg: github.com/user/repo
BenchmarkEncode/json$size=100-8       500000              3000 ns/op
PASS
`

	results, err := ParseGoBenchOutput(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].Name != "BenchmarkEncode/json$size=100" {
		t.Errorf("expected name with special chars preserved, got %s", results[0].Name)
	}
	if results[0].Package != "github.com/user/repo" {
		t.Errorf("expected package github.com/user/repo, got %s", results[0].Package)
	}
	if results[0].Procs != 8 {
		t.Errorf("expected procs 8, got %d", results[0].Procs)
	}
}

func TestParseGoBenchOutput_MultiplePackagesMultipleMetrics(t *testing.T) {
	input := `goos: linux
goarch: amd64
pkg: github.com/user/repo/pkga
BenchmarkA-4      1000000              1000 ns/op        256 B/op
pkg: github.com/user/repo/pkgb
BenchmarkB-8       500000              2000 ns/op        512 B/op
PASS
`

	results, err := ParseGoBenchOutput(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(results))
	}

	// pkga results
	assertResult(t, results[0], model.BenchmarkResult{
		Name:    "BenchmarkA",
		Value:   1000,
		Unit:    "ns/op",
		Extra:   "1000000 times\n4 procs",
		Package: "github.com/user/repo/pkga",
		Procs:   4,
	})
	assertResult(t, results[1], model.BenchmarkResult{
		Name:    "BenchmarkA - B/op",
		Value:   256,
		Unit:    "B/op",
		Extra:   "1000000 times\n4 procs",
		Package: "github.com/user/repo/pkga",
		Procs:   4,
	})

	// pkgb results
	assertResult(t, results[2], model.BenchmarkResult{
		Name:    "BenchmarkB",
		Value:   2000,
		Unit:    "ns/op",
		Extra:   "500000 times\n8 procs",
		Package: "github.com/user/repo/pkgb",
		Procs:   8,
	})
	assertResult(t, results[3], model.BenchmarkResult{
		Name:    "BenchmarkB - B/op",
		Value:   512,
		Unit:    "B/op",
		Extra:   "500000 times\n8 procs",
		Package: "github.com/user/repo/pkgb",
		Procs:   8,
	})
}

func TestParseGoBenchOutput_DifferentProcsValues(t *testing.T) {
	// Benchmarks run with different GOMAXPROCS values
	input := `goos: linux
goarch: amd64
pkg: github.com/user/repo
BenchmarkWork-1        1000000              5000 ns/op
BenchmarkWork-4         400000              1300 ns/op
BenchmarkWork-8         800000               700 ns/op
PASS
`

	results, err := ParseGoBenchOutput(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// All should have the same name but different Procs values
	for _, r := range results {
		if r.Name != "BenchmarkWork" {
			t.Errorf("expected name BenchmarkWork, got %s", r.Name)
		}
		if r.Package != "github.com/user/repo" {
			t.Errorf("expected package github.com/user/repo, got %s", r.Package)
		}
	}

	if results[0].Procs != 1 {
		t.Errorf("expected procs 1, got %d", results[0].Procs)
	}
	if results[1].Procs != 4 {
		t.Errorf("expected procs 4, got %d", results[1].Procs)
	}
	if results[2].Procs != 8 {
		t.Errorf("expected procs 8, got %d", results[2].Procs)
	}
}

func assertResult(t *testing.T, got, want model.BenchmarkResult) {
	t.Helper()
	if got.Name != want.Name {
		t.Errorf("name: got %q, want %q", got.Name, want.Name)
	}
	if got.Value != want.Value {
		t.Errorf("value for %s: got %f, want %f", want.Name, got.Value, want.Value)
	}
	if got.Unit != want.Unit {
		t.Errorf("unit for %s: got %q, want %q", want.Name, got.Unit, want.Unit)
	}
	if got.Extra != want.Extra {
		t.Errorf("extra for %s: got %q, want %q", want.Name, got.Extra, want.Extra)
	}
	if got.Package != want.Package {
		t.Errorf("package for %s: got %q, want %q", want.Name, got.Package, want.Package)
	}
	if got.Procs != want.Procs {
		t.Errorf("procs for %s: got %d, want %d", want.Name, got.Procs, want.Procs)
	}
}
