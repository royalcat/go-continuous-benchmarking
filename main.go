package main

import (
	"bufio"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/royalcat/go-continuous-benchmarking/internal/hwinfo"
	"github.com/royalcat/go-continuous-benchmarking/internal/model"
	"github.com/royalcat/go-continuous-benchmarking/internal/parse"
	"github.com/royalcat/go-continuous-benchmarking/internal/storage"
)

//go:embed frontend/*
var frontendFS embed.FS

func usage() {
	fmt.Fprintf(os.Stderr, `Usage: gobenchdata <command> [flags]

Commands:
  parse   Parse go test -bench output and save a BenchmarkEntry JSON
          along with host metadata (CPU, GOOS, GOARCH, CGO).
          Run this on each benchmark runner.

  store   Read one or more pre-parsed BenchmarkEntry JSON files,
          merge them into the branch data on gh-pages, and deploy
          the frontend. Run this once after all benchmark jobs finish.

Run "gobenchdata <command> -help" for flag details.
`)
	os.Exit(2)
}

func main() {
	log.SetFlags(0)

	if len(os.Args) < 2 {
		usage()
	}

	command := os.Args[1]
	switch command {
	case "parse":
		runParse(os.Args[2:])
	case "store":
		runStore(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
		usage()
	}
}

// ---------------------------------------------------------------------------
// parse subcommand
// ---------------------------------------------------------------------------

func runParse(args []string) {
	fs := flag.NewFlagSet("parse", flag.ExitOnError)

	var (
		outputFile   string
		resultDir    string
		commitSHA    string
		commitMsg    string
		commitAuthor string
		commitDate   string
		commitURL    string
		cpuModel     string
		cgoFlag      string
		goVersion    string
		goModule     string
		repoURL      string
	)

	fs.StringVar(&outputFile, "output-file", "", "Path to go test -bench output file (reads stdin if empty)")
	fs.StringVar(&resultDir, "result-dir", "benchmark-result", "Directory to write the parsed entry JSON and output log")
	fs.StringVar(&commitSHA, "commit-sha", "", "Commit SHA (required)")
	fs.StringVar(&commitMsg, "commit-msg", "", "Commit message")
	fs.StringVar(&commitAuthor, "commit-author", "", "Commit author")
	fs.StringVar(&commitDate, "commit-date", "", "Commit date in ISO 8601 (defaults to now)")
	fs.StringVar(&commitURL, "commit-url", "", "URL to the commit")
	fs.StringVar(&cpuModel, "cpu-model", "", "CPU model name (auto-detected if empty)")
	fs.StringVar(&cgoFlag, "cgo", "", "CGO enabled: 'true', 'false', or '' (auto-detect)")
	fs.StringVar(&goVersion, "go-version", "", "Go version string (auto-detected from runtime if empty)")
	fs.StringVar(&goModule, "go-module", "", "Go module path to strip from package names (auto-detect if empty)")
	fs.StringVar(&repoURL, "repo-url", "", "Repository URL (used for go-module fallback)")

	fs.Parse(args)

	if commitSHA == "" {
		log.Fatal("Error: -commit-sha is required")
	}

	if commitDate == "" {
		commitDate = time.Now().UTC().Format(time.RFC3339)
	}

	// --- Host metadata (auto-detect on the runner) ---

	cpu := cpuModel
	if cpu == "" {
		cpu = hwinfo.CPUModel()
		fmt.Printf("Auto-detected CPU model: %s\n", cpu)
	} else {
		fmt.Printf("Using provided CPU model: %s\n", cpu)
	}

	cgoEnabled := detectCGO(cgoFlag)
	fmt.Printf("CGO enabled: %v\n", cgoEnabled)

	goVer := goVersion
	if goVer == "" {
		goVer = runtime.Version()
		fmt.Printf("Auto-detected Go version: %s\n", goVer)
	} else {
		fmt.Printf("Using provided Go version: %s\n", goVer)
	}

	goos := runtime.GOOS
	goarch := runtime.GOARCH
	fmt.Printf("GOOS: %s, GOARCH: %s\n", goos, goarch)

	if goModule == "" {
		goModule = detectGoModule(repoURL)
		if goModule != "" {
			fmt.Printf("Auto-detected Go module: %s\n", goModule)
		}
	} else {
		fmt.Printf("Using provided Go module: %s\n", goModule)
	}

	// --- Read and parse benchmark output ---

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

	// Tee: we read once and both parse and capture raw output.
	var rawBuf strings.Builder
	tee := io.TeeReader(reader, &rawBuf)

	benchmarks, outputMeta, err := parse.ParseGoBenchOutputWithMeta(tee)
	if err != nil {
		log.Fatalf("Error parsing benchmark output: %v", err)
	}

	// If the go test output had a cpu: line and we auto-detected, prefer
	// the output's CPU (it reflects the actual benchmark machine).
	if cpuModel == "" && outputMeta.CPU != "" {
		cpu = outputMeta.CPU
		fmt.Printf("Using CPU from go test output: %s\n", cpu)
	}

	fmt.Printf("Parsed %d benchmark result(s)\n", len(benchmarks))
	for _, b := range benchmarks {
		fmt.Printf("  %s: %.4f %s\n", b.Name, b.Value, b.Unit)
	}

	// --- Build BenchmarkEntry ---

	// Parse commit date to use as the entry timestamp instead of the current time.
	commitTime, err := time.Parse(time.RFC3339, commitDate)
	if err != nil {
		log.Fatalf("Error parsing commit date %q: %v", commitDate, err)
	}

	entry := model.BenchmarkEntry{
		Commit: model.Commit{
			SHA:     commitSHA,
			Message: firstLine(commitMsg),
			Author:  commitAuthor,
			Date:    commitDate,
			URL:     commitURL,
		},
		Date: commitTime.UnixMilli(),
		Params: model.RunParams{
			CPU:       cpu,
			GOOS:      goos,
			GOARCH:    goarch,
			GoVersion: goVer,
			CGO:       cgoEnabled,
		},
		Benchmarks: benchmarks,
	}

	// --- Write results to result-dir ---

	if err := os.MkdirAll(resultDir, 0o755); err != nil {
		log.Fatalf("Error creating result directory: %v", err)
	}

	// Write entry.json
	entryJSON, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		log.Fatalf("Error marshaling entry: %v", err)
	}
	entryPath := filepath.Join(resultDir, "entry.json")
	if err := os.WriteFile(entryPath, entryJSON, 0o644); err != nil {
		log.Fatalf("Error writing entry JSON: %v", err)
	}
	fmt.Printf("Wrote parsed entry to %s\n", entryPath)

	// Write output.log (raw benchmark output for debugging)
	logPath := filepath.Join(resultDir, "output.log")
	if err := os.WriteFile(logPath, []byte(rawBuf.String()), 0o644); err != nil {
		log.Fatalf("Error writing output log: %v", err)
	}
	fmt.Printf("Wrote raw output to %s\n", logPath)

	// Generate a unique artifact name from run parameters so that matrix
	// jobs never collide when uploading artifacts.
	artifactName := artifactNameFromParams(entry.Params)
	fmt.Printf("artifact-name: %s\n", artifactName)
}

