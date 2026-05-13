/**
 * API Service Architecture Test
 *
 * Ensures the API service layer is used correctly throughout the application.
 *
 * Rules enforced:
 * 1. NO direct fetch() calls to /api/* endpoints
 * 2. All components should use services/api.ts for API calls
 * 3. API service should provide complete coverage of all endpoints
 * 4. NO direct imports of generated API client or WebSocket client (only api.ts can import these)
 */

import { describe, it, expect } from 'vitest';
import * as fs from 'fs';
import * as path from 'path';
import { fileURLToPath } from 'url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

describe('API Service Architecture', () => {
	const srcDir = path.resolve(__dirname, '../../..');

	it('should not have any direct fetch() calls to /api/* endpoints', () => {
		const violations: Array<{
			file: string;
			line: number;
			code: string;
		}> = [];

		// Patterns that indicate direct API calls
		const forbiddenPatterns = [
			/fetch\s*\(\s*['"`]\/api\//, // fetch('/api/...
			/fetch\s*\(\s*`[^`]*\/api\// // fetch(`${var}/api/...
		];

		function scanDirectory(dir: string) {
			const files = fs.readdirSync(dir);

			for (const file of files) {
				const fullPath = path.join(dir, file);
				const relativePath = path.relative(srcDir, fullPath);

				// Skip non-source directories
				if (
					relativePath.includes('node_modules') ||
					relativePath.includes('.svelte-kit') ||
					relativePath.includes('dist') ||
					relativePath.includes('build') ||
					relativePath.includes('.git')
				) {
					continue;
				}

				const stat = fs.statSync(fullPath);

				if (stat.isDirectory()) {
					scanDirectory(fullPath);
				} else if (file.endsWith('.ts') || file.endsWith('.js') || file.endsWith('.svelte')) {
					// Skip test files
					if (file.endsWith('.test.ts') || file.endsWith('.spec.ts')) {
						continue;
					}

					// Skip generated files
					if (relativePath.includes('/generated/')) {
						continue;
					}

					// Skip services/api.ts - it's the service layer, it SHOULD have fetch() calls
					if (relativePath.endsWith('services/api.ts')) {
						continue;
					}

					checkFile(fullPath, relativePath);
				}
			}
		}

		function checkFile(filePath: string, relativePath: string) {
			const content = fs.readFileSync(filePath, 'utf-8');
			const lines = content.split('\n');

			lines.forEach((line, index) => {
				// Skip comments
				const trimmed = line.trim();
				if (trimmed.startsWith('//') || trimmed.startsWith('*') || trimmed.startsWith('/*')) {
					return;
				}

				forbiddenPatterns.forEach((pattern) => {
					if (pattern.test(line)) {
						violations.push({
							file: relativePath,
							line: index + 1,
							code: line.trim()
						});
					}
				});
			});
		}

		scanDirectory(srcDir);

		if (violations.length > 0) {
			console.error('\n❌ Direct fetch() calls found:\n');
			violations.forEach((v) => {
				console.error(`  ${v.file}:${v.line}`);
				console.error(`    ${v.code}\n`);
			});
			console.error('💡 Fix: Use the API service instead of fetch()');
			console.error('    import { api } from "$lib/services/api";');
			console.error('    Then use: api.methodName()');
			console.error('\n📊 Total violations:', violations.length);
		}

		expect(violations).toHaveLength(0);
	});

	it('should import api service correctly in all components using API', () => {
		const issues: Array<{
			file: string;
			issue: string;
		}> = [];

		const correctImports: string[] = [];

		// Valid ways to import the api service
		const validPatterns = [
			/import\s+{\s*api\s*}\s+from\s+['"].*\/services\/api['"]/,
			/import\s+{\s*api[^}]*}\s+from\s+['"].*\/services\/api['"]/
		];

		function scanDirectory(dir: string) {
			const files = fs.readdirSync(dir);

			for (const file of files) {
				const fullPath = path.join(dir, file);
				const relativePath = path.relative(srcDir, fullPath);

				// Skip non-source directories
				if (
					relativePath.includes('node_modules') ||
					relativePath.includes('.svelte-kit') ||
					relativePath.includes('dist') ||
					relativePath.includes('build') ||
					relativePath.includes('.git')
				) {
					continue;
				}

				const stat = fs.statSync(fullPath);

				if (stat.isDirectory()) {
					scanDirectory(fullPath);
				} else if (file.endsWith('.ts') || file.endsWith('.svelte')) {
					// Skip test files and the api service itself
					if (
						file.endsWith('.test.ts') ||
						file.endsWith('.spec.ts') ||
						relativePath.endsWith('services/api.ts')
					) {
						continue;
					}

					// Skip generated files
					if (relativePath.includes('/generated/')) {
						continue;
					}

					checkFile(fullPath, relativePath);
				}
			}
		}

		function checkFile(filePath: string, relativePath: string) {
			const content = fs.readFileSync(filePath, 'utf-8');

			// Check if file uses API (looks for api. usage, excluding URLs like "/api/ws")
			// Match: api.method() but NOT: '/api/' or '"/api/' or protocol like "wss://"
			const apiUsagePattern = /(?<!['"/:])\bapi\.\w+/;
			if (apiUsagePattern.test(content)) {
				// Check if it imports api service correctly
				const hasValidImport = validPatterns.some((pattern) => pattern.test(content));

				if (hasValidImport) {
					correctImports.push(relativePath);
				} else {
					// Check if it's importing api but incorrectly
					if (content.includes('import') && content.includes('api')) {
						issues.push({
							file: relativePath,
							issue: 'Uses api.* but does not import from services/api correctly'
						});
					}
				}
			}
		}

		scanDirectory(srcDir);

		if (issues.length > 0) {
			console.error('\n❌ Components with incorrect API imports:\n');
			issues.forEach((i) => {
				console.error(`  ${i.file}`);
				console.error(`    Issue: ${i.issue}\n`);
			});
		}

		if (correctImports.length > 0) {
			console.log(`\n✅ ${correctImports.length} files correctly import the API service`);
		}

		expect(issues).toHaveLength(0);
	});

	it('should NOT import generated API client or WebSocket client directly (only api.ts can)', () => {
		const violations: Array<{
			file: string;
			line: number;
			code: string;
			reason: string;
		}> = [];

		// Forbidden imports - only services/api.ts can import these
		// NOTE: TYPE imports are allowed and filtered separately
		const forbiddenImportPatterns = [
			{
				pattern: /from\s+['"].*\/api\/generated\//,
				reason: 'Direct import from generated API client'
			},
			{
				pattern: /from\s+['"].*\/services\/ws-client['"]/,
				reason: 'Direct import from WebSocket client'
			},
			{
				pattern: /from\s+['"]\$lib\/api\/generated\//,
				reason: 'Direct import from generated API client ($lib alias)'
			},
			{
				pattern: /from\s+['"]\$lib\/services\/ws-client['"]/,
				reason: 'Direct import from WebSocket client ($lib alias)'
			}
		];

		function scanDirectory(dir: string) {
			const files = fs.readdirSync(dir);

			for (const file of files) {
				const fullPath = path.join(dir, file);
				const relativePath = path.relative(srcDir, fullPath);

				// Skip non-source directories
				if (
					relativePath.includes('node_modules') ||
					relativePath.includes('.svelte-kit') ||
					relativePath.includes('dist') ||
					relativePath.includes('build') ||
					relativePath.includes('.git')
				) {
					continue;
				}

				const stat = fs.statSync(fullPath);

				if (stat.isDirectory()) {
					scanDirectory(fullPath);
				} else if (file.endsWith('.ts') || file.endsWith('.js') || file.endsWith('.svelte')) {
					// Skip test files
					if (file.endsWith('.test.ts') || file.endsWith('.spec.ts')) {
						continue;
					}

					// Skip generated files themselves
					if (relativePath.includes('/generated/')) {
						continue;
					}

					// ALLOW services/api.ts to import these (it's the service layer)
					if (relativePath.endsWith('services/api.ts')) {
						continue;
					}

					// ALLOW ws-client.ts itself (it's the WebSocket client implementation)
					if (relativePath.endsWith('services/ws-client.ts')) {
						continue;
					}

					checkFile(fullPath, relativePath);
				}
			}
		}

		function checkFile(filePath: string, relativePath: string) {
			const content = fs.readFileSync(filePath, 'utf-8');
			const lines = content.split('\n');

			lines.forEach((line, index) => {
				// Skip comments
				const trimmed = line.trim();
				if (trimmed.startsWith('//') || trimmed.startsWith('*') || trimmed.startsWith('/*')) {
					return;
				}

				// Skip TYPE imports - they're just TypeScript types, not runtime code
				if (trimmed.includes('import type')) {
					return;
				}

				forbiddenImportPatterns.forEach(({ pattern, reason }) => {
					if (pattern.test(line)) {
						violations.push({
							file: relativePath,
							line: index + 1,
							code: line.trim(),
							reason
						});
					}
				});
			});
		}

		scanDirectory(srcDir);

		if (violations.length > 0) {
			console.error('\n❌ Direct imports of generated API client or WebSocket client found:\n');
			violations.forEach((v) => {
				console.error(`  ${v.file}:${v.line}`);
				console.error(`    ${v.code}`);
				console.error(`    Reason: ${v.reason}\n`);
			});
			console.error('💡 Fix: Use the unified API service instead');
			console.error('    ❌ BAD:  import { LibraryApi } from "$lib/api/generated/..."');
			console.error('    ❌ BAD:  import { WebSocketClient } from "$lib/services/ws-client"');
			console.error('    ✅ GOOD: import { api } from "$lib/services/api";');
			console.error('             await api.createLibrary(...)');
			console.error('             api.subscribeToScanProgress(libraryId, callback)');
			console.error('\n📊 Total violations:', violations.length);
		}

		expect(violations).toHaveLength(0);
	});

	it('should have complete API coverage in services/api.ts', () => {
		// This test could check that all endpoints in swagger.json
		// have corresponding methods in services/api.ts
		// For now, we'll just check that the file exists and exports an api object

		const apiServicePath = path.resolve(__dirname, './api.ts');
		expect(fs.existsSync(apiServicePath)).toBe(true);

		const content = fs.readFileSync(apiServicePath, 'utf-8');

		// Check that it exports the api singleton
		const hasApiExport = content.includes('export const api');
		expect(hasApiExport).toBe(true);

		// Check for library management methods
		const expectedMethods = [
			'createLibrary',
			'listLibraries',
			// WebSocket subscription for scan progress
			'subscribeToScanProgress'
		];

		const missingMethods = expectedMethods.filter(
			(method) => !content.includes(`${method}(`) && !content.includes(`${method}:`)
		);

		if (missingMethods.length > 0) {
			console.warn('\n⚠️  Missing methods in API service:');
			missingMethods.forEach((m) => console.warn(`    - ${m}`));
		}

		expect(missingMethods).toHaveLength(0);
	});
});
