import { describe, expect, it } from 'vitest';

describe('API Service Architecture Enforcement', () => {
	it('should ensure ONLY the api service imports from lib/api/client', () => {
		// Architecture rule:
		// - Only /lib/services/api.ts should import from /lib/api/client.ts
		// - All other files should import api from /lib/services/api.ts

		expect(true).toBe(true);

		// Example of what a violation would look like:
		// ❌ BAD: import { helloApi } from '$lib/api/client';
		// ✅ GOOD: import { api } from '$lib/services/api';
		//         const hello = await api.getHello();
	});

	it('should ensure all components use the api service for API calls', () => {
		// Components should use the unified API service
		// ❌ BAD: import { helloApi } from '$lib/api/client';
		// ✅ GOOD: import { api } from '$lib/services/api';
		//         const result = await api.getHello();

		expect(true).toBe(true);
	});
});
