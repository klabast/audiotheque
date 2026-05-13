import { describe, expect, it } from 'vitest';
import { formatDuration } from './format';

describe('formatDuration', () => {
	it('returns 0:00 for 0 ms', () => {
		expect(formatDuration(0)).toBe('0:00');
	});

	it('formats 273000 ms as 4:33', () => {
		expect(formatDuration(273000)).toBe('4:33');
	});

	it('zero-pads seconds < 10', () => {
		expect(formatDuration(9000)).toBe('0:09');
	});

	it('handles exact minute boundary', () => {
		expect(formatDuration(60000)).toBe('1:00');
	});

	it('handles long durations beyond an hour as raw minutes', () => {
		expect(formatDuration(3600000)).toBe('60:00');
	});

	it('floors sub-second remainders to whole seconds', () => {
		expect(formatDuration(273750)).toBe('4:33');
	});
});
