// Branding constants — single source of truth for the user-facing brand.
//
// Anything that displays the product name to a user (page titles, welcome
// screens, error toasts, settings page chrome) should import APP_NAME from
// here. i18n messages that mention the product name take it as a `{appName}`
// parameter and the caller supplies APP_NAME.
//
// Internal identifiers (binary name, cookie name, localStorage keys, env
// var prefix) are intentionally decoupled — they live under the "audiod"
// internal name so renaming the brand never breaks existing installs.
//
// Rename procedure: change APP_NAME below, run tests, commit.

export const APP_NAME = 'Audiotheque';

export const APP_TAGLINE = 'Self-hosted high-resolution music streaming';

export const APP_DESCRIPTION =
	'Self-hosted music streaming: browse your collection from any browser, ' +
	'sync playback across every tab and device in real time, and transfer ' +
	'playback to MPD speakers around the house.';
