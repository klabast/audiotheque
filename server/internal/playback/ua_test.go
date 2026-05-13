package playback

import "testing"

func TestDeviceNameFromUA(t *testing.T) {
	cases := []struct {
		name string
		ua   string
		want string
	}{
		{"empty", "", "Browser"},
		{
			"Chrome on macOS",
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			"Chrome on macOS",
		},
		{
			"Safari on iPhone",
			"Mozilla/5.0 (iPhone; CPU iPhone OS 17_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Mobile/15E148 Safari/604.1",
			"Safari on iPhone",
		},
		{
			"Firefox on Linux",
			"Mozilla/5.0 (X11; Linux x86_64; rv:122.0) Gecko/20100101 Firefox/122.0",
			"Firefox on Linux",
		},
		{
			"Edge on Windows",
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0",
			"Edge on Windows",
		},
		{
			"Chrome on Android",
			"Mozilla/5.0 (Linux; Android 13; Pixel 7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36",
			"Chrome on Android",
		},
		{
			"Unknown browser falls back",
			"SomeRobot/1.0 (Custom OS)",
			"Browser",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := DeviceNameFromUA(tc.ua); got != tc.want {
				t.Errorf("DeviceNameFromUA(%q) = %q; want %q", tc.ua, got, tc.want)
			}
		})
	}
}
