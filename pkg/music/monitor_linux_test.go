//go:build linux

package music

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestParseLRC(t *testing.T) {
	input := "[00:01.25] First line\n[01:02.50][01:03.00] Repeated\n[ar:Artist]\n[00:04.00]   "
	want := []LyricLine{
		{Time: 1.25, Text: "First line"},
		{Time: 62.5, Text: "Repeated"},
		{Time: 63, Text: "Repeated"},
	}
	if got := ParseLRC(input); !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseLRC() = %#v, want %#v", got, want)
	}
}

func TestSnapshotReturnsCopies(t *testing.T) {
	monitor := NewMonitor()
	monitor.snapshot.Lyrics = []LyricLine{{Time: 1, Text: "line"}}

	snapshot := monitor.Snapshot()
	snapshot.Lyrics[0].Text = "changed"

	if monitor.snapshot.Lyrics[0].Text != "line" {
		t.Fatal("Snapshot() returned shared lyrics storage")
	}
}

func TestBrowserSafeArtURLConvertsExtensionlessFileImage(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "artwork")
	if err != nil {
		t.Fatal(err)
	}
	_, err = file.Write([]byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
		0xde, 0x00, 0x00, 0x00, 0x0c, 0x49, 0x44, 0x41,
		0x54, 0x08, 0xd7, 0x63, 0xf8, 0xff, 0xff, 0x3f,
		0x00, 0x05, 0xfe, 0x02, 0xfe, 0xdc, 0xcc, 0x59,
		0xe7, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e,
		0x44, 0xae, 0x42, 0x60, 0x82,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	got := browserSafeArtURL("file://" + file.Name())
	if !strings.HasPrefix(got, "data:image/png;base64,") {
		t.Fatalf("browserSafeArtURL() = %q, want PNG data URL", got)
	}
}

func TestYoutubeThumbnailURL(t *testing.T) {
	tests := map[string]string{
		"https://www.youtube.com/watch?v=LpNVf8sczqU&list=RD": "https://i.ytimg.com/vi/LpNVf8sczqU/hqdefault.jpg",
		"https://youtu.be/LpNVf8sczqU?t=4":                    "https://i.ytimg.com/vi/LpNVf8sczqU/hqdefault.jpg",
		"https://example.com/watch?v=LpNVf8sczqU":             "",
	}
	for input, want := range tests {
		if got := youtubeThumbnailURL(input); got != want {
			t.Fatalf("youtubeThumbnailURL(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestNormalizeMPRISPosition(t *testing.T) {
	if got := normalizeMPRISPosition(70048016); got != 70.048016 {
		t.Fatalf("normalizeMPRISPosition() = %v, want seconds", got)
	}
	if got := normalizeMPRISPosition(70.5); got != 70.5 {
		t.Fatalf("normalizeMPRISPosition() = %v, want unchanged seconds", got)
	}
}

func TestCleanLyricsTitle(t *testing.T) {
	tests := map[string]string{
		`"Alicia" from Clair Obscur: Expedition 33 | gamescom Opening Night Live 25 | Live Performance`: "Alicia",
		"Blinding Lights [Official Music Video]":                                                        "Blinding Lights",
		"Song Name | Live at Somewhere":                                                                 "Song Name",
		"Clair Obscur: Expedition 33 | Aline [Official Music Video]":                                    "Aline",
	}
	for input, want := range tests {
		if got := cleanLyricsTitle(input); got != want {
			t.Fatalf("cleanLyricsTitle(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestLyricsSearchScorePrefersContextMatch(t *testing.T) {
	result := lyricsSearchResult{
		TrackName:    "Alicia",
		ArtistName:   "Lorien Testard",
		AlbumName:    "Clair Obscur: Expedition 33 (Original Soundtrack)",
		Duration:     170,
		SyncedLyrics: ptrString("[00:01.00] line"),
	}
	score := lyricsSearchScore(result, "Alicia", "thegameawards", `"Alicia" from Clair Obscur: Expedition 33 | gamescom Opening Night Live 25`, 0)
	if score < 100 {
		t.Fatalf("lyricsSearchScore() = %d, want strong match", score)
	}
}

func ptrString(value string) *string {
	return &value
}
