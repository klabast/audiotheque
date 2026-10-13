import { untrack } from 'svelte';
import { SvelteMap, SvelteSet } from 'svelte/reactivity';
import { api } from '#lib/services/api';
import type { LibraryTrackDetailResponse } from '#lib/api/client';

/**
 * Track details by id, for anything that shows a track outside its album page
 * (player, queue). Components call `load` with the ids they display and read
 * them back reactively with `get`.
 */
export function createTrackDetails() {
	const known = new SvelteMap<number, LibraryTrackDetailResponse>();
	const inFlight = new SvelteSet<number>();

	return {
		get(id: number): LibraryTrackDetailResponse | null {
			return known.get(id) ?? null;
		},

		async load(ids: number[]) {
			// Callers run this from effects; deciding what to fetch must not make
			// those effects depend on the cache they are filling.
			const missing = untrack(() =>
				ids.filter((id, i) => ids.indexOf(id) === i && !known.has(id) && !inFlight.has(id))
			);
			if (missing.length === 0) return;

			missing.forEach((id) => inFlight.add(id));
			try {
				const tracks = await api.lookupTracks(missing);
				for (const track of tracks) {
					if (track.id !== undefined) known.set(track.id, track);
				}
			} catch (err) {
				console.error('[trackDetails] lookup failed', err);
			} finally {
				missing.forEach((id) => inFlight.delete(id));
			}
		}
	};
}

export const trackDetails = createTrackDetails();
