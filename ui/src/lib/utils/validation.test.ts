import { describe, expect, it } from 'vitest';
import {
	PASSWORD_MAX_LENGTH,
	RESET_CODE_LENGTH,
	validatePassword,
	validateUsername
} from './validation';

describe('validatePassword', () => {
	it('rejects an empty password', () => {
		expect(validatePassword('').valid).toBe(false);
	});

	it('accepts a single-character password (warn, do not block)', () => {
		expect(validatePassword('p').valid).toBe(true);
	});

	it('accepts a password at the maximum length', () => {
		expect(validatePassword('a'.repeat(PASSWORD_MAX_LENGTH)).valid).toBe(true);
	});

	it('rejects a password one character above the maximum', () => {
		expect(validatePassword('a'.repeat(PASSWORD_MAX_LENGTH + 1)).valid).toBe(false);
	});
});

describe('validateUsername', () => {
	it('rejects an empty username', () => {
		expect(validateUsername('').valid).toBe(false);
	});

	it('accepts a username in range', () => {
		expect(validateUsername('alice').valid).toBe(true);
	});
});

describe('RESET_CODE_LENGTH', () => {
	it('is 8', () => {
		expect(RESET_CODE_LENGTH).toBe(8);
	});
});
