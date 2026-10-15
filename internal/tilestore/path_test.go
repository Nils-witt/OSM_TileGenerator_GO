package tilestore

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"testing"
)

func TestTilePath(t *testing.T) {
	t.Parallel()

	got := TilePath("/tiles", 14, 8192, 5461)

	want := filepath.FromSlash("/tiles/14/8192/5461.png")
	if got != want {
		t.Errorf("TilePath() = %q, want %q", got, want)
	}
}

func TestTileDirPath(t *testing.T) {
	t.Parallel()

	got := TileDirPath("/tiles", 14, 8192)

	want := filepath.FromSlash("/tiles/14/8192")
	if got != want {
		t.Errorf("TileDirPath() = %q, want %q", got, want)
	}
}

func sortCoords(c []Coord) {
	sort.Slice(c, func(i, j int) bool {
		if c[i].X != c[j].X {
			return c[i].X < c[j].X
		}

		return c[i].Y < c[j].Y
	})
}

func TestWalk(t *testing.T) {
	t.Parallel()

	t.Run("collects tile coordinates and ignores non-tile entries", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()

		mustMkTile := func(x, y int) {
			xDir := filepath.Join(dir, strconv.Itoa(x))
			if err := os.MkdirAll(xDir, 0o755); err != nil {
				t.Fatalf("mkdir %s: %v", xDir, err)
			}

			if err := os.WriteFile(filepath.Join(xDir, strconv.Itoa(y)+".png"), []byte("x"), 0o644); err != nil {
				t.Fatalf("write tile %d/%d: %v", x, y, err)
			}
		}
		mustMkTile(1, 2)
		mustMkTile(1, 3)
		mustMkTile(4, 5)

		// Entries that should be ignored: a non-numeric x directory, a file
		// (not a directory) at the top level, and a non-.png / non-numeric
		// entry inside a valid x directory.
		if err := os.MkdirAll(filepath.Join(dir, "not-a-number"), 0o755); err != nil {
			t.Fatalf("mkdir bogus x dir: %v", err)
		}

		if err := os.WriteFile(filepath.Join(dir, "stray.txt"), []byte("x"), 0o644); err != nil {
			t.Fatalf("write stray file: %v", err)
		}

		if err := os.WriteFile(filepath.Join(dir, "1", "readme.txt"), []byte("x"), 0o644); err != nil {
			t.Fatalf("write stray file in x dir: %v", err)
		}

		if err := os.WriteFile(filepath.Join(dir, "1", "notanumber.png"), []byte("x"), 0o644); err != nil {
			t.Fatalf("write non-numeric tile: %v", err)
		}

		got, err := Walk(dir)
		if err != nil {
			t.Fatalf("Walk() error = %v", err)
		}

		sortCoords(got)

		want := []Coord{{X: 1, Y: 2}, {X: 1, Y: 3}, {X: 4, Y: 5}}
		if len(got) != len(want) {
			t.Fatalf("Walk() = %v, want %v", got, want)
		}

		for i := range want {
			if got[i] != want[i] {
				t.Errorf("Walk()[%d] = %v, want %v", i, got[i], want[i])
			}
		}
	})

	t.Run("missing directory returns error", func(t *testing.T) {
		t.Parallel()

		_, err := Walk(filepath.Join(t.TempDir(), "missing"))
		if err == nil {
			t.Fatal("Walk() error = nil, want error for missing directory")
		}
	})

	t.Run("empty directory returns no tiles", func(t *testing.T) {
		t.Parallel()

		got, err := Walk(t.TempDir())
		if err != nil {
			t.Fatalf("Walk() error = %v", err)
		}

		if len(got) != 0 {
			t.Errorf("Walk() = %v, want empty", got)
		}
	})
}
