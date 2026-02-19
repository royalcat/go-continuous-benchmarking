package parse

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/royalcat/go-continuous-benchmarking/internal/model"
)

// reGoBench matches Go benchmark result lines.
// Format: BenchmarkName-PROCS  iterations  value unit [value unit ...]
// Reference: https://go.googlesource.com/proposal/+/master/design/14313-benchmark-format.md
var reGoBench = regexp.MustCompile(
	`^(?P<name>Benchmark\S+?)(?:-(?P<procs>\d+))?\s+(?P<iters>\d+)\s+(?P<rest>.+)$`,
)

// rePkgLine matches the "pkg: ..." line that precedes benchmark output for a package.
var rePkgLine = regexp.MustCompile(`^pkg:\s+(\S+)`)

// reCPULine matches the "cpu: ..." line emitted by go test.
var reCPULine = regexp.MustCompile(`^cpu:\s+(.+)$`)

// OutputMetadata contains metadata extracted from go test benchmark output headers.
type OutputMetadata struct {
	// CPU is the CPU model string extracted from the "cpu: ..." line.
	// Empty if the line was not present in the output.
	CPU string
}

// ParseGoBenchOutput parses the output of `go test -bench` and returns a slice
// of BenchmarkResult. It handles multiple packages, multiple metrics per benchmark,
// and the standard Go benchmark output format.
func ParseGoBenchOutput(r io.Reader) ([]model.BenchmarkResult, error) {
	results, _, err := ParseGoBenchOutputWithMeta(r)
	return results, err
}

// ParseGoBenchOutputWithMeta parses the output of `go test -bench` and returns
// both the benchmark results and any metadata extracted from the output headers
// (such as the CPU model from the "cpu: ..." line).
func ParseGoBenchOutputWithMeta(r io.Reader) ([]model.BenchmarkResult, OutputMetadata, error) {
	scanner := bufio.NewScanner(r)

	var results []model.BenchmarkResult
	var meta OutputMetadata
	var currentPkg string

	// First pass: collect all lines.
	var lines []string
	for scanner.Scan() {
		line := scanner.Text()
		lines = append(lines, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, meta, fmt.Errorf("reading benchmark output: %w", err)
	}

	for _, line := range lines {
		// Strip Windows-style carriage returns.
		line = strings.TrimRight(line, "\r")

		// Track current package.
		if m := rePkgLine.FindStringSubmatch(line); m != nil {
			currentPkg = m[1]
			continue
		}

		// Extract CPU metadata from the "cpu: ..." header line.
		if m := reCPULine.FindStringSubmatch(line); m != nil {
			if meta.CPU == "" {
				meta.CPU = strings.TrimSpace(m[1])
			}
			continue
		}

		m := reGoBench.FindStringSubmatch(line)
		if m == nil {
			continue
		}

		name := m[1]
		procsStr := m[2]
		iters := m[3]
		rest := m[4]

		procs := 1
		if procsStr != "" {
			procs, _ = strconv.Atoi(procsStr)
		}

		extra := iters + " times"
		if procs > 0 {
			extra += "\n" + strconv.Itoa(procs) + " procs"
		}

		// Parse value/unit pairs from the remainder.
		// The remainder looks like: "41653 ns/op  128 B/op  2 allocs/op"
		fields := strings.Fields(rest)
		if len(fields) < 2 || len(fields)%2 != 0 {
			continue // malformed line, skip
		}

		pairs := make([][2]string, 0, len(fields)/2)
		for i := 0; i < len(fields); i += 2 {
			pairs = append(pairs, [2]string{fields[i], fields[i+1]})
		}

		for i, pair := range pairs {
			val, err := strconv.ParseFloat(pair[0], 64)
			if err != nil {
				continue // skip unparseable values
			}
			unit := pair[1]

			resultName := name
			if i > 0 {
				resultName = name + " - " + unit
			}

			results = append(results, model.BenchmarkResult{
				Name:    resultName,
				Value:   val,
				Unit:    unit,
				Extra:   extra,
				Package: currentPkg,
				Procs:   procs,
			})
		}
	}

	if len(results) == 0 {
		return nil, meta, fmt.Errorf("no benchmark results found in output")
	}

	return results, meta, nil
}
