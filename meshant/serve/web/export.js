// export.js — MeshAnt Web UI export module.
//
// Responsibilities:
//   - exportJSON: download the last /articulate Envelope as a JSON file.
//   - exportDOT:  convert the last MeshGraph data to Graphviz DOT format and
//                 download it. Client-side DOT mirrors PrintGraphDOT conventions
//                 (see graph/export.go). ANT tension T2 in web-ui-v1.md.
//
// Both functions operate on module-level state set by app.js via
// setLastArticulateEnvelope(). They do NOT perform any fetch calls.

// Last /articulate Envelope; set by app.js after each successful articulate call.
let _lastEnvelope = null;

/**
 * setLastArticulateEnvelope — called by app.js to record the most recent
 * /articulate response Envelope for use by export functions.
 *
 * @param {Object} envelope — full Envelope JSON ({ cut, data }).
 */
function setLastArticulateEnvelope(envelope) {
  _lastEnvelope = envelope;
}

/**
 * exportJSON — downloads the last /articulate Envelope as a JSON file.
 * Filename: meshant-cut-{observer}-{date}.json
 * No-op if no articulate response has been received yet.
 */
function exportJSON() {
  if (!_lastEnvelope) return;
  const observer = (_lastEnvelope.cut && _lastEnvelope.cut.observer) || 'unknown';
  const date = new Date().toISOString().slice(0, 10);
  const filename = `meshant-cut-${slugify(observer)}-${date}.json`;
  const blob = new Blob(
    [JSON.stringify(_lastEnvelope, null, 2)],
    { type: 'application/json' }
  );
  triggerDownload(filename, blob);
}

/**
 * exportDOT — converts the last MeshGraph data to Graphviz DOT format and
 * downloads it. Filename: meshant-cut-{observer}-{date}.dot
 *
 * The DOT representation mirrors PrintGraphDOT (graph/export.go):
 *   - Comment line with observer + window + tags
 *   - digraph with rankdir=TB, node [shape=box]
 *   - One node per element with appearance count in label
 *   - One arc per source × target pair per edge
 *   - Shadow cluster (cluster_shadow) with dashed style when shadow exists
 *
 * ANT tension T2 (web-ui-v1.md): this duplicates Go logic client-side.
 * Accepted trade-off: avoids a new server endpoint; canonical format is
 * graph.PrintGraphDOT. If they diverge, the Go output is authoritative.
 */
function exportDOT() {
  if (!_lastEnvelope) return;
  const cut = _lastEnvelope.cut || {};
  const graphData = _lastEnvelope.data || {};
  const observer = cut.observer || 'unknown';
  const date = new Date().toISOString().slice(0, 10);
  const filename = `meshant-cut-${slugify(observer)}-${date}.dot`;
  const dot = buildDOT(cut, graphData);
  const blob = new Blob([dot], { type: 'text/plain' });
  triggerDownload(filename, blob);
}

/**
 * buildDOT — builds a Graphviz DOT string from a CutMeta and MeshGraph.
 *
 * @param {Object} cut — CutMeta object ({ observer, from, to, tags, ... }).
 * @param {Object} graphData — MeshGraph data ({ nodes, edges, cut }).
 * @returns {string} DOT format string.
 */
