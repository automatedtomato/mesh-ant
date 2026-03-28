---
name: resume-session
description: Find the latest session handover document in tmp/session-memory/, read it to restore context, then delete it. Use at the start of a new conversation to pick up where the previous session left off.
---

# Resume Session

Load context from the most recent saved session handover document.

## Steps

1. **Find the latest handover file**:
   ```bash
   ls tmp/session-memory/session-*.md 2>/dev/null | sort | tail -1
   ```

   If no files are found, report that there is no saved session and stop.

2. **Read the file** using the Read tool on the path returned above.

3. **Internalise the context** — do not just quote the document back. Instead:
   - Announce which session is being resumed (datetime from the filename)
   - Summarise the active branch, current state, and immediate next step in 2–3 sentences
   - Ask the user: "Ready to continue. Shall I proceed with [immediate next step]?"

4. **Delete the file** after reading:
   ```bash
   rm tmp/session-memory/session-{datetime}.md
   ```

5. **Confirm deletion** to the user.

## Notes

- If multiple files exist, always load the **newest** (last in sorted order).
- Do not delete without reading first — the read is the point.
- After resuming, follow the project's per-issue pipeline from wherever the session left off.
  Check `tasks/todo.md` if the handover document is ambiguous about the current milestone state.
