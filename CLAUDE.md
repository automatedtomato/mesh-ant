# CLAUDE.md

This repository is **MeshAnt** — an experimental, trace-first framework for analyzing and
eventually simulating socio-technical networks, inspired by Bruno Latour and Actor-Network Theory.

This file is a **catalog**, not a specification. Read the documents it points to.

---

## Project intent and philosophy

- `README.md` — description, principles overview, usage
- `docs/manifesto.md` — why this project exists
- `docs/principles.md` — 8 design principles in detail
- `docs/ant-notes.md` — ANT theoretical grounding
- `docs/directions.md` — strategic direction: three forms, version targets, core inversion

---

## Architecture and implementation

- `docs/CODEMAPS/meshant.md` — package map: what lives where, key types, entry points
- `docs/decisions/` — decision records for each milestone (schema, articulation, diff, etc.)

---

## Active work

- `tasks/todo.md` — milestone tracking, all tasks
- `tasks/plan_m*.md` — detailed plans for in-progress milestones

When new work is requested, record it in `tasks/todo.md`.

---

## Reference

- `reference/miro-fish/` — reference project (actor-first, do NOT copy patterns blindly)

---

## Agent tooling

- `.claude/rules/` — always-follow guidelines (security, style, testing, git workflow)
- `.claude/agents/` — specialized subagents (planner, code-reviewer, tdd-guide, etc.)
- `.claude/skills/` — workflow definitions (orchestrate, plan, tdd, etc.)

---

## Working style

Prefer small, legible, conceptually consistent steps.
Let MeshAnt's own documents define the project — use reference materials to inform, not dictate.
