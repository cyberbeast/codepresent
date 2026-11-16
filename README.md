# codepresent

**codepresent** is an interactive presentation and code-tour engine for AI agents.

Instead of dumping static text snippets into a chat window, `codepresent` allows AI agents (via MCP) to drive a unified visual storyboard: opening files, spotlighting relevant code ranges, and connecting each step to the exact lines being explained—with Play/Pause, step-by-step navigation, and clickable timeline jumps.

## Architecture

- **`cmd/codepresent/`**: CLI & MCP server entrypoint (`stdio` MCP transport + local WebSocket bridge).
- **`pkg/presentation/`**: Thread-safe `Engine` managing `Story` validation, step navigation, auto-advance timers, and event broadcasting.
- **`pkg/mcpserver/`**: MCP server built on the official Go SDK (`github.com/modelcontextprotocol/go-sdk`), exposing `present_code_story`, `control_presentation`, and `get_presentation_status`.
- **`pkg/bridge/`**: Local WebSocket server (`ws://127.0.0.1:9123/ws` by default) that streams presentation events to connected editor display drivers and dispatches playback commands back to the engine.
- **`pkg/tui/`**: Bubble Tea terminal presentation viewer (`chroma/v2` syntax highlighting, per-step colors, and a Unicode box-drawing connector).
- **`editors/vscode/`**: VS Code / Cursor extension display driver rendering the unified storyboard, syntax-highlighted code spotlight, per-step color palette, and live SVG Bezier connector.

## Prerequisites

- **Go** 1.24+
- **Node.js** 20+ & **npm** (for the VS Code extension)
- **VS Code** or **Cursor** (for running the Extension Development Host)

## Development & Local Testing

### 1. Run Backend & Extension Tests

Run all Go backend unit and integration tests (presentation engine, WebSocket bridge, MCP server tools, and demo story bounds):

```bash
go test -count=1 -v ./...
```

Compile and run the VS Code extension unit tests (using Node's built-in test runner):

```bash
npm --prefix editors/vscode install
npm --prefix editors/vscode test
```

### 2. Interactive Local Demo (`F5` or Terminal TUI)

You can test the full end-to-end presentation experience against this repository using the built-in `--demo` walkthrough:

#### Option A: Terminal TUI (Bubble Tea)
```bash
go run ./cmd/codepresent --demo --tui
```
* Controls: `space`/`p` (play/pause), `←`/`→` or `h`/`l` (prev/next), `↑`/`↓` or `j`/`k` (scroll), `1`–`9` (jump to step), `q` (quit).
* To attach the TUI to a separate running `codepresent` MCP server over WebSocket, run `go run ./cmd/codepresent --tui`.

#### Option B: VS Code / Cursor Extension (`F5`)

1. Start the Go server in demo mode:
   ```bash
   go run ./cmd/codepresent --demo
   ```
2. Open this repository in **VS Code** or **Cursor** and press **`F5`** (`Run → Start Debugging → Run codepresent Extension`).
   - `.vscode/tasks.json` compiles `editors/vscode` automatically.
   - `.vscode/launch.json` launches an Extension Development Host window that connects to `ws://127.0.0.1:9123/ws`.
3. Use **Play / Pause / Prev / Next / Stop** or click any step in the left storyboard timeline to jump between files and spotlighted code ranges.

### 3. Connecting an MCP Client

Build the binary:

```bash
go build -o bin/codepresent ./cmd/codepresent
```

Configure your MCP client (e.g., Claude Desktop, Cursor, or CLI agent) to run `bin/codepresent` over `stdio`:

```json
{
  "mcpServers": {
    "codepresent": {
      "command": "/path/to/codepresent/bin/codepresent"
    }
  }
}
```

Optional flags:
- `--ws-addr`: WebSocket address for editor display drivers (default `127.0.0.1:9123`).
- `--demo`: Load the built-in 3-step self-tour without starting the MCP stdio server.
- `--tui`: Launch the Bubble Tea terminal presentation viewer (standalone with `--demo`, or connected via WebSocket to `--ws-addr`).

## Roadmap

- [x] Go MCP Server implementation using official SDK
- [x] Local two-way WebSocket bridge (`pkg/bridge`)
- [x] VS Code Extension with unified storyboard, spotlighting, and SVG connector
- [x] Terminal TUI presentation mode with Bubble Tea (`pkg/tui`)
