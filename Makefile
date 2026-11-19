BIN_DIR := bin
DIST_DIR := dist
BINARY := $(BIN_DIR)/codepresent
VSIX := $(DIST_DIR)/codepresent-vscode.vsix
PLUGIN_DIR := $(HOME)/.gemini/config/plugins/codepresent

ANTIGRAVITY_IDE_CLI := $(shell command -v antigravity-ide 2>/dev/null || echo "/Applications/Antigravity IDE.app/Contents/Resources/app/bin/antigravity-ide")
VSCODE_CLI := $(shell command -v code 2>/dev/null || echo "code")

.PHONY: build test vsix install-plugin install-antigravity-ide install-vscode install-antigravity clean

build:
	@mkdir -p $(BIN_DIR)
	go build -o $(BINARY) ./cmd/codepresent

test:
	go test -count=1 ./...
	npm --prefix editors/vscode test

vsix:
	@mkdir -p $(DIST_DIR)
	cd editors/vscode && printf 'y\n' | npx --yes @vscode/vsce package --allow-missing-repository --out "$(CURDIR)/$(VSIX)"

install-plugin: build
	@mkdir -p "$(PLUGIN_DIR)/skills/codepresent"
	cp plugins/codepresent/plugin.json "$(PLUGIN_DIR)/plugin.json"
	cp plugins/codepresent/README.md "$(PLUGIN_DIR)/README.md"
	cp plugins/codepresent/skills/codepresent/SKILL.md "$(PLUGIN_DIR)/skills/codepresent/SKILL.md"
	printf '{\n  "mcpServers": {\n    "codepresent": {\n      "command": "%s"\n    }\n  }\n}\n' "$(CURDIR)/$(BINARY)" > "$(PLUGIN_DIR)/mcp_config.json"
	@echo "Installed Antigravity plugin to $(PLUGIN_DIR)"

install-antigravity-ide: vsix
	@test -x "$(ANTIGRAVITY_IDE_CLI)" || { echo "Antigravity IDE CLI not found at $(ANTIGRAVITY_IDE_CLI)"; exit 1; }
	"$(ANTIGRAVITY_IDE_CLI)" --install-extension "$(CURDIR)/$(VSIX)"

install-vscode: vsix
	"$(VSCODE_CLI)" --install-extension "$(CURDIR)/$(VSIX)"

install-antigravity: build install-plugin install-antigravity-ide

clean:
	rm -rf $(BIN_DIR) $(DIST_DIR)
