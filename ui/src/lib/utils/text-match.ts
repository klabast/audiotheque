/** Strips diacritics and lowercases so "Björk" matches "bjork" and "BJÖRK". */
export function normalizeForMatch(value: string): string {
	return value.normalize('NFD').replace(/[̀-ͯ]/g, '').toLowerCase();
}

/** Case- and diacritic-insensitive substring match. Empty query matches everything. */
export function matchesQuery(value: string | undefined | null, query: string): boolean {
	const q = query.trim();
	if (!q) return true;
	if (!value) return false;
	return normalizeForMatch(value).includes(normalizeForMatch(q));
}
