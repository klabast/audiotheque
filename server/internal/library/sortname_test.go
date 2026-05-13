package library

import "testing"

func TestComputeSortName(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		// English articles
		{"The Beatles", "Beatles"},
		{"the rolling stones", "rolling stones"}, // case-insensitive
		{"THE WEEKND", "WEEKND"},
		{"A Tribe Called Quest", "Tribe Called Quest"},
		{"An Anthem", "Anthem"},

		// German articles
		{"Die Toten Hosen", "Toten Hosen"},
		{"Der Plan", "Plan"},
		{"Das Modul", "Modul"},

		// French articles
		{"Le Tigre", "Tigre"},
		{"La Roux", "Roux"},
		{"Les Rita Mitsouko", "Rita Mitsouko"},

		// Don't strip when not actually a leading article
		{"Theory of a Deadman", "Theory of a Deadman"},
		{"Anchor Hanger", "Anchor Hanger"},
		{"Death Cab for Cutie", "Death Cab for Cutie"},

		// Standalone article-only names: keep as-is, otherwise sort_name is empty
		{"The", "The"},
		{"", ""},

		// Tabs/multi-space after article still strip the article
		{"The   Beatles", "Beatles"},
	}

	for _, tc := range cases {
		got := computeSortName(tc.in)
		if got != tc.want {
			t.Errorf("computeSortName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
