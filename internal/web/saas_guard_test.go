package web

import "testing"

func TestCanonicalSaaSPathAllowsPublicAssets(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{path: "/static/mfa.css", want: true},
		{path: "/static/style.css", want: true},
		{path: "/private/file.css", want: false},
	}

	for _, tt := range tests {
		if got := canonicalSaaSPath(tt.path); got != tt.want {
			t.Errorf("canonicalSaaSPath(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}
