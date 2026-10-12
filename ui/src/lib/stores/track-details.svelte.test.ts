import { beforeEach, describe, expect, it, vi } from 'vitest';
import { api } from '$lib/services/api';
import { createTrackDetails } from './track-details.svelte';

vi.mock('$lib/services/api', () => ({
	api: { lookupTracks: vi.fn() }
}));

const lookup = vi.mocked(api.lookupTracks);

function track(id: number) {
	return { id, title: `Track ${id}`, albumId: 1, albumTitle: 'Album' };
}

beforeEach(() => {
	lookup.mockReset();
	lookup.mockImplementation(async (ids: number[]) => ids.map(track));
});

describe('trackDetails', () => {
	it('looks tracks up and serves them afterwards', async () => {
		const details = createTrackDetails();

		await details.load([1, 2]);

		expect(details.get(1)?.title).toBe('Track 1');
		expect(details.get(2)?.title).toBe('Track 2');
		expect(details.get(3)).toBeNull();
	});

	it('only asks for tracks it does not know yet, once each', async () => {
		const details = createTrackDetails();
		await details.load([1, 2]);

		await details.load([2, 3, 3, 1]);

		expect(lookup).toHaveBeenCalledTimes(2);
		expect(lookup).toHaveBeenLastCalledWith([3]);
	});

	it('does not ask twice while a lookup is in flight', async () => {
		const details = createTrackDetails();

		await Promise.all([details.load([1]), details.load([1])]);

		expect(lookup).toHaveBeenCalledTimes(1);
	});

	it('asks again after a failed lookup', async () => {
		const details = createTrackDetails();
		lookup.mockRejectedValueOnce(new Error('offline'));
		await details.load([1]);

		await details.load([1]);

		expect(lookup).toHaveBeenCalledTimes(2);
		expect(details.get(1)?.title).toBe('Track 1');
	});
});
