import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/svelte';
import '@testing-library/jest-dom/vitest';
import AudSearchBox from './AudSearchBox.svelte';

const page = vi.hoisted(() => ({ url: new URL('http://localhost/') }));
vi.mock('$app/state', () => ({ page }));

const goto = vi.fn().mockResolvedValue(undefined);
vi.mock('$app/navigation', () => ({
	goto: (...args: unknown[]) => goto(...args)
}));

function setUrl(url: string) {
	page.url = new URL(url);
}

describe('AudSearchBox', () => {
	beforeEach(() => {
		goto.mockClear();
		goto.mockResolvedValue(undefined);
		setUrl('http://localhost/');
	});

	it('reflects ?q= from the URL when on the library page', () => {
		setUrl('http://localhost/?q=daft');
		render(AudSearchBox);

		expect(screen.getByTestId('search-input')).toHaveValue('daft');
	});

	it('updates the URL immediately on every keystroke while on the library page', async () => {
		render(AudSearchBox);

		const input = screen.getByTestId('search-input');
		await fireEvent.input(input, { target: { value: 'solace' } });

		expect(goto).toHaveBeenCalledWith('?q=solace', {
			replaceState: true,
			reset: false
		});
	});

	it('navigates to the library page with ?q= when typing from another route', async () => {
		setUrl('http://localhost/album/42');
		render(AudSearchBox);

		const input = screen.getByTestId('search-input');
		await fireEvent.input(input, { target: { value: 'daft' } });

		expect(goto).toHaveBeenCalledWith('/?q=daft');
	});

	it('shows a clear button once there is text, and Escape clears the query', async () => {
		setUrl('http://localhost/?q=daft');
		render(AudSearchBox);

		expect(screen.getByTestId('search-clear-button')).toBeInTheDocument();

		const input = screen.getByTestId('search-input');
		await fireEvent.keyDown(input, { key: 'Escape' });

		expect(goto).toHaveBeenCalledWith('?', {
			replaceState: true,
			reset: false
		});
	});

	it('clearing also drops the scope param, which only has meaning alongside a query', async () => {
		setUrl('http://localhost/?q=daft&scope=tracks');
		render(AudSearchBox);

		await fireEvent.click(screen.getByTestId('search-clear-button'));

		expect(goto).toHaveBeenCalledWith('?', {
			replaceState: true,
			reset: false
		});
	});

	it('preserves unrelated params (e.g. hiRes) when updating the query', async () => {
		setUrl('http://localhost/?hiRes=true');
		render(AudSearchBox);

		const input = screen.getByTestId('search-input');
		await fireEvent.input(input, { target: { value: 'daft' } });

		expect(goto).toHaveBeenCalledWith('?q=daft&hiRes=true', {
			replaceState: true,
			reset: false
		});
	});
});