// artifactNameFromParams builds a unique, filesystem-safe artifact name
// from the run parameters.  Example: "bench-linux-amd64-go1.24.0-cgo1"
func artifactNameFromParams(p model.RunParams) string {
	cgoVal := "0"
	if p.CGO {
		cgoVal = "1"
	}

	parts := []string{"bench"}

	if p.GOOS != "" {
		parts = append(parts, p.GOOS)
	}
	if p.GOARCH != "" {
		parts = append(parts, p.GOARCH)
	}
	if p.GoVersion != "" {
		parts = append(parts, p.GoVersion)
	}

	parts = append(parts, "cgo"+cgoVal)

	return strings.Join(parts, "-")
}

// ---------------------------------------------------------------------------
// store subcommand
// ---------------------------------------------------------------------------

func runStore(args []string) {
	fs := flag.NewFlagSet("store", flag.ExitOnError)

	var (
		entriesGlob string
		branch      string
		dataDir     string
		maxItems    int
		repoURL     string
		goModule    string
	)

	fs.StringVar(&entriesGlob, "entries", "", "Glob or comma-separated paths to entry.json files (required)")
	fs.StringVar(&branch, "branch", "main", "Git branch name")
	fs.StringVar(&dataDir, "data-dir", "benchmarks", "Directory to store benchmark data and frontend files")
	fs.IntVar(&maxItems, "max-items", 0, "Maximum number of benchmark entries per branch (0 = unlimited)")
	fs.StringVar(&repoURL, "repo-url", "", "Repository URL for the frontend header")
	fs.StringVar(&goModule, "go-module", "", "Go module path for the frontend")

	fs.Parse(args)

	if entriesGlob == "" {
		log.Fatal("Error: -entries is required")
	}

	// Detect Go module if not provided.
	if goModule == "" {
		goModule = detectGoModule(repoURL)
		if goModule != "" {
			fmt.Printf("Auto-detected Go module: %s\n", goModule)
		}
	} else {
		fmt.Printf("Using provided Go module: %s\n", goModule)
	}

	// Resolve entry files.
	entryFiles := resolveFiles(entriesGlob)
	if len(entryFiles) == 0 {
		log.Fatal("Error: no entry files matched")
	}

	fmt.Printf("Found %d entry file(s):\n", len(entryFiles))
	for _, f := range entryFiles {
		fmt.Printf("  %s\n", f)
	}

	// Load all entries.
	var entries []model.BenchmarkEntry
	for _, path := range entryFiles {
		entry, err := loadEntry(path)
		if err != nil {
			log.Fatalf("Error loading entry from %s: %v", path, err)
		}
		fmt.Printf("Loaded entry from %s: CPU=%s GOOS=%s GOARCH=%s GoVersion=%s CGO=%v benchmarks=%d\n",
			path, entry.Params.CPU, entry.Params.GOOS, entry.Params.GOARCH, entry.Params.GoVersion, entry.Params.CGO, len(entry.Benchmarks))
		entries = append(entries, entry)
	}

	// Initialize storage.
	store, err := storage.New(dataDir)
	if err != nil {
		log.Fatalf("Error initializing storage: %v", err)
	}

	// Append all entries in a single batch.
	if err := store.AppendEntries(branch, entries, maxItems); err != nil {
		log.Fatalf("Error appending entries: %v", err)
	}

	commitSHA := ""
	if len(entries) > 0 {
		commitSHA = entries[0].Commit.SHA
	}
	shortSHA := commitSHA
	if len(shortSHA) > 7 {
		shortSHA = shortSHA[:7]
	}

	fmt.Printf("Stored %d entry/entries for branch %q (commit %s)\n", len(entries), branch, shortSHA)

	// Write repo-level metadata for the frontend.
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

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// loadEntry reads a BenchmarkEntry from a JSON file.
func loadEntry(path string) (model.BenchmarkEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return model.BenchmarkEntry{}, fmt.Errorf("reading %s: %w", path, err)
	}
	var entry model.BenchmarkEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return model.BenchmarkEntry{}, fmt.Errorf("decoding %s: %w", path, err)
	}
	return entry, nil
}

