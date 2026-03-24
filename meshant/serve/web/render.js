// render.js — MeshAnt Web UI rendering module.
//
// Responsibilities:
//   - Map /articulate MeshGraph JSON to Cytoscape elements array.
//   - Initialise and layout the Cytoscape instance.
//   - Render the shadow panel from /shadow response data.
//   - Render the element detail panel from /element/{name} response data.
//
// This module does NOT perform any fetch calls; it only consumes data already
// retrieved by app.js. All DOM IDs match index.html exactly.

/* === Cytoscape instance (module-level singleton) === */
let cy = null;

/**
 * initGraph — initialises or re-initialises the Cytoscape instance inside #cy.
 *
 * @param {Object} graphData — the MeshGraph JSON from /articulate data field.
 *   Structure: { nodes: { [name]: { appearance_count, shadow_count } }, edges: [...] }
 * @param {Function} onNodeClick — callback(nodeName) for tap events on nodes.
 */
function initGraph(graphData, onNodeClick) {
  const elements = buildCytoscapeElements(graphData);

  // Destroy the previous instance if one exists; prevents memory leaks on
  // repeated calls (e.g. when the user changes observer and reloads).
  if (cy) {
    cy.destroy();
    cy = null;
  }

  cy = cytoscape({
    container: document.getElementById('cy'),
    elements: elements,
    style: buildCytoscapeStyle(),
    layout: { name: 'cose', animate: false, padding: 40 },
    // Zoom and pan enabled so users can explore large graphs.
    zoomingEnabled: true,
    userZoomingEnabled: true,
    panningEnabled: true,
    userPanningEnabled: true,
  });

  // Wire up the node click handler.
  cy.on('tap', 'node', function(evt) {
    const nodeName = evt.target.id();
    onNodeClick(nodeName);
  });
}

/**
 * buildCytoscapeElements — converts a MeshGraph JSON object to a Cytoscape
 * elements array containing node and edge descriptors.
 *
 * Nodes: keyed by element name; label shows name + appearance count.
 * Edges: one Cytoscape edge per (source × target) pair per graph edge. The
 *        Cartesian-product expansion mirrors PrintGraphDOT's convention.
 *
 * @param {Object} graphData — MeshGraph data payload from /articulate.
 * @returns {Array} Cytoscape elements array.
 */
function buildCytoscapeElements(graphData) {
  const elements = [];

  // Nodes: graphData.nodes is an object keyed by element name.
  const nodes = graphData.nodes || {};
  for (const [name, node] of Object.entries(nodes)) {
    elements.push({
      data: {
        id: name,
        label: `${name} (${node.appearance_count || 0})`,
        appearanceCount: node.appearance_count || 0,
        shadowCount: node.shadow_count || 0,
      },
    });
  }

  // Edges: graphData.edges is an array of Edge objects with sources[] and targets[].
  const edges = graphData.edges || [];
  edges.forEach((edge, idx) => {
    const sources = edge.sources || [];
    const targets = edge.targets || [];
    // Expand to one Cytoscape edge per source × target pair.
    sources.forEach(src => {
      targets.forEach(tgt => {
        elements.push({
          data: {
            // Unique edge ID: traceID + src + tgt to avoid collisions.
            id: `${edge.trace_id || idx}-${src}-${tgt}`,
            source: src,
            target: tgt,
            label: truncateLabel(edge.what_changed || ''),
            whatChanged: edge.what_changed || '',
            mediation: edge.mediation || '',
            tags: edge.tags || [],
          },
        });
      });
    });
  });

  return elements;
}

/**
 * buildCytoscapeStyle — returns the Cytoscape stylesheet array.
 * Nodes are rendered as rectangles with count labels; edges as directed arcs.
 */
function buildCytoscapeStyle() {
  return [
    {
      selector: 'node',
      style: {
        'background-color': '#2980b9',
        'label': 'data(label)',
        'color': '#fff',
        'text-valign': 'center',
        'text-halign': 'center',
        'shape': 'roundrectangle',
        'width': 'label',
        'height': 'label',
        'padding': '10px',
        'font-size': '12px',
      },
    },
    {
      // Highlight selected node.
      selector: 'node:selected',
      style: {
        'background-color': '#1a5276',
        'border-width': 2,
        'border-color': '#f39c12',
      },
    },
    {
      selector: 'edge',
      style: {
        'width': 2,
        'line-color': '#7f8c8d',
        'target-arrow-color': '#7f8c8d',
        'target-arrow-shape': 'triangle',
        'curve-style': 'bezier',
        'label': 'data(label)',
        'font-size': '10px',
        'color': '#555',
        'text-rotation': 'autorotate',
        'text-background-color': '#fafafa',
        'text-background-opacity': 0.8,
        'text-background-padding': '2px',
      },
    },
  ];
}

/**
 * truncateLabel — truncates a string to maxLen runes, appending "…" if needed.
 * Mirrors the Go graph.truncateLabel constant (28 chars).
 *
 * @param {string} s — input string.
 * @param {number} maxLen — maximum length before truncation (default 28).
 * @returns {string}
 */
function truncateLabel(s, maxLen = 28) {
  if (!s) return '';
  const runes = [...s]; // spread handles multi-byte Unicode correctly.
  if (runes.length <= maxLen) return s;
  // Use "..." (three ASCII dots) to match graph.truncateLabel in export.go —
  // the canonical truncation character. This keeps UI labels consistent with
  // DOT/Mermaid exports and the CLI --format json output.
  return runes.slice(0, maxLen).join('') + '...';
}

/**
 * renderShadowPanel — populates the #shadow-list element with shadow items.
 * Each item shows the element name (clickable → triggers detail fetch) and
 * reason chips.
 *
 * @param {Array} shadowData — array of ShadowElement objects from /shadow data.
 * @param {Function} onShadowClick — callback(elementName) when an item is clicked.
 */
