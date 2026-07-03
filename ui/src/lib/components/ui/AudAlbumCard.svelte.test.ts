import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import '@testing-library/jest-dom/vitest';
import AudAlbumCard from './AudAlbumCard.svelte';

describe('AudAlbumCard — year badge', () => {
	it('renders the year when the year prop is set', () => {
		render(AudAlbumCard, {
			props: { id: 1, title: 'OK Computer', artistName: 'Radiohead', year: '1997' }
		});

		expect(screen.getByTestId('album-year-1')).toHaveTextContent('1997');
	});

	it('does not render a year when the year prop is unset', () => {
		render(AudAlbumCard, {
			props: { id: 1, title: 'OK Computer', artistName: 'Radiohead' }
		});

		expect(screen.queryByTestId('album-year-1')).not.toBeInTheDocument();
	});
});
