import * as path from 'path';
import * as fs from 'fs';

/**
 * Find project root by walking up the directory tree looking for .git
 */
function findProjectRoot(startDir: string): string {
  let currentDir = startDir;
  while (currentDir !== path.parse(currentDir).root) {
    if (fs.existsSync(path.join(currentDir, '.git'))) {
      return currentDir;
    }
    currentDir = path.dirname(currentDir);
  }
  throw new Error('Could not find project root (no .git directory found)');
}

export const PROJECT_ROOT = findProjectRoot(__dirname);
export const SERVER_DIR = path.join(PROJECT_ROOT, 'server');
export const E2E_DIR = path.join(PROJECT_ROOT, 'e2e');

/**
 * Resolve a test path to an absolute path.
 * If the path starts with 'e2e/', it's resolved relative to PROJECT_ROOT.
 */
export function resolveTestPath(testPath: string): string {
	if (testPath.startsWith('e2e/')) {
		return path.join(PROJECT_ROOT, testPath);
	}
	return testPath;
}
