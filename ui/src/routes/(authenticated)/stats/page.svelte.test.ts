import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen, fireEvent, within } from '@testing-library/svelte';
import '@testing-library/jest-dom/vitest';
import Page from './+page.svelte';
import { api } from '$lib/services/api';

vi.mock('$lib/services/api', () => ({
	api: { getStats: vi.fn() }
}));

const stats = {
	listens: 12,
	listeningSeconds: 3 * 3600 + 25 * 60,
	topTracks: [
		{ trackId: 7, title: 'Song', artistName: 'Band', albumId: 3, albumTitle: 'Record', plays: 5 }
	],
	topAlbums: [{ albumId: 3, title: 'Record', artistName: 'Band', plays: 9 }],
	topArtists: [{ artistId: 2, name: 'Band', plays: 12 }]
};

const empty = { listens: 0, listeningSeconds: 0, topTracks: [], topAlbums: [], topArtists: [] };

beforeEach(() => {
	vi.clearAllMocks();
	vi.mocked(api.getStats).mockResolvedValue(stats);
});

describe('stats page', () => {
	it('shows the last month by default', async () => {
		render(Page);

		expect(await screen.findByTestId('stats-listens')).toHaveTextContent('12');
		expect(api.getStats).toHaveBeenCalledWith('month');
		expect(screen.getByTestId('stats-period-month-button')).toHaveAttribute('aria-pressed', 'true');
	});

	it('shows time listened in hours and minutes', async () => {
		render(Page);

		expect(await screen.findByTestId('stats-listening-time')).toHaveTextContent(/3.*25/);
	});

	it('ranks tracks, albums and artists with their play counts', async () => {
		render(Page);

		const track = await screen.findByTestId('stats-top-track-7');
		expect(track).toHaveTextContent('Song');
		expect(track).toHaveTextContent('Band');
		expect(track).toHaveTextContent('5');
		const album = screen.getByTestId('stats-top-album-3');
		expect(album).toHaveTextContent('Record');
		expect(album).toHaveTextContent('9');
		expect(within(album).getByRole('link')).toHaveAttribute('href', '/album/3');
		expect(screen.getByTestId('stats-top-artist-2')).toHaveTextContent('12');
	});

	it('switches period', async () => {
		render(Page);
		await screen.findByTestId('stats-listens');

		await fireEvent.click(screen.getByTestId('stats-period-week-button'));

		expect(api.getStats).toHaveBeenLastCalledWith('week');
		expect(screen.getByTestId('stats-period-week-button')).toHaveAttribute('aria-pressed', 'true');
	});

	it('says so when nothing was listened to', async () => {
		vi.mocked(api.getStats).mockResolvedValue(empty);

		render(Page);

		expect(await screen.findByTestId('stats-empty')).toBeInTheDocument();
		expect(screen.queryByTestId('stats-top-tracks')).not.toBeInTheDocument();
	});
});