function buildDOT(cut, graphData) {
  const lines = [];

  // Comment line naming the observer position and time window.
  lines.push(`// Observer: ${dotEscape(cut.observer || 'unknown')} | Window: ${windowLabel(cut)} | Tags: ${tagsLabel(cut)}`);
  lines.push('digraph {');
  lines.push('  rankdir=TB;');
  lines.push('  node [shape=box];');

  // Nodes — sorted alphabetically for deterministic output.
  const nodes = graphData.nodes || {};
  const nodeNames = Object.keys(nodes).sort();
  nodeNames.forEach(name => {
    const node = nodes[name];
    const count = node.appearance_count || 0;
    lines.push(`  ${dotQuote(name)} [label=${dotQuote(`${name} (${count})`)}];`);
  });

  // Edges — one arc per source × target pair.
  const edges = graphData.edges || [];
  edges.forEach(edge => {
    const label = dotQuote(truncateDOTLabel(edge.what_changed || ''));
    (edge.sources || []).forEach(src => {
      (edge.targets || []).forEach(tgt => {
        lines.push(`  ${dotQuote(src)} -> ${dotQuote(tgt)} [label=${label}];`);
      });
    });
  });

  // Shadow cluster — from the articulation's cut.shadow_elements if present.
  // graphData is MeshGraph (envelope.data). MeshGraph.Cut serialises to
  // graphData.cut (json:"cut"), and Cut.ShadowElements serialises to
  // graphData.cut.shadow_elements (json:"shadow_elements"). This matches the
  // Go struct definition in graph/graph.go.
  const shadowElems = (graphData.cut && graphData.cut.shadow_elements) || [];
  if (shadowElems.length > 0) {
    lines.push('  subgraph cluster_shadow {');
    lines.push('    style=dashed; label="shadow";');
    shadowElems.forEach(se => {
      lines.push(`    ${dotQuote(se.name)};`);
    });
    // Invisible edges to encourage vertical layout (mirrors PrintGraphDOT).
    for (let i = 1; i < shadowElems.length; i++) {
      lines.push(`    ${dotQuote(shadowElems[i - 1].name)} -> ${dotQuote(shadowElems[i].name)} [style=invis];`);
    }
    lines.push('  }');
  }

  lines.push('}');
  return lines.join('\n') + '\n';
}

/* --- Helpers --- */

/**
 * dotQuote — wraps s in double-quotes, escaping internal double-quotes.
 * Mirrors graph/export.go dotQuote.
 *
 * @param {string} s
 * @returns {string}
 */
function dotQuote(s) {
  return '"' + String(s).replace(/\\/g, '\\\\').replace(/"/g, '\\"') + '"';
}

/**
 * dotEscape — strips newlines to prevent injection into a DOT comment line.
 * Mirrors graph/export.go stripNewlines.
 *
 * @param {string} s
 * @returns {string}
 */
function dotEscape(s) {
  return String(s).replace(/\r?\n/g, ' ');
}

/**
 * truncateDOTLabel — truncates to 28 characters (matching Go maxEdgeLabel).
 *
 * @param {string} s
 * @returns {string}
 */
function truncateDOTLabel(s) {
  const runes = [...s];
  if (runes.length <= 28) return s;
  return runes.slice(0, 28).join('') + '...';
}

/**
 * windowLabel — formats the time window from a CutMeta as "from–to" or "unbounded".
 *
 * @param {Object} cut
 * @returns {string}
 */
function windowLabel(cut) {
  if (!cut.from && !cut.to) return 'unbounded';
  const from = cut.from || '(unbounded)';
  const to = cut.to || '(unbounded)';
  return `${from}–${to}`;
}

/**
 * tagsLabel — formats the tags array from a CutMeta as "tag1, tag2" or "none".
 *
 * @param {Object} cut
 * @returns {string}
 */
function tagsLabel(cut) {
  if (!cut.tags || cut.tags.length === 0) return 'none';
  return cut.tags.join(', ');
}

/**
 * slugify — converts a string to a safe filename-friendly slug.
 * Replaces non-alphanumeric characters (except hyphens) with hyphens.
 *
 * @param {string} s
 * @returns {string}
 */
function slugify(s) {
  return String(s).toLowerCase().replace(/[^a-z0-9-]/g, '-');
}

/**
 * triggerDownload — creates a temporary anchor element and programmatically
 * clicks it to download the given Blob with the given filename.
 *
 * @param {string} filename
 * @param {Blob} blob
 */
function triggerDownload(filename, blob) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  // Revoke after a short delay to allow the browser to initiate the download.
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
