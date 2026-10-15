package version

import (
	"strings"
	"testing"
)

//nolint:paralleltest // subtests reassign the package-level Version/Commit/Date vars
func TestString(t *testing.T) {
	t.Run("uses injected build vars when set", func(t *testing.T) {
		origVersion, origCommit, origDate := Version, Commit, Date

		t.Cleanup(func() { Version, Commit, Date = origVersion, origCommit, origDate })

		Version, Commit, Date = "v1.2.3", "abc1234", "2026-08-08T00:00:00Z"

		got := String()
		for _, want := range []string{"v1.2.3", "abc1234", "2026-08-08T00:00:00Z"} {
			if !strings.Contains(got, want) {
				t.Errorf("String() = %q, want substring %q", got, want)
			}
		}
	})

	t.Run("falls back to VCS info when defaults are unset", func(t *testing.T) {
		got := String()
		if got == "" {
			t.Fatal("String() returned empty string")
		}

		if !strings.HasPrefix(got, "osm-tile-generator ") {
			t.Errorf("String() = %q, want prefix %q", got, "osm-tile-generator ")
		}
	})
}
