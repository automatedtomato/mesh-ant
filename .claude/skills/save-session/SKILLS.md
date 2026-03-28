---
name: save-session
description: Save a detailed handover document of the current session to tmp/session-memory/session-{datetime}.md. Use when you want to preserve session context across compaction or conversation boundaries.
---

# Save Session

Write a detailed handover document for the current session and save it to `tmp/session-memory/`.

## Steps

1. **Get the current datetime** by running:
   ```bash
   date +"%Y-%m-%dT%H-%M-%S"
   ```

2. **Create the directory** if it does not exist:
   ```bash
   mkdir -p tmp/session-memory
   ```

3. **Write the handover document** to `tmp/session-memory/session-{datetime}.md`.

   The document MUST cover:

   ### Session Handover Document Structure

   ```markdown
   # Session Handover — {datetime}

   ## Active Branch
   Current git branch and whether it is clean or has uncommitted changes.

   ## What Was Being Built
   The feature, issue, or task being worked on. Include GitHub issue numbers.

   ## Current State
   Where in the implementation we are — e.g., "tests written (RED), implementation in progress",
   "implementation complete, code review next", "PR open, awaiting merge", etc.

   ## Files Modified
   List every file touched in this session with a one-line description of what changed.

   ## Key Design Decisions
   Any non-obvious decisions made (with rationale), patterns established, or trade-offs accepted.

   ## Known Issues / Blockers
   Any failing tests, unresolved errors, or open questions.

   ## Immediate Next Step
   The single most important thing to do when resuming.

   ## Full Context
   A thorough narrative of what happened in this session — what was attempted, what worked,
   what failed, what was discovered. This section should be detailed enough for a cold-start
   resume without needing to re-read git history.

   ## Commands to Verify State
   Shell commands to quickly confirm the current state, e.g.:
   - `go test ./...`
   - `git status`
   - `gh pr list`
   ```

4. **Confirm** by printing the path of the saved file.

## Notes

- The datetime format uses `-` instead of `:` so the filename is shell-safe.
- Do not truncate the "Full Context" section — it is the most important part for cold-start resumes.
- If multiple sessions exist in `tmp/session-memory/`, the newest filename (lexicographic sort) is the most recent.
