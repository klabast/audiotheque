/**
 * Centralized validation rules for the application
 */

import * as m from '$lib/paraglide/messages';

// PASSWORD_MIN_LENGTH is intentionally 1 (non-empty). Policy: warn, don't block.
// Any non-empty password is accepted; weak ones get a non-blocking warning via
// assessPassword(). See docs/dev/plans/2026-05-14-roadmap-may.md §7.
export const PASSWORD_MIN_LENGTH = 1;
export const PASSWORD_MAX_LENGTH = 64;
export const USERNAME_MIN_LENGTH = 2;
export const USERNAME_MAX_LENGTH = 32;

// Anything below this is flagged as "short" — recommend length + password manager.
const PASSWORD_SHORT_THRESHOLD = 12;

export interface ValidationResult {
	valid: boolean;
	error?: string;
}

export function validatePassword(password: string): ValidationResult {
	if (!password) {
		return { valid: false, error: m['errors.password_required']() };
	}

	if (password.length > PASSWORD_MAX_LENGTH) {
		return {
			valid: false,
			error: m['errors.password_max_length']({ maxLength: PASSWORD_MAX_LENGTH })
		};
	}

	return { valid: true };
}

export type PasswordWarning = 'short' | 'equals_username';

/**
 * Non-blocking weakness check. Returns an array of warning reasons; an empty
 * array means "fine, no warning to show". Intentionally minimal — we warn on
 * short and on password-equals-username, nothing else. This is a personal
 * streaming app, not a password education tool.
 */
export function assessPassword(password: string, username?: string): PasswordWarning[] {
	const warnings: PasswordWarning[] = [];
	if (!password) return warnings;

	if (password.length < PASSWORD_SHORT_THRESHOLD) {
		warnings.push('short');
	}
	if (username && password.toLowerCase() === username.toLowerCase()) {
		warnings.push('equals_username');
	}
	return warnings;
}

export function validateUsername(username: string): ValidationResult {
	if (!username) {
		return { valid: false, error: m['errors.username_required']() };
	}

	if (username.length < USERNAME_MIN_LENGTH) {
		return {
			valid: false,
			error: m['errors.username_min_length']({ minLength: USERNAME_MIN_LENGTH })
		};
	}

	if (username.length > USERNAME_MAX_LENGTH) {
		return {
			valid: false,
			error: m['errors.username_max_length']({ maxLength: USERNAME_MAX_LENGTH })
		};
	}

	// Only allow alphanumeric, underscore, hyphen
	if (!/^[a-zA-Z0-9_-]+$/.test(username)) {
		return {
			valid: false,
			error: m['errors.username_invalid_chars']()
		};
	}

	return { valid: true };
}

export function validatePasswordMatch(password: string, confirmPassword: string): ValidationResult {
	if (password !== confirmPassword) {
		return { valid: false, error: m['errors.passwords_no_match']() };
	}
	return { valid: true };
}
