# Development Workflow

> This file covers the development process that happens before git operations.
> For commit conventions, see [git-workflow.md](./git-workflow.md).
> For the full GitHub process (issues, branches, PRs, review), see [github-workflow.md](./github-workflow.md).

Two pipeline levels: **per-issue** (each child issue) and **per-thread** (when all child issues are merged).

---

## Per-Issue Pipeline

Each child issue follows this sequence in order. Do not skip steps.

### 0. Research & Reuse _(mandatory before any new implementation)_
- **GitHub code search first:** `gh search repos` and `gh search code` for existing implementations, templates, and patterns.
- **Exa MCP for research:** Use `exa-web-search` MCP for broader research and discovering prior art.
- **Check package registries:** Search npm, PyPI, crates.io, etc. before writing utility code. Prefer battle-tested libraries.
- Prefer adopting or porting a proven approach over writing net-new code when it meets the requirement.

### 1. Plan
- Use **planner** agent to create an implementation plan for the issue scope.
- Identify dependencies, risks, and edge cases.
- **Wait for user confirmation before touching any code.**

### 2. Orchestrate
- Use **orchestrate** skill to chain: planner → tdd-guide → code-reviewer → security-reviewer.
- Ensures the full pipeline runs before committing.

### 3. TDD
- Use **tdd-guide** agent.
- Write tests first (RED) → implement to pass (GREEN) → refactor (IMPROVE).
- Verify 80%+ coverage before proceeding.

### 4. Code Review
- Use **code-reviewer** agent immediately after writing code.
- Address CRITICAL and HIGH issues; fix MEDIUM issues when possible.
- Fix all issues before proceeding.

### 5. Commit, Push, Open PR
- Follow conventional commits — see [git-workflow.md](./git-workflow.md).
- Push branch and open PR targeting `develop` — see [github-workflow.md](./github-workflow.md).
- All work tied to a GitHub Issue.

### 6. Recursive Review: ant-theorist + qa-engineer
- Run **ant-theorist** agent on the PR diff: check for ANT violations, naming concerns, language discipline.
- Run **qa-engineer** agent on the PR diff: check test quality, behavioral completeness, edge case coverage.
- Fix any issues found; repeat until both agents return ALIGNED / PASS.

### 7. All Green → SHIP
- `go test ./...` green, `go vet ./...` clean, both review agents satisfied.
- PR is ready to merge.

### 8. Final Review: architect
- Run **architect** agent for a final structural review of the PR.
- Confirm the change fits the overall architecture and deferred items are documented.
- Address any architectural concerns before merging.

### 9. Update Docs (if needed)
- Update `docs/CODEMAPS/meshant.md` if new files, types, or functions were added.
- Update `tasks/todo.md` to mark the issue complete.
- Update any affected decision records or guides.

### 10. Merge into `develop`
- Merge the PR after all reviews pass and docs are current.

---

## Per-Thread Pipeline

When **all child issues** for a thread are merged into `develop`:

### 1. Refactor Clean
- Run **refactor-cleaner** agent across the thread's new code.
- Remove dead code, unused exports, duplication introduced across PRs.

### 2. Philosophical Review
- Run **ant-theorist** agent across the full thread's changes.
- Check for accumulated ANT violations or language drift across the whole thread.
- Fix any issues found.

### 3. Update Docs
- Update `tasks/todo.md` — mark the thread complete.
- Add or update any decision records, guides, or `docs/CODEMAPS/meshant.md` entries that span the thread.
- Update `README.md` if user-facing behaviour changed.

### 4. Merge Parent Issue Branch into `develop`
- Open a PR for the thread-level branch if one exists, or confirm all child PRs are merged.
- Merge after docs are current and refactor-clean + philosophical review pass.
