package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/royalcat/go-continuous-benchmarking/internal/model"
)

// sortByCommitDate sorts entries by their Commit.Date field (RFC 3339 string).
// Entries with unparseable dates are placed at the beginning.
func sortByCommitDate(entries model.BranchData) {
	sort.SliceStable(entries, func(i, j int) bool {
		ti, erri := time.Parse(time.RFC3339, entries[i].Commit.Date)
		tj, errj := time.Parse(time.RFC3339, entries[j].Commit.Date)
		if erri != nil || errj != nil {
			// Fall back to the Date (unix millis) field when parsing fails.
			return entries[i].Date < entries[j].Date
		}
		return ti.Before(tj)
	})
}

// Storage manages benchmark data files on disk.
// The layout on disk is:
//
//	<baseDir>/
//	  branches.json          – JSON array of branch name strings
//	  data/
//	    <branch>.json        – JSON array of BenchmarkEntry per branch
type Storage struct {
	baseDir string
}

// New creates a Storage rooted at baseDir.
// It ensures the base directory and the data/ subdirectory exist.
func New(baseDir string) (*Storage, error) {
	dataDir := filepath.Join(baseDir, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating data directory: %w", err)
	}
	return &Storage{baseDir: baseDir}, nil
}

// branchesPath returns the path to branches.json.
func (s *Storage) branchesPath() string {
	return filepath.Join(s.baseDir, "branches.json")
}

// branchDataPath returns the path to data/<branch>.json.
// Branch names are sanitised so they are safe as file names: slashes are
// replaced with double underscores.
func (s *Storage) branchDataPath(branch string) string {
	safe := sanitizeBranchName(branch)
	return filepath.Join(s.baseDir, "data", safe+".json")
}

// sanitizeBranchName replaces characters that are problematic in file names.
func sanitizeBranchName(branch string) string {
	replacer := func(r rune) rune {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			return '_'
		}
		return r
	}
	out := []rune(branch)
	for i, r := range out {
		out[i] = replacer(r)
	}
	return string(out)
}

// --------------------------------------------------------------------------
// Branch list operations
// --------------------------------------------------------------------------

// ReadBranches reads the branch list from branches.json.
// If the file does not exist an empty slice is returned.
func (s *Storage) ReadBranches() ([]string, error) {
	data, err := os.ReadFile(s.branchesPath())
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading branches file: %w", err)
	}

	var branches []string
	if err := json.Unmarshal(data, &branches); err != nil {
		return nil, fmt.Errorf("decoding branches file: %w", err)
	}
	return branches, nil
}

// WriteBranches writes the branch list to branches.json.
func (s *Storage) WriteBranches(branches []string) error {
	data, err := json.MarshalIndent(branches, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding branches: %w", err)
	}
	if err := os.WriteFile(s.branchesPath(), data, 0o644); err != nil {
		return fmt.Errorf("writing branches file: %w", err)
	}
	return nil
}

// EnsureBranch adds branch to the branch list if it is not already present.
// It returns true if the branch was newly added.
func (s *Storage) EnsureBranch(branch string) (bool, error) {
	branches, err := s.ReadBranches()
	if err != nil {
		return false, err
	}

	for _, b := range branches {
		if b == branch {
			return false, nil
		}
	}

	branches = append(branches, branch)
	sort.Strings(branches)

	if err := s.WriteBranches(branches); err != nil {
		return false, err
	}
	return true, nil
}

// --------------------------------------------------------------------------
// Branch data operations
// --------------------------------------------------------------------------

// ReadBranchData reads the benchmark entries for a branch.
// If the file does not exist an empty slice is returned.
func (s *Storage) ReadBranchData(branch string) (model.BranchData, error) {
	data, err := os.ReadFile(s.branchDataPath(branch))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading branch data for %q: %w", branch, err)
	}

	var entries model.BranchData
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("decoding branch data for %q: %w", branch, err)
	}
	return entries, nil
}

// WriteBranchData writes benchmark entries for a branch to disk.
func (s *Storage) WriteBranchData(branch string, entries model.BranchData) error {
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding branch data: %w", err)
	}
	if err := os.WriteFile(s.branchDataPath(branch), data, 0o644); err != nil {
		return fmt.Errorf("writing branch data for %q: %w", branch, err)
	}
	return nil
}

