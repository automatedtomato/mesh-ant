// app.js — MeshAnt Web UI main module.
//
// This file wires together the observer gate, API calls, and the render/export
// sub-modules (render.js, export.js). Sections are separated by named comments.
//
// Data flow:
//   observer input → loadGraph() → fetch /articulate + /shadow in parallel
//   → initGraph(graphData, handleNodeClick)     [render.js]
//   → renderShadowPanel(shadowData, ...)         [render.js]
//   → setLastArticulateEnvelope(envelope)        [export.js]
//   → node click → handleNodeClick(name)
//       → fetch /element/{name}?observer=...
//       → renderDetailPanel(name, traces, ...)   [render.js]

// === SECTION 1: Observer Gate ===

// Module-level state: current observer position and last API responses.
let currentObserver = '';

/**
 * initObserverGate — wires up the #observer-form submit handler.
 * Called once on DOMContentLoaded.
 */
function initObserverGate() {
  const form = document.getElementById('observer-form');
  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    const input = document.getElementById('observer-input');
    const value = input.value.trim();
    if (!value) return;
    currentObserver = value;
    clearError();
    await loadGraph();
  });
}

// === SECTION 2: Cut Metadata ===

/**
 * renderCutMeta — populates the #cut-header bar fields with CutMeta values.
 * Shows "unbounded" when from/to are null; tags as comma-separated or "none".
 *
 * @param {Object} cut — CutMeta object from the /articulate Envelope.
 */
function renderCutMeta(cut) {
  document.getElementById('cut-observer').textContent = `Observer: ${cut.observer}`;

  const from = cut.from || 'unbounded';
  const to = cut.to || 'unbounded';
  document.getElementById('cut-window').textContent = `Window: ${from} – ${to}`;

  const tags = (cut.tags && cut.tags.length > 0) ? cut.tags.join(', ') : 'none';
  document.getElementById('cut-tags').textContent = `Tags: ${tags}`;

  document.getElementById('cut-trace-count').textContent = `Traces: ${cut.trace_count}`;
  document.getElementById('cut-shadow-count').textContent = `Shadow: ${cut.shadow_count}`;
}

// === SECTION 3: Graph Rendering (Cytoscape) ===

/**
 * loadGraph — fetches /articulate and /shadow in parallel for the current
 * observer, then renders the graph + shadow panel and shows the main layout.
 *
 * On success: shows #cut-header and #main; calls renderCutMeta, initGraph,
 * renderShadowPanel. On API error: shows the error banner and leaves the layout
 * hidden until the user retries.
 */
async function loadGraph() {
  const observer = currentObserver;
  if (!observer) return;

  try {
    // Fetch /articulate and /shadow in parallel — both require the same observer.
    const [articulateEnv, shadowEnv] = await Promise.all([
      apiFetch(`/articulate?observer=${encodeURIComponent(observer)}`),
      apiFetch(`/shadow?observer=${encodeURIComponent(observer)}`),
    ]);

    // Store the full articulate envelope for export.
    setLastArticulateEnvelope(articulateEnv);

    // Update cut header.
    renderCutMeta(articulateEnv.cut);

    // Render graph (Cytoscape).
    const graphData = articulateEnv.data || {};
    initGraph(graphData, handleNodeClick);

    // Render shadow panel.
    const shadowData = shadowEnv.data || [];
    renderShadowPanel(shadowData, (name) => handleNodeClick(name));

    // Show the main layout; hide observer gate's extra vertical padding.
    document.getElementById('cut-header').hidden = false;
    document.getElementById('main').hidden = false;

  } catch (err) {
    // On reload failure, hide any stale graph from a previous observer so the
    // UI does not display a cut that names a different position than the current
    // error state — an ANT violation (the displayed cut would be mislabelled).
    document.getElementById('cut-header').hidden = true;
    document.getElementById('main').hidden = true;
    showError(err.message || 'Failed to load graph. Check the observer name and try again.');
  }
}

// === SECTION 5: Node Click + Detail Panel ===
// (Section 4 is shadow panel — rendered in render.js via renderShadowPanel.)

/**
 * handleNodeClick — called when a graph node or shadow item is clicked.
 * Fetches /element/{name}?observer=... and renders the detail panel.
 *
 * @param {string} nodeName — the element name.
 */
async function handleNodeClick(nodeName) {
  const observer = currentObserver;
  if (!observer) return;

  try {
    const envelope = await apiFetch(
      `/element/${encodeURIComponent(nodeName)}?observer=${encodeURIComponent(observer)}`
    );
    const traces = envelope.data || [];
    renderDetailPanel(nodeName, traces, observer);
  } catch (err) {
    // Show the error in detail panel rather than the global banner so the user
    // can still navigate other nodes.
    const content = document.getElementById('detail-content');
    content.innerHTML = `<p class="detail-empty">Error loading traces: ${escapeHTMLApp(err.message)}</p>`;
  }
}

// === SECTION 6: Export (JSON + DOT) ===

/**
 * initExportButtons — wires up the Export JSON and Export DOT button handlers.
 * Called once on DOMContentLoaded.
 */
function initExportButtons() {
  document.getElementById('btn-export-json').addEventListener('click', exportJSON);
  document.getElementById('btn-export-dot').addEventListener('click', exportDOT);
}

// === SECTION 7: Error Handling ===

/**
 * showError — displays msg in the #error-banner below the observer gate.
 *
 * @param {string} msg — human-readable error message.
 */
function showError(msg) {
  const banner = document.getElementById('error-banner');
  banner.textContent = msg;
  banner.hidden = false;
}

/**
 * clearError — hides the #error-banner. Called on each new loadGraph attempt.
 */
function clearError() {
  const banner = document.getElementById('error-banner');
  banner.hidden = true;
  banner.textContent = '';
}

/**
 * apiFetch — wrapper around fetch that checks HTTP status and parses JSON.
 * Rejects with a user-facing error message on non-2xx responses.
 *
 * @param {string} url — endpoint path (relative).
 * @returns {Promise<Object>} parsed JSON Envelope.
 */
async function apiFetch(url) {
  let response;
  try {
    response = await fetch(url);
  } catch (networkErr) {
    throw new Error(`Network error: ${networkErr.message}`);
  }
  if (!response.ok) {
    // Attempt to parse the {"error": "..."} body from the server.
    let errMsg = `HTTP ${response.status}`;
    try {
      const body = await response.json();
      if (body && body.error) errMsg = body.error;
    } catch (_) {
      // Ignore JSON parse failure — use status code as message.
    }
    throw new Error(errMsg);
  }
  return response.json();
}

/**
 * escapeHTMLApp — minimal HTML escaping for inline innerHTML error messages.
 * Duplicate of the render.js escapeHTML; kept local to avoid cross-module dependency.
 *
 * @param {string} s
 * @returns {string}
 */
function escapeHTMLApp(s) {
  return String(s)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}

// === SECTION 8: Init ===

/**
 * DOMContentLoaded — entry point.
 * Hide the cut header and main area until the observer is submitted; wire up
 * all event handlers.
 */
document.addEventListener('DOMContentLoaded', () => {
  // Ensure the cut-header and main area start hidden (belt + suspenders with HTML hidden attr).
  document.getElementById('cut-header').hidden = true;
  document.getElementById('main').hidden = true;

  initObserverGate();
  initExportButtons();
});
