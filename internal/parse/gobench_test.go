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
		Name:  "BenchmarkFib10",
		Value: 456.7,
		Unit:  "ns/op",
		Extra: "3000000 times\n12 procs",
	})

	assertResult(t, results[1], model.BenchmarkResult{
		Name:  "BenchmarkFib20",
		Value: 46573.2,
		Unit:  "ns/op",
		Extra: "30000 times\n12 procs",
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
		Name:  "BenchmarkAlloc",
		Value: 15000,
		Unit:  "ns/op",
		Extra: "10000 times\n8 procs",
	})

	assertResult(t, results[1], model.BenchmarkResult{
		Name:  "BenchmarkAlloc - B/op",
		Value: 1024,
		Unit:  "B/op",
		Extra: "10000 times\n8 procs",
	})

	assertResult(t, results[2], model.BenchmarkResult{
		Name:  "BenchmarkAlloc - allocs/op",
		Value: 5,
		Unit:  "allocs/op",
		Extra: "10000 times\n8 procs",
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
		Name:  "BenchmarkSimple",
		Value: 300.0,
		Unit:  "ns/op",
		Extra: "5000000 times",
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

	// With multiple packages, names should include package.
	assertResult(t, results[0], model.BenchmarkResult{
		Name:  "BenchmarkA (github.com/user/repo/pkga)",
		Value: 1000,
		Unit:  "ns/op",
		Extra: "1000000 times\n4 procs",
	})

	assertResult(t, results[1], model.BenchmarkResult{
		Name:  "BenchmarkB (github.com/user/repo/pkgb)",
		Value: 2000,
		Unit:  "ns/op",
		Extra: "500000 times\n4 procs",
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
		Name:  "BenchmarkParent/SubCase1",
		Value: 1100,
		Unit:  "ns/op",
		Extra: "1000000 times\n8 procs",
	})

	assertResult(t, results[1], model.BenchmarkResult{
		Name:  "BenchmarkParent/SubCase2",
		Value: 2200,
		Unit:  "ns/op",
		Extra: "500000 times\n8 procs",
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
		Name:  "BenchmarkHeavy",
		Value: 95258906556,
		Unit:  "ns/op",
		Extra: "1 times\n16 procs",
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
		Name:  "BenchmarkIO",
		Value: 150000,
		Unit:  "ns/op",
		Extra: "10000 times\n8 procs",
	})

	assertResult(t, results[1], model.BenchmarkResult{
		Name:  "BenchmarkIO - MB/s",
		Value: 66.67,
		Unit:  "MB/s",
		Extra: "10000 times\n8 procs",
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
		Name:  "BenchmarkWin",
		Value: 500,
		Unit:  "ns/op",
		Extra: "1000000 times\n8 procs",
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
	if results[1].Name != "BenchmarkBar" {
		t.Errorf("expected name BenchmarkBar, got %s", results[1].Name)
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
}
