import * as path from 'path';
import hljs from 'highlight.js';

export interface PresentationStep {
	step_number: number;
	file_path: string;
	start_line: number;
	end_line: number;
	focus_target?: string;
	title: string;
	narration: string;
	duration_seconds?: number;
}

export interface PresentationState {
	story_title: string;
	workspace_root?: string;
	current_step_index: number;
	total_steps: number;
	is_playing: boolean;
	current_step?: PresentationStep;
	steps?: PresentationStep[];
}

export interface PresentationEvent {
	type: 'story_loaded' | 'step_changed' | 'played' | 'paused' | 'stopped';
	state: PresentationState;
}

export const STEP_PALETTE = [
	{ hex: '#ffd54f', rgb: '255, 213, 79' },  // Gold
	{ hex: '#4fc3f7', rgb: '79, 195, 247' },  // Sky Cyan
	{ hex: '#69f0ae', rgb: '105, 240, 174' }, // Mint Green
	{ hex: '#b388ff', rgb: '179, 136, 255' }, // Lavender
	{ hex: '#ff8a65', rgb: '255, 138, 101' }, // Coral
];

export function stepColor(index: number): { hex: string; rgb: string } {
	return STEP_PALETTE[((index % STEP_PALETTE.length) + STEP_PALETTE.length) % STEP_PALETTE.length];
}

export function resolveStepFilePath(
	filePath: string,
	workspaceFolderPaths: string[],
	workspaceRoot?: string
): string | undefined {
	if (path.isAbsolute(filePath)) {
		return filePath;
	}
	if (workspaceFolderPaths.length > 0) {
		return path.join(workspaceFolderPaths[0], filePath);
	}
	if (workspaceRoot) {
		return path.join(workspaceRoot, filePath);
	}
	return undefined;
}

