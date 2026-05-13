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
 * persists across navigation. Subscribes once to the WebSocket and routes
 * scan-progress messages into a per-library map.
 *
 * `library-updated` events are forwarded to any handler registered via
 * onLibraryUpdated — the library page uses this to refetch albums while a
 * scan is running, without polling.
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

	api.subscribeToAllScanProgress((progress) => {
		activeScans.set(progress.libraryId, progress);

		if (progress.status === 'completed' || progress.status === 'failed') {
			const expected = progress;
			setTimeout(() => {
				const current = activeScans.get(expected.libraryId);
				// Only clear if we haven't seen a newer running scan in the meantime.
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
		onLibraryUpdated
	};
}

export const scan = createScanStore();
