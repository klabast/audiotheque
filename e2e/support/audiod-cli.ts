import { execSync } from 'child_process';
import * as fs from 'fs';
import * as path from 'path';
import { SERVER_DIR, PROJECT_ROOT } from './paths';

const CI_MODE = process.env.CI_MODE === 'true';
const DOCKER = process.env.DOCKER || 'docker';
const COMPOSE_FILE = process.env.COMPOSE_FILE || 'docker-compose.test.yml';
const COMPOSE_SERVICE = process.env.COMPOSE_SERVICE || 'audiod';

/**
 * Run a audiod CLI subcommand. In dev mode (CI_MODE != true) this
 * shells out to ./audiod in SERVER_DIR. In CI mode it execs inside
 * the running container so the test reaches the same data dir as the
 * server under test.
 */
export function runAudiodCli(args: string, opts: { stdio?: 'inherit' | 'pipe' } = {}): string {
	const stdio = opts.stdio ?? 'pipe';
	if (CI_MODE) {
		return execSync(
			`${DOCKER} compose -f ${COMPOSE_FILE} exec -T ${COMPOSE_SERVICE} /app/audiod ${args}`,
			{ cwd: PROJECT_ROOT, encoding: 'utf-8', stdio: ['ignore', stdio === 'inherit' ? 'inherit' : 'pipe', stdio === 'inherit' ? 'inherit' : 'pipe'] }
		).toString();
	}
	return execSync(`./audiod ${args}`, {
		cwd: SERVER_DIR,
		encoding: 'utf-8',
		stdio: ['ignore', stdio === 'inherit' ? 'inherit' : 'pipe', stdio === 'inherit' ? 'inherit' : 'pipe']
	}).toString();
}

/**
 * Translate a test fixture path (e.g. "e2e/data/music" or
 * "e2e/data/music/02-hi-res") to the path the server under test sees.
 *
 * Dev mode: absolute host path (PROJECT_ROOT/e2e/data/music/...).
 * CI mode:  the container mount point (/app/music/...) baked into
 *           docker-compose.test.yml's bind mount of e2e/data/music.
 */
export function resolveServerPath(testPath: string): string {
	if (CI_MODE) {
		if (testPath === 'e2e/data/music') return '/app/music';
		if (testPath.startsWith('e2e/data/music/')) {
			return `/app/music/${testPath.slice('e2e/data/music/'.length)}`;
		}
	}
	if (testPath.startsWith('e2e/')) {
		return `${PROJECT_ROOT}/${testPath}`;
	}
	return testPath;
}

/**
 * List a directory inside the server's data dir. Dev mode: host fs.
 * CI mode: shells into the container.
 */
export function listServerDataDir(relDir: string): string[] {
	if (CI_MODE) {
		const out = execSync(
			`${DOCKER} compose -f ${COMPOSE_FILE} exec -T ${COMPOSE_SERVICE} sh -c "ls -1 /app/data/${relDir} 2>/dev/null || true"`,
			{ cwd: PROJECT_ROOT, encoding: 'utf-8' }
		).toString();
		return out.split('\n').filter((l) => l.trim() !== '');
	}
	const full = path.join(SERVER_DIR, 'data', relDir);
	if (!fs.existsSync(full)) return [];
	return fs.readdirSync(full);
}

/**
 * Read a file inside the server's data dir. Dev mode: host fs. CI mode:
 * `<docker> compose exec` cats the file in the container.
 */
export function readServerDataFile(relPath: string): string {
	if (CI_MODE) {
		return execSync(
			`${DOCKER} compose -f ${COMPOSE_FILE} exec -T ${COMPOSE_SERVICE} cat /app/data/${relPath}`,
			{ cwd: PROJECT_ROOT, encoding: 'utf-8' }
		).toString();
	}
	return fs.readFileSync(path.join(SERVER_DIR, 'data', relPath), 'utf-8');
}