export function renderCanvasHtml(
	state: PresentationState,
	step: PresentationStep,
	fileContent: string,
	readError: string
): string {
	const steps = state.steps && state.steps.length > 0 ? state.steps : [step];
	const progressPct = Math.round(((state.current_step_index + 1) / state.total_steps) * 100);
	const playLabel = state.is_playing ? '⏸ Pause' : '▶ Play';
	const playAction = state.is_playing ? 'pause' : 'play';
	const activePalette = stepColor(state.current_step_index);

	const timelineHtml = steps.map((s, idx) => {
		const isActive = idx === state.current_step_index;
		const isDone = idx < state.current_step_index;
		const nodeClass = isActive ? 'step-node active' : (isDone ? 'step-node done' : 'step-node');
		const cardId = isActive ? 'id="active-step-card"' : '';
		const palette = stepColor(idx);
		const focusBadge = s.focus_target
			? `<span class="badge">${escapeHtml(s.focus_target)}</span>`
			: '';
		const narrationBlock = isActive
			? `<div class="narration">${escapeHtml(s.narration)}</div>`
			: '';

		return `
			<div class="${nodeClass}" style="--step-hex: ${palette.hex}; --step-rgb: ${palette.rgb};" onclick="gotoStep(${idx})">
				<div class="rail">
					<div class="dot">${isDone ? '✓' : idx + 1}</div>
					${idx < steps.length - 1 ? '<div class="connector"></div>' : ''}
				</div>
				<div class="card" ${cardId}>
					<div class="card-header">
						<span class="step-title">${escapeHtml(s.title)}</span>
					</div>
					<div class="meta">
						<span>${escapeHtml(s.file_path)}:${s.start_line}-${s.end_line}</span>
						${focusBadge}
					</div>
					${narrationBlock}
				</div>
			</div>`;
	}).join('\n');

	let codeHtml = '';
	if (readError) {
		codeHtml = `<div class="error-box">${escapeHtml(readError)}</div>`;
	} else {
		const highlightedLines = highlightCodeLines(fileContent, step.file_path);
		const startIdx = Math.max(0, step.start_line - 1);
		const endIdx = Math.min(highlightedLines.length - 1, Math.max(startIdx, step.end_line - 1));

		const beforeHtml = highlightedLines
			.slice(0, startIdx)
			.map((line, i) => renderCodeLine(i + 1, line, true))
			.join('');
		const spotlightHtml = highlightedLines
			.slice(startIdx, endIdx + 1)
			.map((line, i) => renderCodeLine(startIdx + i + 1, line, false))
			.join('');
		const afterHtml = highlightedLines
			.slice(endIdx + 1)
			.map((line, i) => renderCodeLine(endIdx + i + 2, line, true))
			.join('');

		codeHtml = `
				<div class="code-lines">
					${beforeHtml}
					<div class="spotlight-group" id="spotlight-target">
						${spotlightHtml}
					</div>
					${afterHtml}
				</div>
			`;
	}

	return `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<style>
  :root {
    --accent-hex: ${activePalette.hex};
    --accent-rgb: ${activePalette.rgb};
  }
  * {
    box-sizing: border-box;
  }
  html, body {
    margin: 0;
    padding: 0;
    width: 100%;
    height: 100%;
    overflow: hidden;
    font-family: var(--vscode-font-family);
    color: var(--vscode-editor-foreground);
    background: var(--vscode-editor-background);
  }
  .layout {
    display: flex;
    width: 100%;
    height: 100%;
    position: relative;
  }
  .sidebar {
    width: 350px;
    flex-shrink: 0;
    display: flex;
    flex-direction: column;
    border-right: 1px solid var(--vscode-editorWidget-border);
    background: var(--vscode-sideBar-background, var(--vscode-editor-background));
    padding: 18px 18px 18px 20px;
    overflow-y: auto;
    z-index: 2;
  }
  .story-title {
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--vscode-descriptionForeground);
    margin-bottom: 8px;
  }
  .progress-track {
    width: 100%;
    height: 3px;
    background: var(--vscode-editorWidget-border);
    border-radius: 2px;
    margin-bottom: 14px;
    overflow: hidden;
    flex-shrink: 0;
  }
  .progress-fill {
    height: 100%;
    width: ${progressPct}%;
    background: var(--accent-hex);
    transition: width 0.25s ease, background-color 0.25s ease;
  }
  .controls {
    display: flex;
    gap: 6px;
    margin-bottom: 20px;
    flex-shrink: 0;
  }
  button {
    background: var(--vscode-button-background);
    color: var(--vscode-button-foreground);
    border: none;
    padding: 5px 11px;
    border-radius: 4px;
    cursor: pointer;
    font-size: 12px;
  }
  button:hover {
    background: var(--vscode-button-hoverBackground);
  }
  button.secondary {
    background: var(--vscode-button-secondaryBackground);
    color: var(--vscode-button-secondaryForeground);
  }
  .timeline {
    display: flex;
    flex-direction: column;
  }
  .step-node {
    display: flex;
    gap: 10px;
    cursor: pointer;
    opacity: 0.6;
    transition: opacity 0.15s ease;
  }
  .step-node:hover {
    opacity: 0.9;
  }
  .step-node.done {
    opacity: 0.78;
  }
  .step-node.active {
    opacity: 1;
  }
  .rail {
    display: flex;
    flex-direction: column;
    align-items: center;
    width: 22px;
    flex-shrink: 0;
  }
  .dot {
    width: 22px;
    height: 22px;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 11px;
    font-weight: 600;
    background: var(--vscode-editorWidget-background);
    border: 1px solid var(--vscode-editorWidget-border);
    color: var(--vscode-descriptionForeground);
  }
  .step-node.done .dot {
    border-color: rgba(var(--step-rgb), 0.55);
    color: var(--step-hex);
  }
  .step-node.active .dot {
    background: var(--step-hex);
    color: #1e1e1e;
    border-color: var(--step-hex);
    box-shadow: 0 0 8px rgba(var(--step-rgb), 0.45);
  }
  .connector {
    width: 2px;
    flex: 1;
    min-height: 14px;
    background: var(--vscode-editorWidget-border);
    margin: 4px 0;
  }
  .step-node.done .connector {
    background: rgba(var(--step-rgb), 0.4);
  }
  .card {
    flex: 1;
    padding: 9px 12px;
    margin-bottom: 10px;
    border-radius: 6px;
    border: 1px solid transparent;
  }
  .step-node.active .card {
    background: var(--vscode-editorWidget-background);
    border-color: rgba(var(--step-rgb), 0.65);
    box-shadow: 0 2px 10px rgba(0, 0, 0, 0.25);
  }
  .step-title {
    font-size: 13px;
    font-weight: 600;
  }
  .meta {
    font-size: 11px;
    color: var(--vscode-descriptionForeground);
    margin-top: 3px;
    display: flex;
    gap: 6px;
    flex-wrap: wrap;
    align-items: center;
  }
  .badge {
    background: rgba(var(--step-rgb), 0.14);
    color: var(--step-hex);
    border: 1px solid rgba(var(--step-rgb), 0.35);
    padding: 1px 6px;
    border-radius: 4px;
    font-family: var(--vscode-editor-font-family);
    font-size: 10px;
  }
  .narration {
    margin-top: 10px;
    padding-top: 10px;
    border-top: 1px solid var(--vscode-editorWidget-border);
    font-size: 13px;
    line-height: 1.5;
    white-space: pre-wrap;
  }

  /* Right Code Canvas */
  .code-pane {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-width: 0;
    background: var(--vscode-editor-background);
    position: relative;
  }
  .code-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 10px 20px;
    border-bottom: 1px solid var(--vscode-editorWidget-border);
    font-size: 12px;
    color: var(--vscode-descriptionForeground);
    background: var(--vscode-editorGroupHeader-tabsBackground, var(--vscode-editor-background));
  }
  .code-header .file-label {
    font-family: var(--vscode-editor-font-family);
    color: var(--vscode-editor-foreground);
  }
  .code-scroll {
    flex: 1;
    overflow: auto;
    padding: 20px 24px 20px 48px;
    font-family: var(--vscode-editor-font-family);
    font-size: var(--vscode-editor-font-size, 13px);
    line-height: 1.55;
  }
  .code-lines {
    width: 100%;
  }
  .code-line {
    display: flex;
    align-items: baseline;
    padding: 0 10px 0 6px;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }
  .code-line.dimmed {
    opacity: 0.32;
  }
  .ln {
    width: 42px;
    flex-shrink: 0;
    text-align: right;
    padding-right: 16px;
    color: var(--vscode-editorLineNumber-foreground);
    user-select: none;
  }
  .src {
    flex: 1;
    min-width: 0;
  }
  .spotlight-group {
    background: rgba(var(--accent-rgb), 0.11);
    border-left: 3px solid var(--accent-hex);
    border-top: 1px solid rgba(var(--accent-rgb), 0.28);
    border-bottom: 1px solid rgba(var(--accent-rgb), 0.28);
    border-right: 1px solid rgba(var(--accent-rgb), 0.2);
    border-radius: 0 6px 6px 0;
    margin: 2px 0;
    padding: 4px 0;
    box-shadow: 0 4px 20px rgba(0, 0, 0, 0.25);
  }
  .spotlight-group .ln {
    color: var(--accent-hex);
  }
  .error-box {
    padding: 16px;
    color: var(--vscode-errorForeground);
  }

  /* Syntax Highlighting Tokens (Dark+ inspired, theme friendly) */
  .hljs-keyword, .hljs-selector-tag, .hljs-literal, .hljs-type { color: #569cd6; }
  .hljs-string, .hljs-attr { color: #ce9178; }
  .hljs-number { color: #b5cea8; }
  .hljs-comment { color: #6a9955; font-style: italic; }
  .hljs-function, .hljs-title, .hljs-title\\.function_ { color: #dcdcaa; }
  .hljs-built_in { color: #4ec9b0; }
  .hljs-variable, .hljs-params { color: #9cdcfe; }

  /* Full-canvas SVG Bezier Connector */
  #connector-svg {
    position: fixed;
    inset: 0;
    width: 100%;
    height: 100%;
    pointer-events: none;
    z-index: 20;
  }
  #connector-path {
    fill: none;
    stroke: var(--accent-hex);
    stroke-width: 2;
    filter: drop-shadow(0 0 5px rgba(var(--accent-rgb), 0.55));
  }
</style>
</head>
<body>
  <div class="layout">
    <div class="sidebar" id="sidebar">
      <div class="story-title">${escapeHtml(state.story_title)} — ${state.current_step_index + 1} / ${state.total_steps}</div>
      <div class="progress-track"><div class="progress-fill"></div></div>
      <div class="controls">
        <button class="secondary" onclick="send('prev')">◀ Prev</button>
        <button onclick="send('${playAction}')">${playLabel}</button>
        <button class="secondary" onclick="send('next')">Next ▶</button>
        <button class="secondary" onclick="send('stop')">Stop</button>
      </div>
      <div class="timeline">
        ${timelineHtml}
      </div>
    </div>

    <div class="code-pane">
      <div class="code-header">
        <span class="file-label">${escapeHtml(step.file_path)} <span style="opacity:0.6">(${step.start_line}-${step.end_line})</span></span>
        <button class="secondary" onclick="send('open_editor')">↗ Open in Editor</button>
      </div>
      <div class="code-scroll" id="code-scroll">
        ${codeHtml}
      </div>
    </div>
  </div>

  <svg id="connector-svg">
    <circle id="connector-start" r="4" fill="${activePalette.hex}" style="display:none;" />
    <path id="connector-path" style="display:none;" />
    <circle id="connector-end" r="4" fill="${activePalette.hex}" style="display:none;" />
  </svg>

  <script>
    const vscode = acquireVsCodeApi();
    function send(action) {
      vscode.postMessage({ action });
    }
    function gotoStep(index) {
      vscode.postMessage({ action: 'goto', step_index: index });
    }

    function updateCurve() {
      const card = document.getElementById('active-step-card');
      const target = document.getElementById('spotlight-target');
      const codeScroll = document.getElementById('code-scroll');
      const pathEl = document.getElementById('connector-path');
      const startDot = document.getElementById('connector-start');
      const endDot = document.getElementById('connector-end');
      if (!card || !target || !codeScroll || !pathEl || !startDot || !endDot) {
        return;
      }

      const cRect = card.getBoundingClientRect();
      const tRect = target.getBoundingClientRect();
      const sRect = codeScroll.getBoundingClientRect();

      const x1 = cRect.right;
      const y1 = cRect.top + Math.min(cRect.height * 0.5, 42);

      const x2 = Math.max(tRect.left + 1, sRect.left + 8);
      const rawY2 = tRect.top + Math.min(tRect.height * 0.5, 90);
      const y2 = Math.max(sRect.top + 8, Math.min(sRect.bottom - 8, rawY2));

      const dx = Math.max(16, (x2 - x1) * 0.55);
      const d = 'M ' + x1 + ' ' + y1 +
                ' C ' + (x1 + dx) + ' ' + y1 + ', ' +
                (x2 - dx) + ' ' + y2 + ', ' +
                x2 + ' ' + y2;

      startDot.setAttribute('cx', String(x1));
      startDot.setAttribute('cy', String(y1));
      startDot.style.display = 'block';

      endDot.setAttribute('cx', String(x2));
      endDot.setAttribute('cy', String(y2));
      endDot.style.display = 'block';

      pathEl.setAttribute('d', d);
      pathEl.style.display = 'block';
    }

    window.addEventListener('load', () => {
      const target = document.getElementById('spotlight-target');
      if (target) {
        target.scrollIntoView({ block: 'center', behavior: 'instant' });
      }
      requestAnimationFrame(updateCurve);
    });

    const codeScroll = document.getElementById('code-scroll');
    const sidebar = document.getElementById('sidebar');
    if (codeScroll) {
      codeScroll.addEventListener('scroll', () => requestAnimationFrame(updateCurve));
    }
    if (sidebar) {
      sidebar.addEventListener('scroll', () => requestAnimationFrame(updateCurve));
    }
    window.addEventListener('resize', () => requestAnimationFrame(updateCurve));
  </script>
</body>
</html>`;
}

