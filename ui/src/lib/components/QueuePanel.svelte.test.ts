import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen, fireEvent, within } from '@testing-library/svelte';
import '@testing-library/jest-dom/vitest';
import QueuePanel from './QueuePanel.svelte';
import { playback } from '#lib/stores/playback.svelte';
import { api } from '#lib/services/api';

const tracks: Record<number, object> = {
	1: { id: 1, title: 'Now', artistName: 'A', albumId: 100, albumTitle: 'Source Album' },
	2: { id: 2, title: 'Source Next', artistName: 'A', albumId: 100, albumTitle: 'Source Album' },
	7: { id: 7, title: 'Queued One', artistName: 'B', albumId: 3, albumTitle: 'Other Album' },
	8: { id: 8, title: 'Queued Two', artistName: 'B', albumId: 3, albumTitle: 'Other Album' }
};

vi.mock('#lib/services/api', () => ({
	api: {
		lookupTracks: vi.fn(),
		removeFromQueue: vi.fn(),
		moveQueueItem: vi.fn(),
		playFromQueue: vi.fn(),
		clearQueue: vi.fn()
	}
}));

const session = {
	state: 'playing',
	deviceId: 'mpd',
	current: { trackId: 1, position: 0 },
	queue: [
		{ id: 'qa', trackId: 7, addedFrom: { type: 'album', id: 3 } },
		{ id: 'qb', trackId: 8, addedFrom: { type: 'album', id: 3 } }
	],
	source: { type: 'album', id: 100, remaining: [2] }
};

beforeEach(() => {
	vi.clearAllMocks();
	vi.mocked(api.lookupTracks).mockImplementation(async (ids: number[]) =>
		ids.filter((id) => id in tracks).map((id) => tracks[id])
	);
	for (const op of ['removeFromQueue', 'moveQueueItem', 'playFromQueue', 'clearQueue'] as const) {
		vi.mocked(api[op]).mockResolvedValue(session);
	}
	playback.updateSession(session);
});

async function renderPanel() {
	render(QueuePanel);
	return await screen.findByTestId('queue-item-qa');
}

describe('QueuePanel', () => {
	it('lists queued tracks with the album they came from, before the source', async () => {
		const first = await renderPanel();

		expect(first).toHaveTextContent('Queued One');
		expect(first).toHaveTextContent('Other Album');
		expect(screen.getByTestId('queue-explicit')).toHaveTextContent(/Queued One.*Queued Two/);
		expect(await screen.findByTestId('queue-source')).toHaveTextContent('Source Next');
		expect(screen.getByTestId('queue-now-playing')).toHaveTextContent('Now');
	});

	it.each([
		['queue-item-remove-button', 'removeFromQueue', ['qb']],
		['queue-item-move-up-button', 'moveQueueItem', ['qb', 0]],
		['queue-item-play-button', 'playFromQueue', ['qb']]
	] as const)('%s on an entry calls %s', async (button, apiMethod, args) => {
		await renderPanel();
		const second = screen.getByTestId('queue-item-qb');

		await fireEvent.click(within(second).getByTestId(button));

		expect(api[apiMethod]).toHaveBeenCalledWith(...args);
	});

	it('moves an entry down', async () => {
		const first = await renderPanel();

		await fireEvent.click(within(first).getByTestId('queue-item-move-down-button'));

		expect(api.moveQueueItem).toHaveBeenCalledWith('qa', 1);
	});

	it('clears the queue', async () => {
		await renderPanel();

		await fireEvent.click(screen.getByTestId('clear-queue-button'));

		expect(api.clearQueue).toHaveBeenCalled();
	});
});
