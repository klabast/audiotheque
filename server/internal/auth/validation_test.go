package auth

import "testing"

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{"empty", "", true},
		{"single character accepted (warn, don't block)", "p", false},
		{"in range", "a-normal-password", false},
		{"at maximum", makeString(MaxPasswordLength), false},
		{"one above maximum", makeString(MaxPasswordLength + 1), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePassword(tt.password)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePassword(%q) error = %v, wantErr %v", tt.password, err, tt.wantErr)
			}
		})
	}
}

func TestValidateUsername(t *testing.T) {
	tests := []struct {
		name     string
		username string
		wantErr  bool
	}{
		{"empty", "", true},
		{"one below minimum", "a", true},
		{"at minimum", "ab", false},
		{"in range", "alice", false},
		{"at maximum", makeString(MaxUsernameLength), false},
		{"one above maximum", makeString(MaxUsernameLength + 1), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateUsername(tt.username)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateUsername(%q) error = %v, wantErr %v", tt.username, err, tt.wantErr)
			}
		})
	}
}

func TestValidateResetCode(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		wantErr bool
	}{
		{"empty", "", true},
		{"one short", "1234567", true},
		{"exact length", "12345678", false},
		{"one long", "123456789", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateResetCode(tt.code)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateResetCode(%q) error = %v, wantErr %v", tt.code, err, tt.wantErr)
			}
		})
	}
}

func makeString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'a'
	}
	return string(b)
}
