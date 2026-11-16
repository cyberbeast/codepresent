import * as fs from 'fs/promises';
import * as vscode from 'vscode';
import WebSocket from 'ws';
import {
	PresentationEvent,
	PresentationState,
	renderCanvasHtml,
	resolveStepFilePath,
} from './renderer';

export function activate(context: vscode.ExtensionContext) {
	const controller = new CodePresentController(context);
	context.subscriptions.push(controller);

	context.subscriptions.push(
		vscode.commands.registerCommand('codepresent.connect', () => controller.connect()),
		vscode.commands.registerCommand('codepresent.play', () => controller.sendCommand('play')),
		vscode.commands.registerCommand('codepresent.pause', () => controller.sendCommand('pause')),
		vscode.commands.registerCommand('codepresent.next', () => controller.sendCommand('next')),
		vscode.commands.registerCommand('codepresent.prev', () => controller.sendCommand('prev')),
		vscode.commands.registerCommand('codepresent.stop', () => controller.sendCommand('stop'))
	);

	controller.connect();
}

export function deactivate() {
	// Cleanup handled by Disposable subscriptions.
}

class CodePresentController implements vscode.Disposable {
	private ws: WebSocket | undefined;
	private reconnectTimer: NodeJS.Timeout | undefined;
	private disposed = false;

	private panel: vscode.WebviewPanel | undefined;
	private latestState: PresentationState | undefined;

	private readonly prevBtn: vscode.StatusBarItem;
	private readonly playPauseBtn: vscode.StatusBarItem;
	private readonly nextBtn: vscode.StatusBarItem;
	private readonly infoItem: vscode.StatusBarItem;
	private readonly stopBtn: vscode.StatusBarItem;
	private readonly connStatusItem: vscode.StatusBarItem;
	private readonly output: vscode.OutputChannel;

	constructor(private readonly context: vscode.ExtensionContext) {
		this.output = vscode.window.createOutputChannel('codepresent');
		this.output.appendLine('codepresent extension activated');

		this.connStatusItem = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Left, 99);
		this.connStatusItem.text = '$(plug) codepresent: connecting...';
		this.connStatusItem.tooltip = 'Click to reconnect to codepresent Go server';
		this.connStatusItem.command = 'codepresent.connect';
		this.connStatusItem.show();

