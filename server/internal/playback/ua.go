package playback

import "strings"

// DeviceNameFromUA returns a friendly device label from a User-Agent header.
// The exact UA string is variable and lies (Chrome calls itself Safari, etc.)
// so we only try to recognise a few common combos and fall back to "Browser"
// otherwise. Worst case we get "Browser" instead of "Chrome on macOS"; the
// device is still functional.
func DeviceNameFromUA(ua string) string {
	if ua == "" {
		return "Browser"
	}
	lower := strings.ToLower(ua)

	browser := "Browser"
	switch {
	case strings.Contains(lower, "edg/"):
		browser = "Edge"
	case strings.Contains(lower, "firefox/"):
		browser = "Firefox"
	case strings.Contains(lower, "chrome/") && !strings.Contains(lower, "chromium/"):
		browser = "Chrome"
	case strings.Contains(lower, "chromium/"):
		browser = "Chromium"
	case strings.Contains(lower, "safari/"):
		// Note: Chrome includes "Safari/" too, so this branch only fires
		// after the Chrome check above filters Chrome out.
		browser = "Safari"
	}

	platform := ""
	switch {
	case strings.Contains(lower, "iphone"):
		platform = "iPhone"
	case strings.Contains(lower, "ipad"):
		platform = "iPad"
	case strings.Contains(lower, "android"):
		platform = "Android"
	case strings.Contains(lower, "mac os x"), strings.Contains(lower, "macintosh"):
		platform = "macOS"
	case strings.Contains(lower, "windows"):
		platform = "Windows"
	case strings.Contains(lower, "linux"):
		platform = "Linux"
	}

	if platform == "" {
		return browser
	}
	return browser + " on " + platform
}
