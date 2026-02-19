package main

import (
	"bufio"
	"embed"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/royalcat/go-continuous-benchmarking/internal/hwinfo"
	"github.com/royalcat/go-continuous-benchmarking/internal/model"
	"github.com/royalcat/go-continuous-benchmarking/internal/parse"
	"github.com/royalcat/go-continuous-benchmarking/internal/storage"
)

//go:embed frontend/*
var frontendFS embed.FS

func main() {
	var (
		outputFile   string
		branch       string
		dataDir      string
		commitSHA    string
		commitMsg    string
		commitAuthor string
		commitDate   string
		commitURL    string
		maxItems     int
		repoURL      string
		cpuModel     string
		cgoFlag      string
		goModule     string
	)

	flag.StringVar(&outputFile, "output-file", "", "Path(s) to go test -bench output file(s). Supports glob patterns and comma-separated paths. Reads stdin if empty.")
	flag.StringVar(&branch, "branch", "main", "Git branch name")
	flag.StringVar(&dataDir, "data-dir", "dev/bench", "Directory to store benchmark data and frontend files")
	flag.StringVar(&commitSHA, "commit-sha", "", "Commit SHA (required)")
	flag.StringVar(&commitMsg, "commit-msg", "", "Commit message")
	flag.StringVar(&commitAuthor, "commit-author", "", "Commit author username")
	flag.StringVar(&commitDate, "commit-date", "", "Commit date in ISO 8601 format (defaults to now)")
	flag.StringVar(&commitURL, "commit-url", "", "URL to the commit on GitHub")
	flag.IntVar(&maxItems, "max-items", 0, "Maximum number of benchmark entries per branch (0 = unlimited)")
	flag.StringVar(&repoURL, "repo-url", "", "Repository URL for display in the frontend header")
	flag.StringVar(&cpuModel, "cpu-model", "", "CPU model name to record. If empty, auto-detected from the current machine")
	flag.StringVar(&cgoFlag, "cgo", "", "CGO enabled status: 'true', 'false', or '' (auto-detect from CGO_ENABLED env var)")
	flag.StringVar(&goModule, "go-module", "", "Go module path to strip from package names in the dashboard. If empty, auto-detected from go.mod")

	flag.Parse()

	if commitSHA == "" {
		log.Fatal("Error: -commit-sha is required")
	}

	// Default commit date to now.
	if commitDate == "" {
		commitDate = time.Now().UTC().Format(time.RFC3339)
	}

	// Detect CGO status (used as default for files without sidecar metadata).
	defaultCGO := detectCGO(cgoFlag)

	// Auto-detect CPU model if not explicitly provided (used as default fallback).
	defaultCPU := cpuModel
	if defaultCPU == "" {
		defaultCPU = hwinfo.CPUModel()
		fmt.Printf("Auto-detected CPU model: %s\n", defaultCPU)
	} else {
		fmt.Printf("Using provided CPU model: %s\n", defaultCPU)
	}

	fmt.Printf("Default CGO enabled: %v\n", defaultCGO)

	// Detect Go module path if not explicitly provided.
	if goModule == "" {
		goModule = detectGoModule(repoURL)
		if goModule != "" {
			fmt.Printf("Auto-detected Go module: %s\n", goModule)
		}
	} else {
		fmt.Printf("Using provided Go module: %s\n", goModule)
	}

	// Build the commit info (shared across all entries).
	commit := model.Commit{
		SHA:     commitSHA,
		Message: firstLine(commitMsg),
		Author:  commitAuthor,
		Date:    commitDate,
		URL:     commitURL,
	}

	// Resolve output files.
	outputFiles := resolveOutputFiles(outputFile)

	var entries []model.BenchmarkEntry

	if len(outputFiles) == 0 {
		// No files specified — read from stdin (single entry mode).
		benchmarks, err := parse.ParseGoBenchOutput(os.Stdin)
		if err != nil {
			log.Fatalf("Error parsing benchmark output from stdin: %v", err)
		}

		fmt.Printf("Parsed %d benchmark result(s) from stdin\n", len(benchmarks))
		for _, b := range benchmarks {
			fmt.Printf("  %s: %.4f %s\n", b.Name, b.Value, b.Unit)
		}

		entries = append(entries, model.BenchmarkEntry{
			Commit:     commit,
			Date:       time.Now().UnixMilli(),
			CPU:        defaultCPU,
			CGO:        defaultCGO,
			Benchmarks: benchmarks,
		})
	} else {
		// Process each output file with its optional sidecar metadata.
		for _, filePath := range outputFiles {
			entry, err := processOutputFile(filePath, commit, defaultCPU, defaultCGO)
			if err != nil {
				log.Fatalf("Error processing %s: %v", filePath, err)
			}
			entries = append(entries, entry)
		}
	}

	// Initialize storage.
	store, err := storage.New(dataDir)
	if err != nil {
		log.Fatalf("Error initializing storage: %v", err)
	}

	// Append all entries in a single batch operation.
	if err := store.AppendEntries(branch, entries, maxItems); err != nil {
		log.Fatalf("Error appending benchmark entries: %v", err)
	}

	fmt.Printf("Stored %d benchmark entry/entries for branch %q (commit %s)\n",
		len(entries), branch, commitSHA[:minInt(7, len(commitSHA))])

	// Write repo URL metadata file for the frontend.
	if repoURL != "" || goModule != "" {
		if err := store.WriteMetadata(repoURL, goModule); err != nil {
			log.Fatalf("Error writing metadata: %v", err)
		}
	}

	// Deploy frontend static files.
	if err := deployFrontend(dataDir); err != nil {
		log.Fatalf("Error deploying frontend: %v", err)
	}

	fmt.Println("Frontend files deployed successfully")
}