		this.prevBtn = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Left, 104);
		this.prevBtn.text = '$(chevron-left)';
		this.prevBtn.tooltip = 'codepresent: Previous Step';
		this.prevBtn.command = 'codepresent.prev';

		this.playPauseBtn = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Left, 103);
		this.playPauseBtn.command = 'codepresent.play';

		this.nextBtn = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Left, 102);
		this.nextBtn.text = '$(chevron-right)';
		this.nextBtn.tooltip = 'codepresent: Next Step';
		this.nextBtn.command = 'codepresent.next';

		this.infoItem = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Left, 101);

		this.stopBtn = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Left, 100);
		this.stopBtn.text = '$(close)';
		this.stopBtn.tooltip = 'codepresent: Stop Presentation';
		this.stopBtn.command = 'codepresent.stop';
	}

	public connect(): void {
		if (this.disposed) {
			return;
		}
		this.clearReconnect();
		if (this.ws && (this.ws.readyState === WebSocket.OPEN || this.ws.readyState === WebSocket.CONNECTING)) {
			return;
		}

		const serverUrl = vscode.workspace.getConfiguration('codepresent').get<string>(
			'serverUrl',
			'ws://127.0.0.1:9123/ws'
		);

		this.output.appendLine(`Connecting to ${serverUrl}...`);
		this.connStatusItem.text = '$(sync~spin) codepresent: connecting';

		const socket = new WebSocket(serverUrl);
		this.ws = socket;

		socket.on('open', () => {
			this.output.appendLine('Connected to codepresent server');
			this.connStatusItem.text = '$(radio-tower) codepresent: connected';
		});

		socket.on('message', (raw: WebSocket.RawData) => {
			try {
				const event: PresentationEvent = JSON.parse(raw.toString());
				void this.handleEvent(event);
			} catch (err) {
				this.output.appendLine(`Failed to parse event: ${String(err)}`);
			}
		});

		socket.on('close', () => {
			this.ws = undefined;
			this.connStatusItem.text = '$(plug) codepresent: waiting for server';
			this.scheduleReconnect();
		});

		socket.on('error', (err: Error) => {
			this.output.appendLine(`WebSocket error: ${err.message}`);
		});
	}

	public sendCommand(action: 'play' | 'pause' | 'next' | 'prev' | 'stop' | 'goto', stepIndex?: number): void {
		if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
			return;
		}
		this.ws.send(JSON.stringify({ action, step_index: stepIndex }));
	}

	private scheduleReconnect(): void {
		if (this.disposed || this.reconnectTimer) {
			return;
		}
		this.reconnectTimer = setTimeout(() => {
			this.reconnectTimer = undefined;
			this.connect();
		}, 3000);
	}

	private clearReconnect(): void {
		if (this.reconnectTimer) {
			clearTimeout(this.reconnectTimer);
			this.reconnectTimer = undefined;
		}
	}

	private async handleEvent(event: PresentationEvent): Promise<void> {
		if (event.type === 'stopped' || !event.state.current_step) {
			this.clearPresentation();
			return;
		}

		this.latestState = event.state;
		this.updateStatusBar(event.state);
		await this.updateCanvasPanel(event.state);
	}

	private resolveFilePath(filePath: string, workspaceRoot?: string): string | undefined {
		const folderPaths = (vscode.workspace.workspaceFolders ?? []).map((f) => f.uri.fsPath);
		return resolveStepFilePath(filePath, folderPaths, workspaceRoot);
	}

	private updateStatusBar(state: PresentationState): void {
		const step = state.current_step;
		if (!step) {
			return;
		}

		this.playPauseBtn.text = state.is_playing ? '$(debug-pause)' : '$(play)';
		this.playPauseBtn.tooltip = state.is_playing ? 'codepresent: Pause' : 'codepresent: Play';
		this.playPauseBtn.command = state.is_playing ? 'codepresent.pause' : 'codepresent.play';

		this.infoItem.text = `$(eye) ${state.current_step_index + 1}/${state.total_steps}: ${step.title}`;
		this.infoItem.tooltip = state.story_title;

		this.prevBtn.show();
		this.playPauseBtn.show();
		this.nextBtn.show();
		this.infoItem.show();
		this.stopBtn.show();
	}

	private async updateCanvasPanel(state: PresentationState): Promise<void> {
		const step = state.current_step;
		if (!step) {
			return;
		}

		if (!this.panel) {
			this.panel = vscode.window.createWebviewPanel(
				'codepresentCanvas',
				'codepresent',
				{ viewColumn: vscode.ViewColumn.One, preserveFocus: false },
				{ enableScripts: true, retainContextWhenHidden: true }
			);

			this.panel.webview.onDidReceiveMessage((msg: { action?: string; step_index?: number }) => {
				if (msg.action === 'open_editor') {
					void this.openCurrentStepInEditor();
					return;
				}
				if (msg.action === 'goto' && typeof msg.step_index === 'number') {
					this.sendCommand('goto', msg.step_index);
					return;
				}
				if (msg.action === 'play' || msg.action === 'pause' || msg.action === 'next' || msg.action === 'prev' || msg.action === 'stop') {
					this.sendCommand(msg.action);
				}
			});

			this.panel.onDidDispose(() => {
				this.panel = undefined;
			});
		}

		const resolvedPath = this.resolveFilePath(step.file_path, state.workspace_root);
		let fileContent = '';
		let readError = '';
		if (resolvedPath) {
			try {
				fileContent = await fs.readFile(resolvedPath, 'utf8');
			} catch (err) {
				readError = `Unable to read ${resolvedPath}: ${String(err)}`;
			}
		} else {
			readError = `Cannot resolve relative path "${step.file_path}": no workspace folder open.`;
		}

		this.panel.webview.html = renderCanvasHtml(state, step, fileContent, readError);
	}

	private async openCurrentStepInEditor(): Promise<void> {
		const state = this.latestState;
		const step = state?.current_step;
		if (!state || !step) {
			return;
		}
		const resolvedPath = this.resolveFilePath(step.file_path, state.workspace_root);
		if (!resolvedPath) {
			return;
		}
		const doc = await vscode.workspace.openTextDocument(vscode.Uri.file(resolvedPath));
		const editor = await vscode.window.showTextDocument(doc, {
			viewColumn: vscode.ViewColumn.Beside,
			preserveFocus: false,
		});
		const startLine = Math.max(0, step.start_line - 1);
		const endLine = Math.min(doc.lineCount - 1, Math.max(startLine, step.end_line - 1));
		const range = new vscode.Range(startLine, 0, endLine, doc.lineAt(endLine).text.length);
		editor.selection = new vscode.Selection(range.start, range.end);
		editor.revealRange(range, vscode.TextEditorRevealType.InCenter);
	}

	private clearPresentation(): void {
		this.latestState = undefined;
		this.prevBtn.hide();
		this.playPauseBtn.hide();
		this.nextBtn.hide();
		this.infoItem.hide();
		this.stopBtn.hide();

		if (this.panel) {
			this.panel.dispose();
			this.panel = undefined;
		}
	}

	public dispose(): void {
		this.disposed = true;
		this.clearReconnect();
		this.clearPresentation();
		this.prevBtn.dispose();
		this.playPauseBtn.dispose();
		this.nextBtn.dispose();
		this.infoItem.dispose();
		this.stopBtn.dispose();
		this.connStatusItem.dispose();
		this.output.dispose();
		if (this.ws) {
			this.ws.close();
			this.ws = undefined;
		}
	}
}
