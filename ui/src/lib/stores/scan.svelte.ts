import { SvelteMap, SvelteSet } from 'svelte/reactivity';
import { api } from '$lib/services/api';
import type { ScanProgressData } from '$lib/services/ws-client';

export interface LibraryUpdatedData {
	libraryId: number;
}

type LibraryUpdatedHandler = (libraryId: number) => void;

/**
 * Global scan-progress store.
 *
 * Lives at module scope (not in any component) so that scan progress
 * persists across navigation. WebSocket subscriptions are installed lazily
 * via `scan.start()` — called once from `AppLayout.onMount`, AFTER children
 * (PlayFooter) have mounted and registered their own `client-id` listener.
 * Subscribing at module load would open the WS before PlayFooter could
 * attach its listener; the WS welcome would arrive first and the playback
 * store would never learn its own client ID (every e2e playback scenario
 * timed out at waitForClientId). See `start()` for the contract.
 *
 * `library-updated` events are forwarded to any handler registered via
 * `onLibraryUpdated` — the library page uses this to refetch albums while
 * a scan is running, without polling.
 */
function createScanStore() {
	const activeScans = new SvelteMap<number, ScanProgressData>();
	// SvelteSet (vs plain Set) keeps the Svelte lint rule happy and is fine
	// since we never read this set in reactive contexts — we just iterate it
	// to fan out events.
	const libraryUpdatedHandlers = new SvelteSet<LibraryUpdatedHandler>();

	// Auto-clear completed/failed entries after this many ms so the UI can
	// show a brief "Scan complete" before they vanish from the store.
	const COMPLETED_TTL_MS = 5000;

	let started = false;

	/**
	 * Wire up the two WebSocket subscriptions. Idempotent — safe to call
	 * from any number of AppLayouts or tests; only the first call has an
	 * effect.
	 *
	 * MUST be called only after components that need to register their own
	 * `client-id` WS listener (PlayFooter via playback.loadSession) have
	 * mounted, otherwise the welcome race in playback breaks. AppLayout's
	 * onMount is the canonical call site: children mount first, so by the
	 * time AppLayout.onMount runs, PlayFooter has already registered.
	 */
	function start() {
		if (started) return;
		started = true;

		api.subscribeToAllScanProgress((progress) => {
			activeScans.set(progress.libraryId, progress);

			if (progress.status === 'completed' || progress.status === 'failed') {
				const expected = progress;
				setTimeout(() => {
					const current = activeScans.get(expected.libraryId);
					// Only clear if we haven't seen a newer running scan since.
					if (current && current.status === expected.status) {
						activeScans.delete(expected.libraryId);
					}
				}, COMPLETED_TTL_MS);
			}
		});

		api.subscribeToLibraryUpdated((data) => {
			libraryUpdatedHandlers.forEach((cb) => {
				try {
					cb(data.libraryId);
				} catch (err) {
					console.error('[scan store] library-updated handler threw:', err);
				}
			});
		});
	}

	function getProgress(libraryId: number): ScanProgressData | undefined {
		return activeScans.get(libraryId);
	}

	function onLibraryUpdated(handler: LibraryUpdatedHandler): () => void {
		libraryUpdatedHandlers.add(handler);
		return () => libraryUpdatedHandlers.delete(handler);
	}

	return {
		get activeScans(): ReadonlyMap<number, ScanProgressData> {
			return activeScans;
		},
		get isAnyScanRunning(): boolean {
			for (const p of activeScans.values()) {
				if (p.status === 'running') return true;
			}
			return false;
		},
		getProgress,
		onLibraryUpdated,
		start
	};
}

export const scan = createScanStore();
