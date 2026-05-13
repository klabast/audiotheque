package config

import "testing"

func TestGetPortDefault(t *testing.T) {
	t.Setenv("AUDIOD_PORT", "")
	if got := GetPort(); got != "8080" {
		t.Errorf("GetPort() with empty AUDIOD_PORT = %q, want %q", got, "8080")
	}
}

func TestGetPortFromEnv(t *testing.T) {
	t.Setenv("AUDIOD_PORT", "8880")
	if got := GetPort(); got != "8880" {
		t.Errorf("GetPort() with AUDIOD_PORT=8880 = %q, want %q", got, "8880")
	}
}
