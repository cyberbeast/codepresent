---
name: codepresent
description: >-
  Present interactive, step-by-step code walkthroughs in the connected editor
  (Antigravity IDE / VS Code) or terminal TUI using the codepresent MCP tools.
  Use when the user asks to present, tour, or walk through files or code flows.
---

# codepresent Walkthroughs

Use the `codepresent` MCP server tools (`present_code_story`, `control_presentation`, `get_presentation_status`) to drive a live visual storyboard instead of dumping large static code blocks into chat.

## Workflow

1. **Read the target files first.** Always inspect the exact files you plan to present so every `file_path`, `start_line`, and `end_line` is 100% accurate.
2. **Break explanations into focused steps.**
   - Aim for **5–15 lines** per step (`start_line` to `end_line`, 1-indexed inclusive).
   - Split long functions or multi-file flows into sequential steps so the editor spotlight and SVG connector stay focused.
3. **Call `present_code_story`.**
   - `story.story_title`: Concise title for the walkthrough.
   - `story.workspace_root`: Absolute path to the workspace root so relative `file_path` values resolve cleanly.
   - `story.steps[]`:
     - `file_path`: Path relative to `workspace_root` (or absolute).
     - `start_line` / `end_line`: 1-indexed line range (`>= 1`, `end_line >= start_line`).
     - `focus_target`: Symbol, function, or expression name being spotlighted.
     - `title`: Short step title (e.g., `"1. Engine & WebSocket Bridge"`).
     - `narration`: Clear, concise explanation synchronized with the highlighted lines.
     - `duration_seconds`: Optional auto-advance duration (defaults to `6`).
   - `auto_play`: Defaults to `true`. Set to `false` if the user prefers manual stepping.
4. **Control playback when asked.** Use `control_presentation` with `action` set to `play`, `pause`, `next`, `prev`, `stop`, or `goto` (with 0-indexed `step_index`), and `get_presentation_status` to inspect the current step and playback state.
