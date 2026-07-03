import { describe, expect, it } from 'vitest';
import { matchesQuery, normalizeForMatch } from './text-match';

describe('normalizeForMatch', () => {
	it('lowercases', () => {
		expect(normalizeForMatch('ABBA')).toBe('abba');
	});

	it('strips diacritics', () => {
		expect(normalizeForMatch('Björk')).toBe('bjork');
	});

	it('strips diacritics from multiple accented characters', () => {
		expect(normalizeForMatch('Café Tacvba')).toBe('cafe tacvba');
	});
});

describe('matchesQuery', () => {
	it('matches a plain substring, case-insensitively', () => {
		expect(matchesQuery('Solace', 'sol')).toBe(true);
		expect(matchesQuery('Solace', 'SOL')).toBe(true);
	});

	it('matches across diacritics on either side', () => {
		expect(matchesQuery('Björk', 'bjork')).toBe(true);
		expect(matchesQuery('Bjork', 'björk')).toBe(true);
	});

	it('returns false when the substring is absent', () => {
		expect(matchesQuery('Solace', 'xyz')).toBe(false);
	});

	it('treats an empty or whitespace-only query as matching everything', () => {
		expect(matchesQuery('Solace', '')).toBe(true);
		expect(matchesQuery('Solace', '   ')).toBe(true);
		expect(matchesQuery(undefined, '')).toBe(true);
	});

	it('returns false for a non-empty query against an undefined value', () => {
		expect(matchesQuery(undefined, 'sol')).toBe(false);
		expect(matchesQuery(null, 'sol')).toBe(false);
	});
});
