import { describe, expect, it, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/svelte';
import '@testing-library/jest-dom/vitest';
import AudSeekBar from './AudSeekBar.svelte';

describe('AudSeekBar', () => {
	it('is a focusable slider that seeks with the arrow keys', async () => {
		const onSeek = vi.fn();
		render(AudSeekBar, { currentTime: 30, duration: 100, onSeek });

		const bar = screen.getByRole('slider');
		expect(bar).toHaveAttribute('tabindex', '0');
		expect(bar).toHaveAttribute('aria-valuenow', '30');

		await fireEvent.keyDown(bar, { key: 'ArrowRight' });
		expect(onSeek).toHaveBeenCalledWith(35);
	});

	it('is a plain progress bar when not interactive', () => {
		render(AudSeekBar, { currentTime: 30, duration: 100, interactive: false });

		const bar = screen.getByRole('progressbar');
		expect(bar).not.toHaveAttribute('tabindex');
		expect(screen.queryByRole('slider')).not.toBeInTheDocument();
	});

	it('shows elapsed and total time when asked', () => {
		render(AudSeekBar, { currentTime: 65, duration: 200, showTimes: true });

		expect(screen.getByTestId('seek-bar')).toBeInTheDocument();
		expect(screen.getByText('1:05')).toBeInTheDocument();
		expect(screen.getByText('3:20')).toBeInTheDocument();
	});
});
