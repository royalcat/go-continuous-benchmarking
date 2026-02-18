"use strict";

(function () {
  // ---- Configuration ----
  const CHART_COLOR = "#0969da";
  const CHART_COLOR_ALPHA = CHART_COLOR + "30";
  const POINT_RADIUS = 4;
  const POINT_HOVER_RADIUS = 6;

  // ---- DOM references ----
  const branchSelect = document.getElementById("branch-select");
  const filterInput = document.getElementById("filter-input");
  const mainEl = document.getElementById("main");
  const loadingMsg = document.getElementById("loading-msg");
  const lastUpdateEl = document.getElementById("last-update");
  const repoLinkEl = document.getElementById("repo-link");
  const dlButton = document.getElementById("dl-button");

  // ---- State ----
  let currentBranchData = null; // raw array of BenchmarkEntry
  let currentBranch = null;
  let chartInstances = []; // keep references so we can destroy on re-render

  // ---- Helpers ----

  function getBasePath() {
    // Determine the base URL path from the current page location.
    // This allows the app to be deployed under any sub-path on GitHub Pages.
    const path = window.location.pathname;
    // Remove trailing index.html if present
    const base = path.replace(/\/index\.html$/, "");
    return base.endsWith("/") ? base : base + "/";
  }

  async function fetchJSON(url) {
    const resp = await fetch(url);
    if (!resp.ok) {
      throw new Error(`HTTP ${resp.status} fetching ${url}`);
    }
    return resp.json();
  }

  function showMessage(html) {
    mainEl.innerHTML = `<div class="state-message">${html}</div>`;
  }

  function destroyCharts() {
    for (const c of chartInstances) {
      c.destroy();
    }
    chartInstances = [];
  }

  /**
   * Group benchmark entries by benchmark name.
   * Returns a Map<string, Array<{commit, date, bench}>>
   */
  function collectBenchesPerTestCase(entries) {
    const map = new Map();
    for (const entry of entries) {
      const { commit, date, benchmarks } = entry;
      for (const bench of benchmarks) {
        const result = { commit, date, bench };
        let arr = map.get(bench.name);
        if (!arr) {
          arr = [];
          map.set(bench.name, arr);
        }
        arr.push(result);
      }
    }
    return map;
  }

  function formatDate(isoOrTimestamp) {
    try {
      const d =
        typeof isoOrTimestamp === "number"
          ? new Date(isoOrTimestamp)
          : new Date(isoOrTimestamp);
      return d.toLocaleString();
    } catch {
      return String(isoOrTimestamp);
    }
  }

  function shortSHA(sha) {
    return sha ? sha.slice(0, 7) : "?";
  }

  // ---- Rendering ----

  function renderChart(container, name, dataset) {
    const card = document.createElement("div");
    card.className = "chart-card";

    const title = document.createElement("h2");
    title.textContent = name;
    card.appendChild(title);

    const wrapper = document.createElement("div");
    wrapper.className = "chart-wrapper";
    card.appendChild(wrapper);

    const canvas = document.createElement("canvas");
    wrapper.appendChild(canvas);
    container.appendChild(card);

    const labels = dataset.map((d) => shortSHA(d.commit.sha));
    const values = dataset.map((d) => d.bench.value);
    const unit = dataset.length > 0 ? dataset[0].bench.unit : "";

    const isDarkMode =
      window.matchMedia &&
      window.matchMedia("(prefers-color-scheme: dark)").matches;
    const gridColor = isDarkMode
      ? "rgba(255,255,255,0.1)"
      : "rgba(0,0,0,0.08)";
    const textColor = isDarkMode ? "#8b949e" : "#656d76";

    const chart = new Chart(canvas, {
      type: "line",
      data: {
        labels,
        datasets: [
          {
            label: name,
            data: values,
            borderColor: CHART_COLOR,
            backgroundColor: CHART_COLOR_ALPHA,
            borderWidth: 2,
            pointRadius: POINT_RADIUS,
            pointHoverRadius: POINT_HOVER_RADIUS,
            pointBackgroundColor: CHART_COLOR,
            fill: true,
            tension: 0.15,
          },
        ],
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        interaction: {
          mode: "index",
          intersect: false,
        },
        scales: {
          x: {
            title: {
              display: true,
              text: "Commit",
              color: textColor,
            },
            ticks: { color: textColor },
            grid: { color: gridColor },
          },
          y: {
            title: {
              display: true,
              text: unit,
              color: textColor,
            },
            beginAtZero: true,
            ticks: { color: textColor },
            grid: { color: gridColor },
          },
        },
        plugins: {
          legend: {
            display: false,
          },
          tooltip: {
            callbacks: {
              title: function (items) {
                if (!items.length) return "";
                const idx = items[0].dataIndex;
                const d = dataset[idx];
                const sha = shortSHA(d.commit.sha);
                return `Commit: ${sha}`;
              },
              beforeBody: function (items) {
                if (!items.length) return "";
                const idx = items[0].dataIndex;
                const d = dataset[idx];
                const lines = [];
                if (d.commit.message) {
                  lines.push(d.commit.message);
                }
                lines.push("");
                if (d.commit.date) {
                  lines.push("Date: " + formatDate(d.commit.date));
                }
                if (d.commit.author) {
                  lines.push("Author: @" + d.commit.author);
                }
                return lines.join("\n");
              },
              label: function (item) {
                const idx = item.dataIndex;
                const d = dataset[idx];
                let label = item.formattedValue + " " + d.bench.unit;
                return label;
              },
              afterLabel: function (item) {
                const idx = item.dataIndex;
                const d = dataset[idx];
                return d.bench.extra ? "\n" + d.bench.extra : "";
              },
            },
          },
        },
        onClick: function (_event, elements) {
          if (!elements || elements.length === 0) return;
          const idx = elements[0].index;
          const url = dataset[idx].commit.url;
          if (url) {
            window.open(url, "_blank");
          }
        },
      },
    });

    chartInstances.push(chart);
  }

  function renderBranch(entries) {
    destroyCharts();
    mainEl.innerHTML = "";

    if (!entries || entries.length === 0) {
      showMessage("No benchmark data available for this branch.");
      return;
    }

    const benchMap = collectBenchesPerTestCase(entries);

    const filterText = (filterInput.value || "").toLowerCase().trim();

    let rendered = 0;
    for (const [benchName, dataset] of benchMap.entries()) {
      if (filterText && !benchName.toLowerCase().includes(filterText)) {
        continue;
      }
      renderChart(mainEl, benchName, dataset);
      rendered++;
    }

    if (rendered === 0) {
      showMessage("No benchmarks match the current filter.");
    }
  }

  // ---- Data loading ----

  async function loadMetadata() {
    try {
      const base = getBasePath();
      const metadata = await fetchJSON(base + "metadata.json");
      if (metadata.lastUpdate) {
        lastUpdateEl.textContent = formatDate(metadata.lastUpdate);
      }
      if (metadata.repoUrl) {
        repoLinkEl.href = metadata.repoUrl;
        repoLinkEl.textContent = metadata.repoUrl;
      }
    } catch {
      // metadata.json is optional
      lastUpdateEl.textContent = "—";
    }
  }

  async function loadBranches() {
    const base = getBasePath();
    const branches = await fetchJSON(base + "branches.json");
    return branches;
  }

  async function loadBranchData(branch) {
    const base = getBasePath();
    // Branch file names match how the Go tool sanitizes them:
    // slashes and special chars become underscores.
    const safeName = branch.replace(/[/\\:*?"<>|]/g, "_");
    const data = await fetchJSON(base + "data/" + safeName + ".json");
    return data;
  }

  async function selectBranch(branch) {
    if (!branch) return;
    currentBranch = branch;

    destroyCharts();
    mainEl.innerHTML = "";
    showMessage('<span class="spinner"></span> Loading benchmark data…');
    dlButton.disabled = true;

    try {
      currentBranchData = await loadBranchData(branch);
      dlButton.disabled = false;
      renderBranch(currentBranchData);
    } catch (err) {
      showMessage("Error loading data for branch <b>" + branch + "</b>: " + err.message);
    }
  }

  // ---- Event listeners ----

  branchSelect.addEventListener("change", function () {
    selectBranch(branchSelect.value);
  });

  let filterTimeout = null;
  filterInput.addEventListener("input", function () {
    // Debounce filter re-renders
    clearTimeout(filterTimeout);
    filterTimeout = setTimeout(function () {
      if (currentBranchData) {
        renderBranch(currentBranchData);
      }
    }, 200);
  });

  dlButton.addEventListener("click", function () {
    if (!currentBranchData) return;
    const json = JSON.stringify(currentBranchData, null, 2);
    const blob = new Blob([json], { type: "application/json" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = (currentBranch || "benchmark") + ".json";
    a.click();
    URL.revokeObjectURL(url);
  });

  // ---- Initialization ----

  async function init() {
    await loadMetadata();

    let branches;
    try {
      branches = await loadBranches();
    } catch (err) {
      showMessage(
        "Could not load branch list. Make sure benchmark data has been generated.<br><small>" +
          err.message +
          "</small>"
      );
      return;
    }

    if (!branches || branches.length === 0) {
      showMessage("No branches with benchmark data found.");
      return;
    }

    // Populate branch selector
    branchSelect.innerHTML = "";
    for (const branch of branches) {
      const opt = document.createElement("option");
      opt.value = branch;
      opt.textContent = branch;
      branchSelect.appendChild(opt);
    }

    // Try to select from URL hash, otherwise pick the first branch.
    // URL format: #branch=main
    let initialBranch = branches[0];
    const hash = window.location.hash;
    if (hash) {
      const params = new URLSearchParams(hash.slice(1));
      const requested = params.get("branch");
      if (requested && branches.includes(requested)) {
        initialBranch = requested;
      }
    }

    branchSelect.value = initialBranch;
    await selectBranch(initialBranch);
  }

  // Update hash when branch changes so links can be shared
  branchSelect.addEventListener("change", function () {
    if (branchSelect.value) {
      window.location.hash = "branch=" + encodeURIComponent(branchSelect.value);
    }
  });

  init();
})();
