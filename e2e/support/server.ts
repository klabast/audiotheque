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
 * Parse reset code from server logs
 * Expected format: "RESET CODE: ABC123" or similar
 */
export function parseResetCode(logs: string): string {
  // Match various formats:
  // - "RESET CODE: ABC123"
  // - "Reset code: ABC123"
  // - "[RESET] Code: ABC123"
  const match = logs.match(/RESET[\s\w]*CODE[:\s]+([A-Z0-9]+)/i);

  if (!match || !match[1]) {
    throw new Error('Reset code not found in server logs. Logs: ' + logs.slice(-500));
  }

  return match[1];
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
