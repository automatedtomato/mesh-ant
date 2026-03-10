#!/usr/bin/env node
/**
 * extract-patterns.js — Stop (lifecycle) hook
 *
 * Runs when the Claude Code session ends.
 * Outputs a structured prompt that triggers Claude to evaluate the session
 * for extractable patterns and update project memory before closing.
 *
 * Also resets the compact-suggest counter so the next session starts fresh.
 *
 * Pattern extraction targets:
 *   - Design decisions made (schema, architecture, naming)
 *   - MeshAnt-specific patterns (trace vocabulary, ANT concepts in code)
 *   - Corrections to previously recorded memory
 *   - Workflow patterns worth preserving (tool sequences, debugging approaches)
 *   - New open questions or deferred decisions
 */

const fs   = require('fs');
const os   = require('os');
const path = require('path');

// Reset the tool-call counter for the next session
const counterFile = path.join(os.tmpdir(), 'mesh-ant-tool-calls');
try { fs.unlinkSync(counterFile); } catch (_) {}

// Derive the project memory path
const homeDir     = os.homedir();
const memoryDir   = path.join(
  homeDir,
  '.claude/projects/-home-automatedtomato-github-com-automatedtomato-mesh-ant/memory'
);
const memoryFile  = path.join(memoryDir, 'MEMORY.md');

const prompt = `
[session-end: pattern-extraction]

Before this session closes, evaluate what happened and decide whether
project memory needs updating. Check each category:

1. DESIGN DECISIONS
   Were schema, architecture, or API decisions made or changed?
   → Update MEMORY.md "Key schema decisions" or add a new section.

2. MESHANT-SPECIFIC PATTERNS
   Did any new trace-first, ANT-inspired, or MeshAnt conventions emerge
   in code or discussion that should carry forward?
   → Add to MEMORY.md or a linked topic file.

3. CORRECTIONS
   Was anything in memory wrong or outdated?
   → Fix it at the source — do not let the mistake persist.

4. WORKFLOW PATTERNS
   Did any tool sequences, debugging approaches, or process steps
   prove especially effective or ineffective?
   → Record in MEMORY.md or memory/workflow.md if substantial.

5. OPEN QUESTIONS / DEFERRED DECISIONS
   Were any decisions explicitly deferred for later discussion?
   → Record them so they surface in the next session.

Memory location: ${memoryFile}

If nothing notable happened this session, no update is needed.
Prefer a short, accurate update over a long, speculative one.
`.trim();

process.stdout.write(prompt + '\n');
