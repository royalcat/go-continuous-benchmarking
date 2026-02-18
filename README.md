# Go Continuous Benchmarking

A GitHub Action and CLI tool for continuous benchmarking of Go projects. It parses `go test -bench` output, stores results organized by branch, and publishes interactive Chart.js dashboards to GitHub Pages.

## Features

- **Parse Go benchmarks** — Understands standard `go test -bench` output including multiple metrics per benchmark (ns/op, B/op, allocs/op, MB/s), sub-benchmarks, and multiple packages
- **Branch-based organization** — Results are separated by branch with a branch list stored in `branches.json` for the frontend to discover
- **Interactive dashboard** — Chart.js line charts with tooltips showing commit message, author, date; click a data point to open the commit on GitHub
- **Dark mode support** — Dashboard automatically adapts to system color scheme
- **Filter benchmarks** — Client-side filter to quickly find specific benchmark charts
- **Shareable links** — Branch selection is stored in the URL hash (`#branch=main`)
- **Lightweight** — Written in Go with embedded frontend assets; no Node.js runtime needed

## Quick Start

### 1. Create a GitHub Pages branch

```sh
git checkout --orphan gh-pages
git rm -rf .
git commit --allow-empty -m "Initialize GitHub Pages"
git push origin gh-pages
git checkout main
```

Then enable GitHub Pages in your repository settings, pointing to the `gh-pages` branch.

### 2. Add the workflow

Create `.github/workflows/benchmark.yml`:

```yaml
name: Benchmarks
on:
  push:
    branches: [main]

permissions:
  contents: write

jobs:
  benchmark:
    name: Run benchmarks
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - uses: actions/setup-go@v5
        with:
          go-version: "stable"

      - name: Run benchmarks
        run: go test -bench=. -benchmem ./... | tee bench-output.txt

      - name: Store benchmark results
        uses: royalcat/go-continuous-benchmarking@v1
        with:
          output-file-path: bench-output.txt
          github-token: ${{ secrets.GITHUB_TOKEN }}
          auto-push: "true"
```

After the first run, your benchmark dashboard will be available at:

```
https://<user>.github.io/<repo>/dev/bench/
```

## Action Inputs

| Input | Required | Default | Description |
|---|---|---|---|
| `output-file-path` | **Yes** | — | Path to the file containing `go test -bench` output |
| `branch` | No | Current branch (`GITHUB_REF_NAME`) | Branch name for organizing results |
| `gh-pages-branch` | No | `gh-pages` | Name of the GitHub Pages branch |
| `benchmark-data-dir-path` | No | `dev/bench` | Path within the Pages branch for benchmark data and dashboard |
| `github-token` | No | — | GitHub API token for pushing to the Pages branch |
| `auto-push` | No | `false` | Automatically push results to the Pages branch |
| `max-items-in-chart` | No | `0` | Maximum data points per branch (0 = unlimited) |
| `repo-url` | No | Current repository URL | Repository URL shown in the dashboard header |
| `skip-fetch-gh-pages` | No | `false` | Skip fetching the Pages branch (if already checked out) |

## Action Outputs

| Output | Description |
|---|---|
| `benchmark-results-json` | Path to the JSON file containing parsed results for this run |

## Data Format

### Directory layout on the `gh-pages` branch

```
dev/bench/
├── index.html          # Dashboard page (auto-generated)
├── app.js              # Chart.js frontend (auto-generated)
├── metadata.json       # Repository URL and last update timestamp
├── branches.json       # ["main", "develop", "feature-x"]
└── data/
    ├── main.json       # Benchmark entries for the main branch
    ├── develop.json    # Benchmark entries for the develop branch
    └── ...
```

### `branches.json`

A JSON array of branch name strings. This file is maintained automatically by the tool and allows the frontend to discover which branches have data without relying on any API.

```json
["develop", "feature-perf", "main"]
```

### Branch data file (e.g. `data/main.json`)

Each branch data file is a JSON array of benchmark entries, ordered chronologically:

```json
[
  {
    "commit": {
      "sha": "abc123def456789",
      "message": "Optimize hot path in parser",
      "author": "royalcat",
      "date": "2024-06-15T10:30:00Z",
      "url": "https://github.com/owner/repo/commit/abc123def456789"
    },
    "date": 1718444400000,
    "benchmarks": [
      {
        "name": "BenchmarkParse",
        "value": 1523.4,
        "unit": "ns/op",
        "extra": "1000000 times\n8 procs"
      },
      {
        "name": "BenchmarkParse - B/op",
        "value": 256,
        "unit": "B/op",
        "extra": "1000000 times\n8 procs"
      },
      {
        "name": "BenchmarkParse - allocs/op",
        "value": 3,
        "unit": "allocs/op",
        "extra": "1000000 times\n8 procs"
      }
    ]
  }
]
```

### Branch name sanitization

Branch names containing `/`, `\`, `:`, `*`, `?`, `"`, `<`, `>`, or `|` have those characters replaced with `_` when used as file names. The mapping is stored in `branches.json` with the original names so the frontend can display them correctly.

## Benchmark Output Parsing