// processOutputFile parses a single benchmark output file and creates a
// BenchmarkEntry. It looks for a sidecar metadata file in the same directory
// (named "metadata.json") to obtain per-file CPU model and CGO status. If the
// sidecar is not found, it falls back to extracting the CPU from the go test
// output headers, then to the provided defaults.
func processOutputFile(filePath string, commit model.Commit, defaultCPU string, defaultCGO bool) (model.BenchmarkEntry, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return model.BenchmarkEntry{}, fmt.Errorf("opening output file: %w", err)
	}
	defer f.Close()

	// Parse benchmark results and extract output metadata (cpu: line).
	benchmarks, outputMeta, err := parse.ParseGoBenchOutputWithMeta(f)
	if err != nil {
		return model.BenchmarkEntry{}, fmt.Errorf("parsing benchmark output: %w", err)
	}

	fmt.Printf("Parsed %d benchmark result(s) from %s\n", len(benchmarks), filePath)
	for _, b := range benchmarks {
		fmt.Printf("  %s: %.4f %s\n", b.Name, b.Value, b.Unit)
	}

	// Load sidecar metadata if available.
	// Look for metadata.json in the same directory as the output file.
	dir := filepath.Dir(filePath)
	sidecar, err := storage.LoadFileMetadata(filepath.Join(dir, "metadata.json"))
	if err != nil {
		return model.BenchmarkEntry{}, fmt.Errorf("loading sidecar metadata: %w", err)
	}

	// Resolve CPU: sidecar > output header > default.
	cpu := defaultCPU
	if outputMeta.CPU != "" {
		cpu = outputMeta.CPU
	}
	if sidecar.CPU != "" {
		cpu = sidecar.CPU
	}

	// Resolve CGO: sidecar > default.
	cgo := defaultCGO
	if sidecar.CGO != nil {
		cgo = *sidecar.CGO
	}

	fmt.Printf("  CPU: %s, CGO: %v\n", cpu, cgo)

	return model.BenchmarkEntry{
		Commit:     commit,
		Date:       time.Now().UnixMilli(),
		CPU:        cpu,
		CGO:        cgo,
		Benchmarks: benchmarks,
	}, nil
}

// resolveOutputFiles takes the raw -output-file flag value and expands it into
// a list of concrete file paths. It supports:
//   - Empty string → returns nil (stdin mode)
//   - Comma-separated paths (e.g. "a.txt,b.txt")
//   - Newline-separated paths (e.g. from a multiline GitHub Actions input)
//   - Glob patterns in each segment (e.g. "results/*/output.txt")
func resolveOutputFiles(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	// Split by comma and newline.
	var segments []string
	for _, part := range strings.Split(raw, ",") {
		for _, seg := range strings.Split(part, "\n") {
			seg = strings.TrimSpace(seg)
			if seg != "" {
				segments = append(segments, seg)
			}
		}
	}

	// Expand globs.
	var files []string
	for _, seg := range segments {
		matches, err := filepath.Glob(seg)
		if err != nil {
			// If glob is invalid, treat it as a literal path.
			files = append(files, seg)
			continue
		}
		if len(matches) == 0 {
			// No matches — keep as literal so we get a clear "file not found" error later.
			files = append(files, seg)
		} else {
			files = append(files, matches...)
		}
	}

	return files
}

// deployFrontend copies the embedded frontend files into the data directory.
func deployFrontend(dataDir string) error {
	files := []string{"index.html", "app.js"}
	for _, name := range files {
		content, err := frontendFS.ReadFile("frontend/" + name)
		if err != nil {
			return fmt.Errorf("reading embedded file %s: %w", name, err)
		}
		dest := dataDir + "/" + name
		if err := os.WriteFile(dest, content, 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", dest, err)
		}
	}
	return nil
}

// firstLine returns the first line of s.
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// detectGoModule tries to find the Go module path.
// It first tries parsing go.mod in the current directory, then falls back
// to deriving the module path from the repo URL.
func detectGoModule(repoURL string) string {
	// Try go.mod in current directory
	if mod := parseGoMod("go.mod"); mod != "" {
		return mod
	}

	// Fall back to repo URL: "https://github.com/user/repo" -> "github.com/user/repo"
	if repoURL != "" {
		trimmed := strings.TrimSuffix(repoURL, "/")
		trimmed = strings.TrimSuffix(trimmed, ".git")
		for _, prefix := range []string{"https://", "http://"} {
			if strings.HasPrefix(trimmed, prefix) {
				return strings.TrimPrefix(trimmed, prefix)
			}
		}
	}

	return ""
}

// parseGoMod reads a go.mod file and extracts the module path from the
// "module" directive. Returns empty string if the file doesn't exist or
// can't be parsed.
func parseGoMod(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return ""
}

// detectCGO determines CGO enabled status.
// If flagVal is "true"/"false", use that. Otherwise auto-detect from CGO_ENABLED env var.
// If env var is also unset, defaults to true (Go's default behavior).
func detectCGO(flagVal string) bool {
	flagVal = strings.TrimSpace(strings.ToLower(flagVal))
	switch flagVal {
	case "true", "1":
		return true
	case "false", "0":
		return false
	default:
		// Auto-detect from environment
		env := os.Getenv("CGO_ENABLED")
		switch strings.TrimSpace(env) {
		case "0":
			return false
		default:
			// CGO is enabled by default in Go when not cross-compiling
			return true
		}
	}
}
