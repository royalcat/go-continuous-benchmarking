package main

import (
	"embed"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
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
	)

	flag.StringVar(&outputFile, "output-file", "", "Path to go test -bench output file (reads stdin if empty)")
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

	flag.Parse()

	if commitSHA == "" {
		log.Fatal("Error: -commit-sha is required")
	}

	// Default commit date to now.
	if commitDate == "" {
		commitDate = time.Now().UTC().Format(time.RFC3339)
	}

	// Auto-detect CPU model if not explicitly provided.
	if cpuModel == "" {
		cpuModel = hwinfo.CPUModel()
		fmt.Printf("Auto-detected CPU model: %s\n", cpuModel)
	} else {
		fmt.Printf("Using provided CPU model: %s\n", cpuModel)
	}

	// Detect CGO status: explicit flag > CGO_ENABLED env var.
	cgoEnabled := detectCGO(cgoFlag)
	fmt.Printf("CGO enabled: %v\n", cgoEnabled)

	// Read benchmark output.
	var reader io.Reader
	if outputFile != "" {
		f, err := os.Open(outputFile)
		if err != nil {
			log.Fatalf("Error opening output file: %v", err)
		}
		defer f.Close()
		reader = f
	} else {
		reader = os.Stdin
	}

	// Parse benchmark results.
	benchmarks, err := parse.ParseGoBenchOutput(reader)
	if err != nil {
		log.Fatalf("Error parsing benchmark output: %v", err)
	}

	fmt.Printf("Parsed %d benchmark result(s)\n", len(benchmarks))
	for _, b := range benchmarks {
		fmt.Printf("  %s: %.4f %s\n", b.Name, b.Value, b.Unit)
	}

	// Build the benchmark entry.
	entry := model.BenchmarkEntry{
		Commit: model.Commit{
			SHA:     commitSHA,
			Message: firstLine(commitMsg),
			Author:  commitAuthor,
			Date:    commitDate,
			URL:     commitURL,
		},
		Date:       time.Now().UnixMilli(),
		CPU:        cpuModel,
		CGO:        cgoEnabled,
		Benchmarks: benchmarks,
	}

	// Initialize storage.
	store, err := storage.New(dataDir)
	if err != nil {
		log.Fatalf("Error initializing storage: %v", err)
	}

	// Append entry to branch data.
	if err := store.AppendEntry(branch, entry, maxItems); err != nil {
		log.Fatalf("Error appending benchmark entry: %v", err)
	}

	fmt.Printf("Stored benchmark data for branch %q (commit %s)\n", branch, commitSHA[:minInt(7, len(commitSHA))])

	// Write repo URL metadata file for the frontend.
	if repoURL != "" {
		if err := store.WriteMetadata(repoURL); err != nil {
			log.Fatalf("Error writing metadata: %v", err)
		}
	}

	// Deploy frontend static files.
	if err := deployFrontend(dataDir); err != nil {
		log.Fatalf("Error deploying frontend: %v", err)
	}

	fmt.Println("Frontend files deployed successfully")
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
