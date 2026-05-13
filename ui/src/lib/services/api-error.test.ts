import { describe, expect, it } from 'vitest';
import { throwIfNotOk } from './api-error';

function makeResponse(status: number, body: string): Response {
	return new Response(body, { status });
}

describe('throwIfNotOk', () => {
	it('does not throw on ok response', async () => {
		await expect(throwIfNotOk(makeResponse(200, 'fine'), 'Failed to X')).resolves.toBeUndefined();
	});

	it('throws Error including default message, status, and server body', async () => {
		const response = makeResponse(403, 'forbidden');
		await expect(throwIfNotOk(response, 'Failed to X')).rejects.toThrow(/Failed to X/);
		await expect(throwIfNotOk(makeResponse(403, 'forbidden'), 'Failed to X')).rejects.toThrow(
			/403/
		);
		await expect(throwIfNotOk(makeResponse(403, 'forbidden'), 'Failed to X')).rejects.toThrow(
			/forbidden/
		);
	});

	it('throws Error with default message and status when body is empty', async () => {
		await expect(throwIfNotOk(makeResponse(500, ''), 'Failed to Y')).rejects.toThrow(/Failed to Y/);
		await expect(throwIfNotOk(makeResponse(500, ''), 'Failed to Y')).rejects.toThrow(/500/);
	});
});
