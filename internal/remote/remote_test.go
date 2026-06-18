package remote

import "testing"

func TestInjectCredentials(t *testing.T) {
	cases := []struct {
		url, user, pass, want string
	}{
		{"https://host/r.git", "u", "p", "https://u:p@host/r.git"},
		{"https://host/r.git", "u", "", "https://u@host/r.git"},
		{"ssh://host/r.git", "git", "", "ssh://git@host/r.git"},
		{"ssh://git@host/r.git", "other", "", "ssh://git@host/r.git"},
		{"git@host:r.git", "u", "", "git@host:r.git"},
		{"https://host/r.git", "", "", "https://host/r.git"},
	}
	for _, c := range cases {
		if got := injectCredentials(c.url, c.user, c.pass); got != c.want {
			t.Errorf("injectCredentials(%q,%q,%q) = %q, want %q", c.url, c.user, c.pass, got, c.want)
		}
	}
}
