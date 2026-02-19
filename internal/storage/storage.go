package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"

	"github.com/royalcat/go-continuous-benchmarking/internal/model"
)

// releaseTagsFileName is the name of the JSON file that maps commit SHAs to
// their semver tag names. It lives alongside the branch data files in data/.
const releaseTagsFileName = "release_tags.json"

// ReleasesVirtualBranch is the name of the synthetic branch that aggregates
// benchmark data from all semver-tagged runs (e.g. v1.0.0, v2.3.4-rc.1).
// It always appears at the top of the branch selector in the frontend.
const ReleasesVirtualBranch = "releases"

// semverRe matches tag names that follow semantic versioning:
// optional "v" prefix, then MAJOR.MINOR.PATCH, with optional pre-release suffix.
var semverRe = regexp.MustCompile(`^v?\d+\.\d+\.\d+`)

// IsSemanticVersionTag reports whether name looks like a semver tag
// (e.g. "v1.0.0", "1.2.3", "v0.1.0-beta.1").
func IsSemanticVersionTag(name string) bool {
	return semverRe.MatchString(name)
}

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

// releaseTagsPath returns the path to data/release_tags.json.
func (s *Storage) releaseTagsPath() string {
	return filepath.Join(s.baseDir, "data", releaseTagsFileName)
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
//
// Semver tags (e.g. "v1.0.0") are never added individually. Instead the
// virtual "releases" branch is registered so that all tag data is aggregated
// under a single entry in the selector.
func (s *Storage) EnsureBranch(branch string) (bool, error) {
	// For semver tags, register the virtual "releases" branch instead.
	nameToRegister := branch
	if IsSemanticVersionTag(branch) {
		nameToRegister = ReleasesVirtualBranch
	}

	branches, err := s.ReadBranches()
	if err != nil {
		return false, err
	}

	for _, b := range branches {
		if b == nameToRegister {
			return false, nil
		}
	}

	branches = append(branches, nameToRegister)
	sortBranches(branches)

	if err := s.WriteBranches(branches); err != nil {
		return false, err
	}
	return true, nil
}

// sortBranches sorts the branch list alphabetically but always keeps
// the "releases" virtual branch at the very top of the list.
func sortBranches(branches []string) {
	sort.SliceStable(branches, func(i, j int) bool {
		if branches[i] == ReleasesVirtualBranch {
			return true
		}
		if branches[j] == ReleasesVirtualBranch {
			return false
		}
		return branches[i] < branches[j]
	})
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
//
// When branch is a semver tag, the entries are also merged into the combined
// "releases" data file so that all tagged releases can be compared side by
// side. The individual tag data file is still written for reference.
func (s *Storage) AppendEntries(branch string, newEntries []model.BenchmarkEntry, maxItems int) error {
	if len(newEntries) == 0 {
		return nil
	}

	// Register the branch (or "releases" for semver tags) in the branch list.
	if _, err := s.EnsureBranch(branch); err != nil {
		return fmt.Errorf("ensuring branch %q: %w", branch, err)
	}

	// Write to the individual branch/tag data file.
	if err := s.mergeEntries(branch, newEntries, maxItems); err != nil {
		return err
	}

	// For semver tags, also merge entries into the combined "releases" file
	// and record the tag→SHA mapping so the frontend can show version labels.
	if IsSemanticVersionTag(branch) {
		if err := s.mergeEntries(ReleasesVirtualBranch, newEntries, maxItems); err != nil {
			return fmt.Errorf("updating releases data: %w", err)
		}
		if err := s.recordReleaseTags(branch, newEntries); err != nil {
			return fmt.Errorf("updating release tags map: %w", err)
		}
	}

	return nil
}

// mergeEntries performs the actual read-modify-write merge of newEntries into
// the data file for the given branch name. It handles deduplication, sorting,
// and trimming.
func (s *Storage) mergeEntries(branch string, newEntries []model.BenchmarkEntry, maxItems int) error {
	// Read existing data.
	entries, err := s.ReadBranchData(branch)
	if err != nil {
		return err
	}

	// Build a set of new entry keys for fast lookup.
	newKeys := make(map[model.EntryKeyValue]struct{}, len(newEntries))
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
// Release tags map
// --------------------------------------------------------------------------

// readReleaseTags reads the release_tags.json map (commit SHA → tag name).
// Returns an empty map if the file does not exist.
func (s *Storage) readReleaseTags() (map[string]string, error) {
	data, err := os.ReadFile(s.releaseTagsPath())
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return make(map[string]string), nil
		}
		return nil, fmt.Errorf("reading release tags: %w", err)
	}
	var tags map[string]string
	if err := json.Unmarshal(data, &tags); err != nil {
		return nil, fmt.Errorf("decoding release tags: %w", err)
	}
	if tags == nil {
		tags = make(map[string]string)
	}
	return tags, nil
}

// writeReleaseTags writes the release tags map to disk.
func (s *Storage) writeReleaseTags(tags map[string]string) error {
	data, err := json.MarshalIndent(tags, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding release tags: %w", err)
	}
	if err := os.WriteFile(s.releaseTagsPath(), data, 0o644); err != nil {
		return fmt.Errorf("writing release tags: %w", err)
	}
	return nil
}

// recordReleaseTags updates release_tags.json with mappings from each entry's
// commit SHA to the given tag name. If a SHA already has a mapping, it is
// overwritten (the latest tag wins, which handles re-tags).
func (s *Storage) recordReleaseTags(tag string, entries []model.BenchmarkEntry) error {
	tags, err := s.readReleaseTags()
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.Commit.SHA != "" {
			tags[e.Commit.SHA] = tag
		}
	}
	return s.writeReleaseTags(tags)
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
