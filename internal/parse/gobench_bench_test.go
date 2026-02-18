package parse

import (
	"fmt"
	"strings"
	"testing"
)

// buildOutput generates a synthetic go test -bench output with n benchmark lines.
// Each line has 3 metrics: ns/op, B/op, allocs/op.
func buildOutput(nBenchmarks int, nPackages int) string {
	var sb strings.Builder
	sb.WriteString("goos: linux\n")
	sb.WriteString("goarch: amd64\n")
	sb.WriteString("cpu: Intel(R) Core(TM) i7-8700 CPU @ 3.20GHz\n")

	benchesPerPkg := nBenchmarks / max(nPackages, 1)
	if benchesPerPkg < 1 {
		benchesPerPkg = 1
	}

	for p := 0; p < max(nPackages, 1); p++ {
		if nPackages > 0 {
			fmt.Fprintf(&sb, "pkg: github.com/test/repo/pkg%d\n", p)
		}
		for i := 0; i < benchesPerPkg; i++ {
			idx := p*benchesPerPkg + i
			fmt.Fprintf(&sb,
				"BenchmarkFunc%d-8\t%d\t\t%d ns/op\t\t%d B/op\t\t%d allocs/op\n",
				idx,
				1000000/(idx+1),
				500+idx*13,
				64+idx*8,
				1+idx%10,
			)
		}
	}

	sb.WriteString("PASS\n")
	sb.WriteString("ok\tgithub.com/test/repo\t1.234s\n")
	return sb.String()
}

func BenchmarkParse_SingleLine(b *testing.B) {
	input := "pkg: github.com/test/repo\nBenchmarkFoo-8\t\t1000000\t\t1234 ns/op\n"
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		_, err := ParseGoBenchOutput(strings.NewReader(input))
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParse_SingleLineMultiMetric(b *testing.B) {
	input := "pkg: github.com/test/repo\nBenchmarkFoo-8\t\t1000000\t\t1234 ns/op\t\t256 B/op\t\t5 allocs/op\n"
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		_, err := ParseGoBenchOutput(strings.NewReader(input))
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParse_10Lines(b *testing.B) {
	input := buildOutput(10, 1)
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		_, err := ParseGoBenchOutput(strings.NewReader(input))
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParse_100Lines(b *testing.B) {
	input := buildOutput(100, 1)
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		_, err := ParseGoBenchOutput(strings.NewReader(input))
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParse_1000Lines(b *testing.B) {
	input := buildOutput(1000, 1)
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		_, err := ParseGoBenchOutput(strings.NewReader(input))
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParse_MultiPackage_10x10(b *testing.B) {
	input := buildOutput(100, 10)
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		_, err := ParseGoBenchOutput(strings.NewReader(input))
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParse_SubBenchmarks(b *testing.B) {
	var sb strings.Builder
	sb.WriteString("pkg: github.com/test/repo\n")
	for i := 0; i < 50; i++ {
		for j := 0; j < 5; j++ {
			fmt.Fprintf(&sb,
				"BenchmarkGroup%d/Case%d-8\t\t%d\t\t%d ns/op\t\t%d B/op\n",
				i, j, 100000/(j+1), 200+i*10+j, 32+j*16,
			)
		}
	}
	input := sb.String()

	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		_, err := ParseGoBenchOutput(strings.NewReader(input))
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParse_WithNoise(b *testing.B) {
	// Simulates realistic output with lots of non-benchmark lines (test logs, etc.)
	var sb strings.Builder
	sb.WriteString("goos: linux\n")
	sb.WriteString("goarch: amd64\n")
	sb.WriteString("pkg: github.com/test/repo\n")
	sb.WriteString("cpu: Intel(R) Core(TM) i7-8700 CPU @ 3.20GHz\n")
	for i := 0; i < 200; i++ {
		// Intersperse log lines and benchmark lines
		if i%5 == 0 {
			fmt.Fprintf(&sb, "BenchmarkWork%d-8\t\t%d\t\t%d ns/op\t\t%d B/op\t\t%d allocs/op\n",
				i/5, 500000/(i/5+1), 1000+i*7, 128+i, 2+i%8)
		} else {
			fmt.Fprintf(&sb, "    some test log output line %d: status=ok elapsed=0.%03ds\n", i, i)
		}
	}
	sb.WriteString("PASS\n")
	sb.WriteString("ok\tgithub.com/test/repo\t4.567s\n")
	input := sb.String()

	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		_, err := ParseGoBenchOutput(strings.NewReader(input))
		if err != nil {
			b.Fatal(err)
		}
	}
}
