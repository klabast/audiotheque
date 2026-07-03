import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen, fireEvent, within } from '@testing-library/svelte';
import '@testing-library/jest-dom/vitest';
import { writable } from 'svelte/store';
import Page from './+page.svelte';

// jsdom has no ResizeObserver — the page observes its grid container to
// pick column count from width. A no-op stub is enough for these tests.
class ResizeObserverStub {
	observe() {}
	disconnect() {}
	unobserve() {}
}
vi.stubGlobal('ResizeObserver', ResizeObserverStub);

const pageStore = writable({ url: new URL('http://localhost/') });

vi.mock('$app/stores', () => ({
	page: {
		subscribe: (fn: (value: { url: URL }) => void) => pageStore.subscribe(fn)
	}
}));

const goto = vi.fn().mockResolvedValue(undefined);
vi.mock('$app/navigation', () => ({
	goto: (...args: unknown[]) => goto(...args),
	beforeNavigate: () => {}
}));

vi.mock('$lib/services/api', () => ({
	api: {
		listLibraries: vi.fn(),
		listAlbums: vi.fn(),
		searchLibrary: vi.fn(),
		playAlbum: vi.fn()
	}
}));

vi.mock('$lib/stores/playback.svelte', () => ({
	playback: {
		playAlbum: vi.fn().mockResolvedValue(undefined),
		currentTrackId: undefined,
		isPlaying: false,
		isPaused: false,
		session: null
	}
}));

const library = { id: 1, name: 'Test Music' };

const albums = [
	{ id: 1, title: 'Solace', artistName: 'RÜFÜS DU SOL', isHiRes: false },
	{ id: 2, title: 'Discovery', artistName: 'Daft Punk', isHiRes: false },
	{ id: 3, title: 'Random Access Memories', artistName: 'Daft Punk', isHiRes: true }
];

function setUrl(search: string) {
	pageStore.set({ url: new URL(`http://localhost/${search}`) });
}

describe('Library page — search filtering', () => {
	beforeEach(async () => {
		vi.clearAllMocks();
		goto.mockClear();
		setUrl('');
		const { api } = await import('$lib/services/api');
		vi.mocked(api.listLibraries).mockResolvedValue([library] as never);
		vi.mocked(api.listAlbums).mockResolvedValue(albums as never);
	});

	it('shows every album when there is no query', async () => {
		render(Page);

		expect(await screen.findByTestId('album-card-1')).toBeInTheDocument();
		expect(screen.getByTestId('album-card-2')).toBeInTheDocument();
		expect(screen.getByTestId('album-card-3')).toBeInTheDocument();
		expect(screen.queryByTestId('search-scope-tabs')).not.toBeInTheDocument();
	});

	it('filters the album grid by title match in the albums scope', async () => {
		setUrl('?q=disco');
		render(Page);

		await screen.findByTestId('album-card-2');
		expect(screen.queryByTestId('album-card-1')).not.toBeInTheDocument();
		expect(screen.queryByTestId('album-card-3')).not.toBeInTheDocument();
	});

	it('matches diacritic-insensitively and case-insensitively', async () => {
		setUrl('?q=rufus');
		render(Page);

		expect(await screen.findByTestId('album-card-1')).toBeInTheDocument();
	});

	it('matches on artist name too in the default albums scope', async () => {
		setUrl('?q=daft');
		render(Page);

		await screen.findByTestId('album-card-2');
		expect(screen.getByTestId('album-card-3')).toBeInTheDocument();
		expect(screen.queryByTestId('album-card-1')).not.toBeInTheDocument();
	});

	it('shows the empty state when nothing matches', async () => {
		setUrl('?q=zzzznotfound');
		render(Page);

		expect(await screen.findByTestId('search-empty')).toBeInTheDocument();
	});

	it('shows scope tabs only when there is a query, defaulting to albums', async () => {
		setUrl('?q=daft');
		render(Page);

		const tabs = await screen.findByTestId('search-scope-tabs');
		expect(within(tabs).getByTestId('search-scope-albums')).toHaveAttribute(
			'aria-selected',
			'true'
		);
	});

	it('switching to the artists scope keeps the query but updates the URL scope param', async () => {
		setUrl('?q=daft');
		render(Page);

		const artistsTab = await screen.findByTestId('search-scope-artists');
		await fireEvent.click(artistsTab);

		expect(goto).toHaveBeenCalledWith('?scope=artists&q=daft', expect.any(Object));
	});

	it('artists scope filters by artist name only, not album title', async () => {
		setUrl('?q=discovery&scope=artists');
		render(Page);

		// "Discovery" is an album title, not an artist name — no album should match.
		expect(await screen.findByTestId('search-empty')).toBeInTheDocument();
	});

	it('tracks scope debounces a call to the search endpoint and renders results', async () => {
		vi.useFakeTimers();
		const { api } = await import('$lib/services/api');
		vi.mocked(api.searchLibrary).mockResolvedValue({
			albums: [],
			artists: [],
			tracks: [{ id: 101, title: 'Harder Better Faster Stronger', artist: 'Daft Punk', albumId: 2 }]
		} as never);

		setUrl('?q=harder&scope=tracks');
		render(Page);
		await vi.waitFor(() => expect(screen.getByTestId('library-toolbar')).toBeInTheDocument());

		await vi.advanceTimersByTimeAsync(250);

		expect(api.searchLibrary).toHaveBeenCalledWith(1, 'harder');
		expect(await screen.findByTestId('track-search-result-101')).toBeInTheDocument();
		vi.useRealTimers();
	});
});
