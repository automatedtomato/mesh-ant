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
//       → fetch /element/{name}?observer=...&from=...&to=...
//       → renderDetailPanel(name, traces, ...)   [render.js]
//
// Time window state (currentFrom / currentTo) is wired in SECTION 6.
// All API calls append buildWindowParams() to include the active window.

// === SECTION 1: Observer Gate ===

// Module-level state: current observer position and active time window.
// currentFrom / currentTo are RFC3339 strings or '' (unbounded).
// They are reset to '' whenever the observer changes (observer gate pattern:
// a new observer position implies a fresh, unconstrained reading by default).
let currentObserver = '';
let currentFrom = '';
let currentTo = '';

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

    // Reset time window when observer changes — a new observer position should
    // not silently inherit the previous observer's time constraint (T2 tension:
    // see time-window-controls-v1.md; resetting is the safer default).
    currentFrom = '';
    currentTo = '';
    document.getElementById('time-from').value = '';
    document.getElementById('time-to').value = '';

    clearError();
    await loadGraph();
  });
}

/**
 * loadObserverHints — fetches GET /observers and renders clickable chips
 * below the observer input. Clicking a chip fills the input with that observer name.
 * Silently does nothing on error (hints are cosmetic; the gate still works without them).
 *
 * Called once on DOMContentLoaded.
 */
async function loadObserverHints() {
  let observers;
  try {
    const envelope = await apiFetch('/observers');
    observers = envelope.data || [];
  } catch (_) {
    // Hints are non-critical; ignore fetch errors.
    return;
  }

  if (!observers.length) return;

  const hintsEl = document.getElementById('observer-hints');
  const chipsEl = document.getElementById('observer-chips');

  observers.forEach(name => {
    const chip = document.createElement('button');
    chip.type = 'button'; // prevent form submit
    chip.className = 'observer-chip';
    chip.textContent = name;
    chip.addEventListener('click', () => {
      document.getElementById('observer-input').value = name;
    });
    chipsEl.appendChild(chip);
  });

  hintsEl.hidden = false;
}

// === SECTION 2: Time Window Helpers ===

/**
 * toRFC3339 — converts a datetime-local input value to an RFC3339 UTC string.
 *
 * The datetime-local input yields values in two forms depending on the browser:
 *   "YYYY-MM-DDThh:mm"    — no seconds, no timezone (most browsers)
 *   "YYYY-MM-DDThh:mm:ss" — with seconds, no timezone (some browsers)
 * Neither form includes a timezone offset. We append seconds (if absent) and
 * 'Z' to produce a valid RFC3339 UTC string.
 *
 * ANT tension T1: this imposes a UTC temporal frame on the cut. Traces whose
 * original timestamps use other timezone offsets may be unexpectedly included
 * or excluded. Named tension; not resolved in v1.
 *
 * @param {string} val — datetime-local value or empty string.
 * @returns {string} RFC3339 UTC string, or '' if val is empty.
 */
function toRFC3339(val) {
  if (!val) return '';
  // Append ':00' if seconds are absent (value ends with "hh:mm" pattern),
  // then append 'Z' for UTC. datetime-local never emits a timezone offset,
  // so no offset-stripping is needed.
  const withSeconds = /T\d{2}:\d{2}$/.test(val) ? val + ':00' : val;
  return withSeconds + 'Z';
}

/**
 * buildWindowParams — constructs the from/to query string fragment for API calls.
 *
 * Returns '&from=RFC3339&to=RFC3339', '&from=RFC3339', '&to=RFC3339', or ''
 * depending on which of currentFrom / currentTo are set. The leading '&' is
 * intentional: callers always have at least one prior param (observer=...).
 *
 * @returns {string}
 */
function buildWindowParams() {
  let params = '';
  if (currentFrom) params += `&from=${encodeURIComponent(currentFrom)}`;
  if (currentTo) params += `&to=${encodeURIComponent(currentTo)}`;
  return params;
}

// === SECTION 3: Cut Metadata (renderCutMeta) ===

/**
 * renderCutMeta — populates the #cut-header bar fields with CutMeta values.
 * Shows "unbounded" when from/to are null; tags as comma-separated or "none".
 * The from/to values come from the server response (CutMeta), not from
 * currentFrom/currentTo, so the bar always reflects the actual applied cut.
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

// === SECTION 4: Graph Rendering (Cytoscape) ===

/**
 * loadGraph — fetches /articulate and /shadow in parallel for the current
 * observer and active time window, then renders the graph + shadow panel
 * and shows the main layout.
 *
 * On success: shows #cut-header, #time-window-picker, and #main; calls
 * renderCutMeta, initGraph, renderShadowPanel. On API error: hides all three
 * so no stale cut is displayed (ANT: a mislabelled cut is an integrity violation).
 */