// AppendEntry adds a new benchmark entry for the given branch, persists it,
// and ensures the branch is registered in branches.json.
//
// If maxItems > 0, the oldest entries are trimmed so that at most maxItems
// entries remain per branch.
func (s *Storage) AppendEntry(branch string, entry model.BenchmarkEntry, maxItems int) error {
	return s.AppendEntries(branch, []model.BenchmarkEntry{entry}, maxItems)
}

// AppendEntries adds multiple benchmark entries for the given branch in a single
// read-modify-write cycle. This is more efficient than calling AppendEntry in a
// loop when processing multiple output files (e.g. from a matrix build).
//
// Entries are keyed by (commit SHA, CPU model, CGO status). If a new entry
// has the same key as an existing one, the old entry is replaced. After
// merging, entries are sorted by commit date.
//
// If maxItems > 0, the oldest entries are trimmed so that at most maxItems
// entries remain per branch after all new entries have been appended.
func (s *Storage) AppendEntries(branch string, newEntries []model.BenchmarkEntry, maxItems int) error {
	if len(newEntries) == 0 {
		return nil
	}

	// Register the branch in the branch list.
	if _, err := s.EnsureBranch(branch); err != nil {
		return fmt.Errorf("ensuring branch %q: %w", branch, err)
	}

	// Read existing data.
	entries, err := s.ReadBranchData(branch)
	if err != nil {
		return err
	}

	// Build a set of new entry keys for fast lookup.
	newKeys := make(map[string]struct{}, len(newEntries))
	for _, e := range newEntries {
		newKeys[e.EntryKey()] = struct{}{}
	}

	// Remove existing entries whose key matches a new entry (replace semantics).
	filtered := entries[:0]
	for _, e := range entries {
		if _, dup := newKeys[e.EntryKey()]; !dup {
			filtered = append(filtered, e)
		}
	}

	// Append all new entries.
	filtered = append(filtered, newEntries...)

	// Sort by commit date so the timeline is always chronological.
	sortByCommitDate(filtered)

	// Trim old entries if maxItems is set.
	if maxItems > 0 && len(filtered) > maxItems {
		filtered = filtered[len(filtered)-maxItems:]
	}

	return s.WriteBranchData(branch, filtered)
}

// --------------------------------------------------------------------------
// Metadata operations
// --------------------------------------------------------------------------

// Metadata holds repository-level information displayed by the frontend.
type Metadata struct {
	RepoURL    string `json:"repoUrl"`
	LastUpdate int64  `json:"lastUpdate"`
	GoModule   string `json:"goModule,omitempty"`
}

// metadataPath returns the path to metadata.json.
func (s *Storage) metadataPath() string {
	return filepath.Join(s.baseDir, "metadata.json")
}

// ReadMetadata reads metadata.json. If it does not exist, a zero Metadata is returned.
func (s *Storage) ReadMetadata() (Metadata, error) {
	data, err := os.ReadFile(s.metadataPath())
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Metadata{}, nil
		}
		return Metadata{}, fmt.Errorf("reading metadata: %w", err)
	}
	var m Metadata
	if err := json.Unmarshal(data, &m); err != nil {
		return Metadata{}, fmt.Errorf("decoding metadata: %w", err)
	}
	return m, nil
}

// WriteMetadata writes (or updates) metadata.json with the given repo URL
// and sets LastUpdate to the current time.
func (s *Storage) WriteMetadata(repoURL string, goModule string) error {
	m := Metadata{
		RepoURL:    repoURL,
		LastUpdate: time.Now().UnixMilli(),
		GoModule:   goModule,
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding metadata: %w", err)
	}
	if err := os.WriteFile(s.metadataPath(), data, 0o644); err != nil {
		return fmt.Errorf("writing metadata: %w", err)
	}
	return nil
}

// --------------------------------------------------------------------------
// Static file helpers
// --------------------------------------------------------------------------

// BranchFileName returns the file name (without directory) used for a branch's
// data file. This is useful for the frontend to know what URL to fetch.
func BranchFileName(branch string) string {
	return sanitizeBranchName(branch) + ".json"
}
