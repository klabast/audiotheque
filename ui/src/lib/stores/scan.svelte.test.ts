import { beforeEach, describe, expect, it, vi } from 'vitest';

// The scan store is a module-level singleton. Reset modules between tests so
// listeners and state are fresh (same pattern as playback.svelte.test.ts).
beforeEach(() => {
	vi.resetModules();
	vi.clearAllMocks();
});

type WsCb = (data: unknown) => void;

function setupApiMock() {
	const scanProgressListeners: WsCb[] = [];
	const libraryUpdatedListeners: WsCb[] = [];

	vi.doMock('$lib/services/api', () => ({
		api: {
			subscribeToAllScanProgress: vi.fn((cb: WsCb) => {
				scanProgressListeners.push(cb);
				return () => {};
			}),
			subscribeToLibraryUpdated: vi.fn((cb: WsCb) => {
				libraryUpdatedListeners.push(cb);
				return () => {};
			}),
			getScanProgress: vi.fn().mockResolvedValue(null)
		}
	}));

	return { scanProgressListeners, libraryUpdatedListeners };
}

const runningProgress = (libraryId: number, processed = 5, total = 100) => ({
	libraryId,
	status: 'running' as const,
	totalFiles: total,
	processedFiles: processed,
	tracksAdded: processed,
	tracksUpdated: 0,
	errors: 0,
	currentFile: `/music/track-${processed}.flac`,
	startedAt: new Date().toISOString()
});

describe('scan store — receives scan-progress WS messages', () => {
	it('starts empty with no scans running', async () => {
		setupApiMock();
		const { scan } = await import('./scan.svelte');
		expect(scan.isAnyScanRunning).toBe(false);
		expect(scan.getProgress(1)).toBeUndefined();
	});

	it('stores progress for a library when a scan-progress message arrives', async () => {
		const { scanProgressListeners } = setupApiMock();
		const { scan } = await import('./scan.svelte');

		scanProgressListeners.forEach((cb) => cb(runningProgress(1, 5, 100)));

		expect(scan.getProgress(1)?.processedFiles).toBe(5);
		expect(scan.getProgress(1)?.totalFiles).toBe(100);
	});

	it('flips isAnyScanRunning to true while a scan is running', async () => {
		const { scanProgressListeners } = setupApiMock();
		const { scan } = await import('./scan.svelte');

		expect(scan.isAnyScanRunning).toBe(false);
		scanProgressListeners.forEach((cb) => cb(runningProgress(1)));
		expect(scan.isAnyScanRunning).toBe(true);
	});

	it('clears state when a scan completes', async () => {
		const { scanProgressListeners } = setupApiMock();
		const { scan } = await import('./scan.svelte');

		scanProgressListeners.forEach((cb) => cb(runningProgress(1, 10, 10)));
		expect(scan.isAnyScanRunning).toBe(true);

		// Backend signals completion.
		scanProgressListeners.forEach((cb) =>
			cb({ ...runningProgress(1, 10, 10), status: 'completed' as const })
		);
		expect(scan.isAnyScanRunning).toBe(false);
	});

	it('tracks scans on multiple libraries independently', async () => {
		const { scanProgressListeners } = setupApiMock();
		const { scan } = await import('./scan.svelte');

		scanProgressListeners.forEach((cb) => cb(runningProgress(1, 1, 10)));
		scanProgressListeners.forEach((cb) => cb(runningProgress(2, 50, 50)));

		expect(scan.isAnyScanRunning).toBe(true);
		// Library 2 finishes — library 1 still running.
		scanProgressListeners.forEach((cb) =>
			cb({ ...runningProgress(2, 50, 50), status: 'completed' as const })
		);
		expect(scan.isAnyScanRunning).toBe(true);
		expect(scan.getProgress(1)?.processedFiles).toBe(1);
	});
});

describe('scan store — notifies subscribers of library-updated events', () => {
	it('invokes onLibraryUpdated callbacks when a library-updated WS message arrives', async () => {
		const { libraryUpdatedListeners } = setupApiMock();
		const { scan } = await import('./scan.svelte');

		const calls: number[] = [];
		const unsub = scan.onLibraryUpdated((libraryId) => calls.push(libraryId));

		libraryUpdatedListeners.forEach((cb) => cb({ libraryId: 7 }));
		expect(calls).toEqual([7]);

		libraryUpdatedListeners.forEach((cb) => cb({ libraryId: 7 }));
		expect(calls).toEqual([7, 7]);

		unsub();
		libraryUpdatedListeners.forEach((cb) => cb({ libraryId: 7 }));
		expect(calls).toEqual([7, 7]); // No further calls after unsubscribe.
	});
});