export function renderCodeLine(lineNumber: number, html: string, dimmed: boolean): string {
	const cls = dimmed ? 'code-line dimmed' : 'code-line';
	const content = html.length > 0 ? html : ' ';
	return `<div class="${cls}"><span class="ln">${lineNumber}</span><span class="src">${content}</span></div>`;
}

export function highlightCodeLines(source: string, filePath: string): string[] {
	const ext = path.extname(filePath).toLowerCase().replace('.', '');
	const langMap: Record<string, string> = {
		go: 'go',
		ts: 'typescript',
		tsx: 'typescript',
		js: 'javascript',
		jsx: 'javascript',
		py: 'python',
		rs: 'rust',
		json: 'json',
		yaml: 'yaml',
		yml: 'yaml',
		md: 'markdown',
		sh: 'bash',
	};

	const lang = langMap[ext];
	let highlighted = '';
	try {
		highlighted = lang && hljs.getLanguage(lang)
			? hljs.highlight(source, { language: lang }).value
			: hljs.highlightAuto(source).value;
	} catch {
		highlighted = escapeHtml(source);
	}

	return balanceSpanTagsPerLine(highlighted);
}

// Ensures multi-line <span> tokens from highlight.js are closed and reopened cleanly across line breaks.
export function balanceSpanTagsPerLine(html: string): string[] {
	const rawLines = html.split('\n');
	const openTags: string[] = [];

	return rawLines.map((line) => {
		const prefix = openTags.join('');
		const tagRegex = /<\/?span(?:\s+[^>]*)?>/g;
		let match: RegExpExecArray | null;
		while ((match = tagRegex.exec(line)) !== null) {
			if (match[0].startsWith('</')) {
				openTags.pop();
			} else {
				openTags.push(match[0]);
			}
		}
		const suffix = '</span>'.repeat(openTags.length);
		return prefix + line + suffix;
	});
}

export function escapeHtml(value: string): string {
	return value
		.replace(/&/g, '&amp;')
		.replace(/</g, '&lt;')
		.replace(/>/g, '&gt;')
		.replace(/"/g, '&quot;')
		.replace(/'/g, '&#039;');
}
