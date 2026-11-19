import * as assert from 'node:assert/strict';
import * as fs from 'node:fs/promises';
import * as os from 'node:os';
import * as path from 'node:path';
import { test } from 'node:test';
import { listActiveSessions, selectSessionForWorkspace, SessionEntry } from './sessions';

test('listActiveSessions reads live PIDs and prunes stale PIDs', async () => {
	const dir = await fs.mkdtemp(path.join(os.tmpdir(), 'codepresent-reg-'));
	try {
		await fs.writeFile(
			path.join(dir, `${process.pid}.json`),
			JSON.stringify({
				pid: process.pid,
				ws_url: 'ws://127.0.0.1:51001/ws',
				workspace_root: '/tmp/repo-a',
				started_at: '2026-11-18T23:00:00Z',
			})
		);
		await fs.writeFile(
			path.join(dir, '999999.json'),
			JSON.stringify({
				pid: 999999,
				ws_url: 'ws://127.0.0.1:51002/ws',
				started_at: '2026-11-18T23:01:00Z',
			})
		);

		const active = await listActiveSessions(dir);
		assert.equal(active.length, 1);
		assert.equal(active[0].pid, process.pid);

		const remaining = await fs.readdir(dir);
		assert.deepEqual(remaining, [`${process.pid}.json`]);
	} finally {
		await fs.rm(dir, { recursive: true, force: true });
	}
});

test('selectSessionForWorkspace matches workspace folders and avoids cross-repo hijacking', () => {
	const sessions: SessionEntry[] = [
		{ pid: 2, ws_url: 'ws://127.0.0.1:52002/ws', workspace_root: '/Users/dev/repo-b' },
		{ pid: 1, ws_url: 'ws://127.0.0.1:52001/ws', workspace_root: '/Users/dev/repo-a' },
	];

	const matched = selectSessionForWorkspace(sessions, ['/Users/dev/repo-a']);
	assert.equal(matched?.pid, 1);

	const unrelated = selectSessionForWorkspace(sessions, ['/Users/dev/repo-c']);
	assert.equal(unrelated, undefined);

	const noWorkspace = selectSessionForWorkspace(sessions, []);
	assert.equal(noWorkspace?.pid, 2);
});
