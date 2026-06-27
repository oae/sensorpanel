package music

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/oae/sensorpanel/pkg/paths"
)

const (
	lyricsCacheVersion = 1
	lyricsFoundTTL     = 90 * 24 * time.Hour
	lyricsNotFoundTTL  = 24 * time.Hour
)

type lyricsCacheEntry struct {
	Version     int         `json:"version"`
	CachedAt    time.Time   `json:"cached_at"`
	Status      string      `json:"status"`
	Lyrics      []LyricLine `json:"lyrics,omitempty"`
	PlainLyrics string      `json:"plain_lyrics,omitempty"`
}

func lyricsCacheKey(title, artist string, duration float64) string {
	normalized := strings.ToLower(strings.TrimSpace(title)) + "\x00" +
		strings.ToLower(strings.TrimSpace(artist)) + "\x00" +
		strconv.Itoa(int(duration+0.5))
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}

func lyricsCachePath(key string) (string, error) {
	cacheDir, err := paths.CacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cacheDir, "lyrics", key+".json"), nil
}

func loadLyricsCache(key string, now time.Time) (lyricsCacheEntry, bool) {
	path, err := lyricsCachePath(key)
	if err != nil {
		return lyricsCacheEntry{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return lyricsCacheEntry{}, false
	}

	var entry lyricsCacheEntry
	if err := json.Unmarshal(data, &entry); err != nil || entry.Version != lyricsCacheVersion {
		return lyricsCacheEntry{}, false
	}

	ttl := lyricsFoundTTL
	if entry.Status == "not_found" {
		ttl = lyricsNotFoundTTL
	} else if entry.Status != "synced" && entry.Status != "plain" {
		return lyricsCacheEntry{}, false
	}
	if entry.CachedAt.IsZero() || now.Sub(entry.CachedAt) > ttl || now.Before(entry.CachedAt) {
		return lyricsCacheEntry{}, false
	}
	return entry, true
}

func storeLyricsCache(key string, entry lyricsCacheEntry) error {
	path, err := lyricsCachePath(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	entry.Version = lyricsCacheVersion
	entry.CachedAt = time.Now().UTC()
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	temp, err := os.CreateTemp(filepath.Dir(path), ".lyrics-*.tmp")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}
