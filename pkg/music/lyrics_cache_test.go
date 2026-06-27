package music

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLyricsCacheRoundTrip(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	key := lyricsCacheKey("Track", "Artist", 123.4)
	want := lyricsCacheEntry{
		Status:      "synced",
		Lyrics:      []LyricLine{{Time: 1.25, Text: "line"}},
		PlainLyrics: "line",
	}
	if err := storeLyricsCache(key, want); err != nil {
		t.Fatalf("storeLyricsCache() error = %v", err)
	}

	got, ok := loadLyricsCache(key, time.Now())
	if !ok {
		fatalCacheFile(t, key, "loadLyricsCache() missed stored entry")
	}
	if got.Status != want.Status || got.PlainLyrics != want.PlainLyrics || len(got.Lyrics) != 1 || got.Lyrics[0] != want.Lyrics[0] {
		t.Fatalf("loadLyricsCache() = %#v, want %#v", got, want)
	}
}

func TestLyricsCacheExpiration(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	key := lyricsCacheKey("Missing", "Artist", 100)
	entry := lyricsCacheEntry{Status: "not_found"}
	if err := storeLyricsCache(key, entry); err != nil {
		t.Fatalf("storeLyricsCache() error = %v", err)
	}

	if _, ok := loadLyricsCache(key, time.Now().Add(lyricsNotFoundTTL+time.Second)); ok {
		t.Fatal("loadLyricsCache() returned an expired not-found entry")
	}
}

func TestLyricsCacheKeyNormalization(t *testing.T) {
	a := lyricsCacheKey(" Track ", "ARTIST", 123.49)
	b := lyricsCacheKey("track", "artist", 123.4)
	if a != b {
		t.Fatalf("normalized keys differ: %q != %q", a, b)
	}
}

func fatalCacheFile(t *testing.T, key, message string) {
	t.Helper()
	root := os.Getenv("XDG_CACHE_HOME")
	path := filepath.Join(root, "sensorpanel", "lyrics", key+".json")
	data, _ := os.ReadFile(path)
	t.Fatalf("%s (%s: %s)", message, path, data)
}
