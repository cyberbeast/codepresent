# codepresent Antigravity Plugin

This plugin bundles the `codepresent` MCP server configuration and an agent skill that teaches Antigravity how to drive interactive, step-by-step code walkthroughs.

## Installation

From the repository root, run:

```bash
make install-antigravity
```

This will:
1. Build `bin/codepresent`.
2. Install this plugin to `~/.gemini/config/plugins/codepresent` with `mcp_config.json` pointing to the built `bin/codepresent` binary.
3. Package `editors/vscode` as a `.vsix` and install it into **Antigravity IDE** (`antigravity-ide`).
