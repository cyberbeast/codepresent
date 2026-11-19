import * as fsSync from 'fs';
import * as fs from 'fs/promises';
import * as os from 'os';
import * as path from 'path';

export interface SessionEntry {
	pid: number;
	ws_url: string;
	workspace_root?: string;
	story_title?: string;
	started_at?: string;
}

export function defaultRegistryDir(): string {
	return path.join(os.homedir(), '.codepresent', 'sessions');
}

export function isProcessAlive(pid: number): boolean {
	if (!Number.isInteger(pid) || pid <= 0) {
		return false;
	}
	try {
		process.kill(pid, 0);
		return true;
	} catch (err: unknown) {
		const code = (err as NodeJS.ErrnoException | undefined)?.code;
		return code === 'EPERM';
	}
}

export async function listActiveSessions(registryDir: string = defaultRegistryDir()): Promise<SessionEntry[]> {
	let entries: fsSync.Dirent[];
	try {
		entries = await fs.readdir(registryDir, { withFileTypes: true });
	} catch (err: unknown) {
		const code = (err as NodeJS.ErrnoException | undefined)?.code;
		if (code === 'ENOENT') {
			return [];
		}
		throw err;
	}

	const sessions: SessionEntry[] = [];
	for (const entry of entries) {
		if (!entry.isFile() || !entry.name.endsWith('.json') || entry.name.startsWith('.')) {
			continue;
		}
		const fullPath = path.join(registryDir, entry.name);
		try {
			const raw = await fs.readFile(fullPath, 'utf8');
			const parsed = JSON.parse(raw) as SessionEntry;
			if (!parsed.pid || !parsed.ws_url || !isProcessAlive(parsed.pid)) {
				await fs.unlink(fullPath).catch(() => undefined);
				continue;
			}
			sessions.push(parsed);
		} catch {
			// Ignore malformed or concurrently removed session files.
		}
	}

	sessions.sort((a, b) => {
		const ta = a.started_at ? Date.parse(a.started_at) : 0;
		const tb = b.started_at ? Date.parse(b.started_at) : 0;
		return tb - ta;
	});
	return sessions;
}

export function selectSessionForWorkspace(
	sessions: SessionEntry[],
	workspaceFolders: string[]
): SessionEntry | undefined {
	if (sessions.length === 0) {
		return undefined;
	}

	const cleanFolders = workspaceFolders.map((f) => path.resolve(f));
	if (cleanFolders.length > 0) {
		for (const session of sessions) {
			if (!session.workspace_root) {
				continue;
			}
			const root = path.resolve(session.workspace_root);
			if (cleanFolders.some((folder) => folder === root || isSubpath(folder, root) || isSubpath(root, folder))) {
				return session;
			}
		}
		const unrooted = sessions.find((s) => !s.workspace_root);
		if (unrooted && sessions.length === 1) {
			return unrooted;
		}
		return undefined;
	}

	return sessions[0];
}

function isSubpath(child: string, parent: string): boolean {
	const rel = path.relative(parent, child);
	return rel !== '' && rel !== '.' && !rel.startsWith('..') && !path.isAbsolute(rel);
}