function renderShadowPanel(shadowData, onShadowClick) {
  const list = document.getElementById('shadow-list');
  list.innerHTML = '';

  if (!shadowData || shadowData.length === 0) {
    const empty = document.createElement('p');
    empty.className = 'shadow-empty';
    empty.textContent = 'No shadow — all elements visible from this position.';
    list.appendChild(empty);
    return;
  }

  shadowData.forEach(se => {
    const item = document.createElement('div');
    item.className = 'shadow-item';

    // Name — clickable to load detail.
    const nameEl = document.createElement('div');
    nameEl.className = 'shadow-item-name';
    nameEl.textContent = se.name;
    nameEl.addEventListener('click', () => onShadowClick(se.name));
    item.appendChild(nameEl);

    // Reason chips.
    if (se.reasons && se.reasons.length > 0) {
      const reasonsEl = document.createElement('div');
      reasonsEl.className = 'shadow-reasons';
      se.reasons.forEach(reason => {
        const chip = document.createElement('span');
        chip.className = 'shadow-reason-chip';
        chip.textContent = reason;
        reasonsEl.appendChild(chip);
      });
      item.appendChild(reasonsEl);
    }

    list.appendChild(item);
  });
}

/**
 * renderDetailPanel — populates the #detail-content element with trace cards
 * for the selected element.
 *
 * Each trace card shows: what_changed, timestamp, observer, source[], target[],
 * mediation (if present), tags. Session-promoted traces (tagged "session") get
 * a provenance block.
 *
 * @param {string} elementName — the name of the element whose traces are shown.
 * @param {Array} traces — array of schema.Trace objects from /element/{name}.
 * @param {string} observer — the current observer position (for "no traces" message).
 */
function renderDetailPanel(elementName, traces, observer) {
  const content = document.getElementById('detail-content');
  content.innerHTML = '';

  if (!traces || traces.length === 0) {
    const msg = document.createElement('p');
    msg.className = 'detail-empty';
    msg.textContent = `No traces from observer "${observer}" mention "${elementName}".`;
    content.appendChild(msg);
    return;
  }

  traces.forEach(trace => {
    const card = buildTraceCard(trace);
    content.appendChild(card);
  });
}

/**
 * buildTraceCard — creates a DOM element for a single trace.
 *
 * @param {Object} trace — schema.Trace JSON object.
 * @returns {HTMLElement}
 */
function buildTraceCard(trace) {
  const card = document.createElement('div');
  card.className = 'trace-card';

  // What changed — primary descriptor.
  const whatEl = document.createElement('div');
  whatEl.className = 'trace-card-what';
  whatEl.textContent = trace.what_changed || '(no description)';
  card.appendChild(whatEl);

  // Timestamp formatted as locale string.
  const tsEl = document.createElement('div');
  tsEl.className = 'trace-card-meta';
  const ts = trace.timestamp ? new Date(trace.timestamp).toLocaleString() : '—';
  tsEl.textContent = `When: ${ts}`;
  card.appendChild(tsEl);

  // Observer.
  const obsEl = document.createElement('div');
  obsEl.className = 'trace-card-meta';
  obsEl.textContent = `Observer: ${trace.observer || '—'}`;
  card.appendChild(obsEl);

  // Source and target arrays.
  const srcEl = document.createElement('div');
  srcEl.className = 'trace-card-meta';
  srcEl.textContent = `Source: ${(trace.source || []).join(', ') || '—'}`;
  card.appendChild(srcEl);

  const tgtEl = document.createElement('div');
  tgtEl.className = 'trace-card-meta';
  tgtEl.textContent = `Target: ${(trace.target || []).join(', ') || '—'}`;
  card.appendChild(tgtEl);

  // Mediation — only shown when present.
  if (trace.mediation) {
    const medEl = document.createElement('div');
    medEl.className = 'trace-card-meta';
    medEl.textContent = `Mediation: ${trace.mediation}`;
    card.appendChild(medEl);
  }

  // Tags row.
  if (trace.tags && trace.tags.length > 0) {
    const tagsRow = document.createElement('div');
    tagsRow.className = 'trace-card-tags';
    trace.tags.forEach(tag => {
      const chip = document.createElement('span');
      chip.className = 'trace-tag';
      chip.textContent = tag;
      tagsRow.appendChild(chip);
    });
    card.appendChild(tagsRow);
  }

  // Provenance block: shown when the trace carries the "session" tag.
  // Signals that this trace was promoted from an LLM SessionRecord — the
  // observation apparatus entered the mesh (Principle 8 reflexivity).
  //
  // SAFETY NOTE: innerHTML is used here (and ONLY here) to produce bold/italic
  // markup in the provenance block. The only dynamic value interpolated is
  // trace.what_changed, which is passed through escapeHTML() before insertion.
  // All other fields in buildTraceCard use textContent, which is always safe.
  // Do NOT extend this pattern to additional fields without also wrapping them
  // in escapeHTML(); prefer textContent + DOM manipulation for new fields.
  const isSession = trace.tags && trace.tags.includes('session');
  if (isSession) {
    const prov = document.createElement('div');
    prov.className = 'trace-provenance';
    prov.innerHTML = `<strong>Provenance:</strong> Promoted from LLM session — <em>${escapeHTML(trace.what_changed || '')}</em>`;
    card.appendChild(prov);
  }

  return card;
}

/**
 * escapeHTML — minimal HTML escaping to prevent XSS in innerHTML fragments.
 * Used only for the provenance block (all other content uses textContent).
 *
 * @param {string} s — raw string from trace data.
 * @returns {string}
 */
function escapeHTML(s) {
  return s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}
