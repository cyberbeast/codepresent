# codepresent

**codepresent** is an interactive presentation and code-tour engine for AI agents.

Instead of dumping static text snippets into a chat window, `codepresent` allows AI agents (via MCP) to dynamically open files, spotlight relevant functions with focused dimming, and step through code execution paths in synchronization with narration—controllable with play/pause and step-by-step navigation.

## Architecture

- **Core Engine & MCP Server (`go`)**: Implements the Model Context Protocol (MCP) server, state management (play/pause, timers, steps), and an optional terminal TUI (Bubble Tea).
- **Editor Extension (`vscode` / `typescript`)**: A lightweight display driver for VS Code / Cursor that receives visual commands over local IPC/WebSocket and renders non-destructive text editor decorations and smooth scrolling.
- **Agent Interoperability**: Compatible with Claude Desktop, Cursor, Claude Code, Antigravity, and any standard MCP client.

## Roadmap

- [ ] Go MCP Server implementation using official SDK
- [ ] VS Code Extension with spotlight decorations & floating narration cards
- [ ] Terminal TUI presentation mode with Bubble Tea
- [ ] Play/Pause and auto-advance controls