The tool parses standard Go benchmark output as defined by the [Go benchmark format proposal](https://go.googlesource.com/proposal/+/master/design/14313-benchmark-format.md):

```
BenchmarkFib20-8           30000             41653 ns/op
BenchmarkAlloc-8           10000             15000 ns/op        1024 B/op          5 allocs/op
BenchmarkIO-8              10000            150000 ns/op       66.67 MB/s
BenchmarkParent/SubCase-8  50000              3200 ns/op
```

**Supported features:**

- Standard `BenchmarkName-PROCS iterations value unit` format
- Multiple metrics per benchmark line (ns/op, B/op, allocs/op, MB/s, or any custom metric)
- Sub-benchmarks with `/` separator
- Multiple packages (benchmark names are prefixed with the package path when there are multiple)
- Windows (`\r\n`) and Unix (`\n`) line endings

When a benchmark line contains multiple value/unit pairs, each additional metric is stored as a separate chart with the name `BenchmarkName - unit` (e.g. `BenchmarkAlloc - B/op`).

## CLI Usage

You can also use the tool directly from the command line outside of GitHub Actions:

```sh
# Build
go build -o gobenchdata .

# Parse and store benchmark results
go test -bench=. -benchmem ./... | ./gobenchdata \
  -branch=main \
  -data-dir=./bench-results \
  -commit-sha="$(git rev-parse HEAD)" \
  -commit-msg="$(git log -1 --format='%s')" \
  -commit-author="$(git log -1 --format='%an')" \
  -commit-date="$(git log -1 --format='%aI')" \
  -commit-url="https://github.com/owner/repo/commit/$(git rev-parse HEAD)" \
  -repo-url="https://github.com/owner/repo" \
  -max-items=100
```

Or using a file:

```sh
go test -bench=. -benchmem ./... > bench-output.txt

./gobenchdata \
  -output-file=bench-output.txt \
  -branch=main \
  -data-dir=./bench-results \
  -commit-sha="$(git rev-parse HEAD)"
```

### CLI Flags

| Flag | Default | Description |
|---|---|---|
| `-output-file` | stdin | Path to benchmark output file (reads stdin if empty) |
| `-branch` | `main` | Git branch name |
| `-data-dir` | `dev/bench` | Directory for benchmark data and frontend files |
| `-commit-sha` | **(required)** | Commit SHA |
| `-commit-msg` | `""` | Commit message |
| `-commit-author` | `""` | Commit author |
| `-commit-date` | Now (RFC 3339) | Commit date |
| `-commit-url` | `""` | URL to the commit |
| `-max-items` | `0` | Max entries per branch (0 = unlimited) |
| `-repo-url` | `""` | Repository URL for the frontend header |

## Examples

### Multiple benchmark suites

If your project has benchmarks in multiple packages, just run them all and pipe the output:

```yaml
- name: Run all benchmarks
  run: go test -bench=. -benchmem ./... | tee bench-output.txt
```

The tool will detect multiple `pkg:` lines and prefix benchmark names accordingly to avoid collisions.

### Tracking multiple branches

```yaml
name: Benchmarks
on:
  push:
    branches: [main, develop, "release/*"]

# ...same steps as Quick Start...
```

Each branch will get its own data file and appear in the branch selector dropdown on the dashboard.

### Limiting chart history

To keep the data files from growing indefinitely:

```yaml
- name: Store benchmark results
  uses: royalcat/go-continuous-benchmarking@v1
  with:
    output-file-path: bench-output.txt
    github-token: ${{ secrets.GITHUB_TOKEN }}
    auto-push: "true"
    max-items-in-chart: "100"
```

### Custom data directory

```yaml
- name: Store benchmark results
  uses: royalcat/go-continuous-benchmarking@v1
  with:
    output-file-path: bench-output.txt
    github-token: ${{ secrets.GITHUB_TOKEN }}
    auto-push: "true"
    benchmark-data-dir-path: "docs/benchmarks"
```

The dashboard will then be available at `https://<user>.github.io/<repo>/docs/benchmarks/`.

### Manual push (without auto-push)

If you want more control over when results are pushed:

```yaml
- name: Store benchmark results
  uses: royalcat/go-continuous-benchmarking@v1
  with:
    output-file-path: bench-output.txt
    auto-push: "false"

- name: Push results
  run: git push origin gh-pages
```

## Dashboard

The dashboard is a single-page application that loads data via `fetch()` from the same directory. It requires no server — it works purely as static files on GitHub Pages.

**Features:**

- **Branch selector** — Switch between branches to view their benchmark history
- **Filter** — Type to filter benchmarks by name across all charts
- **Tooltips** — Hover over data points to see commit SHA, message, author, and date
- **Click to open** — Click any data point to open the commit on GitHub
- **Download** — Download the current branch's raw JSON data
- **Dark mode** — Automatically follows system preference via `prefers-color-scheme`
- **URL hash** — Branch selection is persisted in the URL hash for sharing (e.g. `#branch=develop`)

## Development

```sh
# Run tests
go test -v ./...

# Build
go build -o gobenchdata .

# Run with sample data
echo 'BenchmarkFoo-8  1000000  1234 ns/op  56 B/op  2 allocs/op' | \
  ./gobenchdata -branch=test -data-dir=/tmp/bench -commit-sha=abc123

# View the generated dashboard
cd /tmp/bench && python3 -m http.server 8080
# Open http://localhost:8080
```

## How It Works

1. **Parse** — The Go tool reads `go test -bench` output (from a file or stdin) and extracts benchmark names, values, and units using regex matching
2. **Store** — Results are appended to a per-branch JSON file under `data/<branch>.json`, and the branch name is registered in `branches.json`
3. **Deploy** — The embedded frontend files (`index.html`, `app.js`) are written to the data directory
4. **Push** — If `auto-push` is enabled, the changes are committed and pushed to the gh-pages branch
5. **View** — GitHub Pages serves the static files; the frontend fetches `branches.json` and per-branch data files to render Chart.js graphs

## Security Notes

- **Only run on push events to your own branches.** Do not run this action on `pull_request` events from forks, as it has write access to your gh-pages branch.
- The `github-token` is only used for pushing to the gh-pages branch and is not exposed to the frontend.

## License

MIT