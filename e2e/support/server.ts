import { exec } from 'child_process';
import { promisify } from 'util';

const execAsync = promisify(exec);
const { testConfig } = require('../cucumber');

/**
 * Get server logs from Docker container
 * Container name differs between dev and CI mode
 */
export async function getServerLogs(): Promise<string> {
  const containerName = testConfig.isCIMode
    ? 'audiod-test'      // CI: Specific test container
    : 'audiotheque-audiod-1';   // Dev: Docker compose container (project=audiotheque, service=audiod)

  try {
    const { stdout, stderr } = await execAsync(`docker logs ${containerName}`);
    return stdout + stderr; // Combine stdout and stderr
  } catch (error) {
    throw new Error(`Failed to get server logs: ${error}`);
  }
}

/**
 * Wait for a condition to be true with timeout
 */
export async function waitFor(
  condition: () => Promise<boolean>,
  timeoutMs: number = 5000,
  pollIntervalMs: number = 100
): Promise<void> {
  const startTime = Date.now();

  while (Date.now() - startTime < timeoutMs) {
    if (await condition()) {
      return;
    }
    await new Promise(resolve => setTimeout(resolve, pollIntervalMs));
  }

  throw new Error(`Timeout waiting for condition after ${timeoutMs}ms`);
}
