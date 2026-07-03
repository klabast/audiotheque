import { beforeEach, describe, expect, it } from 'vitest';
import { createWebSocketClient, getOrCreateStableClientId } from './ws-client';

describe('getOrCreateStableClientId', () => {
	beforeEach(() => {
		sessionStorage.clear();
	});

	it('generates and persists a UUID on first call', () => {
		const id = getOrCreateStableClientId();
		expect(id).toMatch(/^[0-9a-f-]{36}$/i);
		expect(sessionStorage.getItem('audiod-device-id')).toBe(id);
	});

	it('returns the SAME id on subsequent calls (survives reconnects/reloads within the tab)', () => {
		const first = getOrCreateStableClientId();
		const second = getOrCreateStableClientId();
		expect(second).toBe(first);
	});

	it('uses sessionStorage, not localStorage — the identity must die with the tab', () => {
		const id = getOrCreateStableClientId();
		expect(localStorage.getItem('audiod-device-id')).toBeNull();
		expect(sessionStorage.getItem('audiod-device-id')).toBe(id);
	});
});

describe('createWebSocketClient', () => {
	beforeEach(() => {
		sessionStorage.clear();
	});

	it('includes the stable clientId as a query param on the WS URL', () => {
		const id = getOrCreateStableClientId();
		const client = createWebSocketClient();
		expect(client.connectUrl).toContain(`clientId=${id}`);
	});

	it('two clients created in the same tab share the same clientId', () => {
		const clientA = createWebSocketClient();
		const clientB = createWebSocketClient();
		const idA = new URL(clientA.connectUrl.replace('ws:', 'http:')).searchParams.get('clientId');
		const idB = new URL(clientB.connectUrl.replace('ws:', 'http:')).searchParams.get('clientId');
		expect(idA).toBe(idB);
		expect(idA).not.toBeNull();
	});
});
