/**
 * Centralized validation rules for the application
 */

import * as m from '$lib/paraglide/messages';

export const PASSWORD_MIN_LENGTH = 8;
export const PASSWORD_MAX_LENGTH = 64;
export const USERNAME_MIN_LENGTH = 2;
export const USERNAME_MAX_LENGTH = 32;

export interface ValidationResult {
	valid: boolean;
	error?: string;
}

export function validatePassword(password: string): ValidationResult {
	if (!password) {
		return { valid: false, error: m['errors.password_required']() };
	}

	if (password.length < PASSWORD_MIN_LENGTH) {
		return {
			valid: false,
			error: m['errors.password_min_length']({ minLength: PASSWORD_MIN_LENGTH })
		};
	}

	if (password.length > PASSWORD_MAX_LENGTH) {
		return {
			valid: false,
			error: m['errors.password_max_length']({ maxLength: PASSWORD_MAX_LENGTH })
		};
	}

	return { valid: true };
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
