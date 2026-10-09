package hyperuplink

import "testing"

func TestOriginsLoseTheirAPISuffix(t *testing.T) {
	cases := []struct {
		raw     string
		origin  string
		trimmed bool
	}{
		{"https://board.example:3001", "https://board.example:3001", false},
		{"https://board.example:3001/", "https://board.example:3001", false},
		{"https://api.hyperup.link", "https://api.hyperup.link", false},
		{"https://board.example/api/v1", "https://board.example", true},
		{"https://board.example/api/v1/", "https://board.example", true},
		{"https://board.example:3001/api", "https://board.example:3001", true},
		{"https://board.example/forum/api/v1", "https://board.example/forum", true},
		{" https://board.example/api ", "https://board.example", true},
		{"", "", false},
	}

	for _, c := range cases {
		origin, trimmed := apiOrigin(c.raw)
		if origin != c.origin || trimmed != c.trimmed {
			t.Errorf("apiOrigin(%q) = %q, %v; want %q, %v", c.raw, origin, trimmed, c.origin, c.trimmed)
		}
	}
}
