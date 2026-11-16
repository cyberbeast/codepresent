import test from 'node:test';
import assert from 'node:assert/strict';
import * as path from 'node:path';
import {
	STEP_PALETTE,
	balanceSpanTagsPerLine,
	escapeHtml,
	highlightCodeLines,
	PresentationState,
	PresentationStep,
	renderCanvasHtml,
	resolveStepFilePath,
	stepColor,
} from './renderer';

test('stepColor cycles cleanly through palette', () => {
	assert.deepEqual(stepColor(0), STEP_PALETTE[0]);
	assert.deepEqual(stepColor(1), STEP_PALETTE[1]);
	assert.deepEqual(stepColor(STEP_PALETTE.length), STEP_PALETTE[0]);
	assert.deepEqual(stepColor(-1), STEP_PALETTE[STEP_PALETTE.length - 1]);
});

test('escapeHtml escapes special HTML characters', () => {
	const raw = `<script>alert("x" & 'y')</script>`;
	assert.equal(
		escapeHtml(raw),
		'&lt;script&gt;alert(&quot;x&quot; &amp; &#039;y&#039;)&lt;/script&gt;'
	);
});

test('balanceSpanTagsPerLine keeps multi-line spans balanced on each line', () => {
	const input = '<span class="hljs-comment">/* line 1\nline 2\nline 3 */</span>\nplain';
	const lines = balanceSpanTagsPerLine(input);
	assert.equal(lines.length, 4);
	assert.equal(lines[0], '<span class="hljs-comment">/* line 1</span>');
	assert.equal(lines[1], '<span class="hljs-comment">line 2</span>');
	assert.equal(lines[2], '<span class="hljs-comment">line 3 */</span>');
	assert.equal(lines[3], 'plain');
});

test('highlightCodeLines splits Go source into exact line count', () => {
	const goSrc = 'package main\n\nfunc main() {\n\tprintln("hi")\n}';
	const lines = highlightCodeLines(goSrc, 'cmd/codepresent/main.go');
	assert.equal(lines.length, 5);
	assert.match(lines[0], /hljs-keyword/);
});

test('resolveStepFilePath resolves absolute, workspace folder, and workspaceRoot fallback', () => {
	const absPath = path.resolve('/tmp/foo.go');
	assert.equal(resolveStepFilePath(absPath, ['/ws'], '/root'), absPath);
	assert.equal(
		resolveStepFilePath('pkg/main.go', ['/ws'], '/root'),
		path.join('/ws', 'pkg/main.go')
	);
	assert.equal(
		resolveStepFilePath('pkg/main.go', [], '/root'),
		path.join('/root', 'pkg/main.go')
	);
	assert.equal(resolveStepFilePath('pkg/main.go', [], undefined), undefined);
});

test('renderCanvasHtml renders storyboard, spotlight, word-wrap, and SVG connector', () => {
	const step1: PresentationStep = {
		step_number: 1,
		file_path: 'main.go',
		start_line: 2,
		end_line: 3,
		focus_target: 'main()',
		title: 'Entrypoint',
		narration: 'Explains <main> & startup',
	};
	const step2: PresentationStep = {
		step_number: 2,
		file_path: 'main.go',
		start_line: 4,
		end_line: 4,
		title: 'Exit',
		narration: 'Done',
	};
	const state: PresentationState = {
		story_title: 'Demo Tour',
		current_step_index: 0,
		total_steps: 2,
		is_playing: false,
		current_step: step1,
		steps: [step1, step2],
	};

	const html = renderCanvasHtml(state, step1, 'package main\nfunc main() {\n\tprintln("hi")\n}', '');
	assert.match(html, /Demo Tour — 1 \/ 2/);
	assert.match(html, /id="active-step-card"/);
	assert.match(html, /id="spotlight-target"/);
	assert.match(html, /id="connector-path"/);
	assert.match(html, /id="connector-end"/);
	assert.match(html, /white-space:\s*pre-wrap/);
	assert.match(html, /Explains &lt;main&gt; &amp; startup/);
	assert.match(html, /gotoStep\(1\)/);
	assert.match(html, /--accent-hex:\s*#ffd54f/);
});
