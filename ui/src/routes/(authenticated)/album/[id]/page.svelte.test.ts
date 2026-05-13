import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/svelte';
import '@testing-library/jest-dom/vitest';
import { readable } from 'svelte/store';
import Page from './+page.svelte';

vi.mock('$app/stores', () => ({
	page: readable({ params: { id: '42' } })
}));

vi.mock('$lib/services/api', () => ({
	api: {
		getAlbum: vi.fn(),
		listAlbumTracks: vi.fn(),
		playAlbum: vi.fn(),
		getPlaybackSession: vi.fn().mockResolvedValue(null),
		subscribeToPlaybackSession: vi.fn(() => () => {}),
		subscribeToClientId: vi.fn(() => () => {}),
		listDevices: vi.fn().mockResolvedValue([]),
		getTrackStreamUrl: vi.fn(() => '')
	}
}));

const album = {
	id: 42,
	title: 'Solace',
	artistName: 'RÜFÜS DU SOL',
	totalTracks: 9
};

const tracks = [
	{ id: 101, title: 'Treat You Better', trackNumber: 1, duration: 273000 },
	{ id: 102, title: 'Eyes', trackNumber: 2, duration: 230000 }
];

describe('Album page — Play Album button', () => {
	beforeEach(() => vi.clearAllMocks());

	it('plays the album when the Play Album button is clicked', async () => {
		const { api } = await import('$lib/services/api');
		vi.mocked(api.getAlbum).mockResolvedValue(album as never);
		vi.mocked(api.listAlbumTracks).mockResolvedValue(tracks as never);
		vi.mocked(api.playAlbum).mockResolvedValue({} as never);

		render(Page);

		const button = await screen.findByTestId('play-album-button');
		await fireEvent.click(button);

		// Play button routes through the playback store, which calls
		// api.playAlbum(albumId, startTrackId, targetDeviceId). For a plain
		// album-play, both optional args are undefined.
		expect(api.playAlbum).toHaveBeenCalledWith(42, undefined, undefined);
	});

	it('plays the album from a specific track when its row is clicked', async () => {
		const { api } = await import('$lib/services/api');
		vi.mocked(api.getAlbum).mockResolvedValue(album as never);
		vi.mocked(api.listAlbumTracks).mockResolvedValue(tracks as never);
		vi.mocked(api.playAlbum).mockResolvedValue({} as never);

		render(Page);

		// The row's data-testid is on the clickable button itself.
		const row = await screen.findByTestId('track-row-102');
		await fireEvent.click(row);

		expect(api.playAlbum).toHaveBeenCalledWith(42, 102, undefined);
	});
});
