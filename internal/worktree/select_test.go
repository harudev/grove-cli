package worktree

import "testing"

func TestMatchRegex(t *testing.T) {
	cases := []struct {
		target string
		branch string
		want   bool
	}{
		// bare number matches any prefix
		{"169", "feature/TEAM-169", true},
		{"169", "feature/PROJ-169", true},
		{"169", "bugfix/TEAM-169-hotfix", true},
		// bare number must not match a longer number
		{"169", "feature/TEAM-1690", false},
		{"169", "feature/TEAM-2169", false},
		// full key, case-insensitive
		{"TEAM-169", "feature/TEAM-169", true},
		{"team-169", "feature/TEAM-169", true},
		{"DS-169", "feature/TEAM-169", false},
	}
	for _, c := range cases {
		got := matchRegex(c.target).MatchString(c.branch)
		if got != c.want {
			t.Errorf("matchRegex(%q).MatchString(%q) = %v, want %v", c.target, c.branch, got, c.want)
		}
	}
}