// resolveFiles expands a raw string (comma-separated, newline-separated,
// with optional glob patterns) into a list of file paths.
func resolveFiles(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	var segments []string
	for _, part := range strings.Split(raw, ",") {
		for _, seg := range strings.Split(part, "\n") {
			seg = strings.TrimSpace(seg)
			if seg != "" {
				segments = append(segments, seg)
			}
		}
	}

	var files []string
	for _, seg := range segments {
		matches, err := filepath.Glob(seg)
		if err != nil {
			files = append(files, seg)
			continue
		}
		if len(matches) == 0 {
			files = append(files, seg)
		} else {
			files = append(files, matches...)
		}
	}
	return files
}

// deployFrontend copies the embedded frontend files into the data directory.
func deployFrontend(dataDir string) error {
	names := []string{"index.html", "app.js"}
	for _, name := range names {
		content, err := frontendFS.ReadFile("frontend/" + name)
		if err != nil {
			return fmt.Errorf("reading embedded file %s: %w", name, err)
		}
		dest := filepath.Join(dataDir, name)
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

// detectGoModule tries to find the Go module path from go.mod or the repo URL.
func detectGoModule(repoURL string) string {
	if mod := parseGoMod("go.mod"); mod != "" {
		return mod
	}

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

// parseGoMod reads a go.mod file and extracts the module path.
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
// Explicit flag value > CGO_ENABLED env var > default true.
func detectCGO(flagVal string) bool {
	flagVal = strings.TrimSpace(strings.ToLower(flagVal))
	switch flagVal {
	case "true", "1":
		return true
	case "false", "0":
		return false
	default:
		env := os.Getenv("CGO_ENABLED")
		switch strings.TrimSpace(env) {
		case "0":
			return false
		default:
			return true
		}
	}
}