async function loadGraph() {
  const observer = currentObserver;
  if (!observer) return;

  // buildWindowParams() appends &from=...&to=... if a time window is active.
  const windowParams = buildWindowParams();

  try {
    // Fetch /articulate and /shadow in parallel — both receive the same observer
    // and time window so the cut is consistent across the two responses.
    const [articulateEnv, shadowEnv] = await Promise.all([
      apiFetch(`/articulate?observer=${encodeURIComponent(observer)}${windowParams}`),
      apiFetch(`/shadow?observer=${encodeURIComponent(observer)}${windowParams}`),
    ]);

    // Store the full articulate envelope for export (includes cut.from/cut.to).
    setLastArticulateEnvelope(articulateEnv);

    // Update cut header — from/to come from server CutMeta, not client state.
    renderCutMeta(articulateEnv.cut);

    // Render graph (Cytoscape).
    const graphData = articulateEnv.data || {};
    initGraph(graphData, handleNodeClick);

    // Render shadow panel.
    const shadowData = shadowEnv.data || [];
    renderShadowPanel(shadowData, (name) => handleNodeClick(name));

    // Show the main layout and the time window picker (observer gate pattern:
    // the picker is only meaningful once an observer has been committed).
    document.getElementById('cut-header').hidden = false;
    document.getElementById('time-window-picker').hidden = false;
    document.getElementById('main').hidden = false;

  } catch (err) {
    // On reload failure, hide any stale graph from a previous observer so the
    // UI does not display a cut that names a different position than the current
    // error state — an ANT violation (the displayed cut would be mislabelled).
    document.getElementById('cut-header').hidden = true;
    document.getElementById('time-window-picker').hidden = true;
    document.getElementById('main').hidden = true;
    showError(err.message || 'Failed to load graph. Check the observer name and try again.');
  }
}

// === SECTION 5: Node Click + Detail Panel ===
// (Shadow panel — Section 3b — is rendered in render.js via renderShadowPanel.
//  Time window picker init — Section 6 — follows below.)

/**
 * handleNodeClick — called when a graph node or shadow item is clicked.
 * Fetches /element/{name}?observer=...&from=...&to=... and renders the detail
 * panel. The time window is included so the detail panel is consistent with the
 * current cut — showing traces outside the window would be a cut mismatch.
 *
 * @param {string} nodeName — the element name.
 */
async function handleNodeClick(nodeName) {
  const observer = currentObserver;
  if (!observer) return;

  try {
    const envelope = await apiFetch(
      `/element/${encodeURIComponent(nodeName)}?observer=${encodeURIComponent(observer)}${buildWindowParams()}`
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

// === SECTION 6: Time Window Picker (init) ===

/**
 * initTimeWindowPicker — wires up the Apply and Reset buttons for the time
 * window picker. Called once on DOMContentLoaded.
 *
 * Apply: reads the from/to inputs, converts to RFC3339, updates module state,
 *        and calls loadGraph() to re-fetch with the new window.
 *
 * Reset to unbounded: clears the inputs, resets module state to '' (unbounded),
 *        and calls loadGraph(). "Unbounded" is a named cut state — not a missing
 *        value — which is why the button is "Reset to unbounded", not "Clear"
 *        (T3 tension: see time-window-controls-v1.md).
 *
 * ANT tension T4: the datetime-local picker presents time as a continuous,
 * linear axis. Traces, however, are scattered and discontinuous — the picker
 * provides no indication of where traces actually fall in time. An analyst may
 * set a window containing no traces and receive an empty graph without knowing
 * they are looking at a temporal void, not an empty network. Named; not resolved
 * in v1 (a future enhancement could surface temporal trace density).
 */
function initTimeWindowPicker() {
  document.getElementById('btn-apply-window').addEventListener('click', async () => {
    const fromVal = document.getElementById('time-from').value;
    const toVal = document.getElementById('time-to').value;
    currentFrom = toRFC3339(fromVal);
    currentTo = toRFC3339(toVal);
    await loadGraph();
  });

  document.getElementById('btn-reset-window').addEventListener('click', async () => {
    currentFrom = '';
    currentTo = '';
    document.getElementById('time-from').value = '';
    document.getElementById('time-to').value = '';
    await loadGraph();
  });
}

// === SECTION 7: Export (JSON + DOT) ===
// (exportJSON and exportDOT are defined in export.js; only init wiring here.)

/**
 * initExportButtons — wires up the Export JSON and Export DOT button handlers.
 * Called once on DOMContentLoaded.
 */
function initExportButtons() {
  document.getElementById('btn-export-json').addEventListener('click', exportJSON);
  document.getElementById('btn-export-dot').addEventListener('click', exportDOT);
}

// === SECTION 8: Error Handling and Utilities ===

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

// === SECTION 9: Init ===

/**
 * DOMContentLoaded — entry point.
 * Hide the cut header, time window picker, and main area until the observer is
 * submitted; wire up all event handlers.
 */
document.addEventListener('DOMContentLoaded', () => {
  // Ensure the cut-header, time-window-picker, and main area start hidden
  // (belt + suspenders with HTML hidden attr — observer gate pattern).
  document.getElementById('cut-header').hidden = true;
  document.getElementById('time-window-picker').hidden = true;
  document.getElementById('main').hidden = true;

  initObserverGate();
  initTimeWindowPicker();
  initExportButtons();
  loadObserverHints();
});
