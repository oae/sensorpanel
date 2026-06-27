//go:build linux

// Package music collects now-playing metadata and synchronized lyrics for the
// music dashboard.
package music

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"mime"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const metadataSeparator = "\x1f"

// LyricLine is one timestamped line from an LRC document.
type LyricLine struct {
	Time float64 `json:"time"`
	Text string  `json:"text"`
}

// Snapshot is the complete state consumed by the music dashboard.
type Snapshot struct {
	Available    bool        `json:"available"`
	Player       string      `json:"player,omitempty"`
	Status       string      `json:"status,omitempty"`
	Title        string      `json:"title,omitempty"`
	Artist       string      `json:"artist,omitempty"`
	Album        string      `json:"album,omitempty"`
	ArtURL       string      `json:"art_url,omitempty"`
	Position     float64     `json:"position"`
	Duration     float64     `json:"duration"`
	PositionOK   bool        `json:"position_reliable"`
	Lyrics       []LyricLine `json:"lyrics,omitempty"`
	PlainLyrics  string      `json:"plain_lyrics,omitempty"`
	LyricsStatus string      `json:"lyrics_status,omitempty"`
}

// Monitor maintains current music state in background goroutines.
type Monitor struct {
	mu               sync.RWMutex
	snapshot         Snapshot
	positionAtPoll   float64
	positionTrackKey string
	polledAt         time.Time
	lyricsTrackKey   string
	mediaTrackKey    string
	mediaDuration    float64
	mediaArtURL      string
	artSource        string
	artResolved      string
	httpClient       *http.Client
}

// NewMonitor creates a music monitor.
func NewMonitor() *Monitor {
	return &Monitor{
		httpClient: &http.Client{Timeout: 12 * time.Second},
	}
}

// Start begins metadata collection until ctx is canceled.
func (m *Monitor) Start(ctx context.Context) {
	go m.pollMetadataLoop(ctx)
}

// Snapshot returns a copy of current state with an extrapolated position.
func (m *Monitor) Snapshot() Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := m.snapshot
	result.Lyrics = append([]LyricLine(nil), m.snapshot.Lyrics...)
	result.Position = m.positionAtPoll
	if result.Status == "Playing" && !m.polledAt.IsZero() {
		result.Position += time.Since(m.polledAt).Seconds()
	}
	if result.Duration > 0 && result.Position > result.Duration {
		result.Position = result.Duration
	}
	return result
}

func (m *Monitor) pollMetadataLoop(ctx context.Context) {
	m.pollMetadata(ctx)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.pollMetadata(ctx)
		}
	}
}

func (m *Monitor) pollMetadata(ctx context.Context) {
	format := strings.Join([]string{
		"{{playerName}}", "{{status}}", "{{xesam:title}}", "{{xesam:artist}}",
		"{{xesam:album}}", "{{mpris:artUrl}}", "{{mpris:length}}", "{{xesam:url}}", "{{position}}",
	}, metadataSeparator)
	cmd := exec.CommandContext(ctx, "playerctl", "-a", "metadata", "--format", format)
	output, err := cmd.Output()
	if err != nil {
		m.mu.Lock()
		m.snapshot.Available = false
		m.snapshot.Status = "Stopped"
		m.mu.Unlock()
		return
	}

	var selected []string
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		fields := strings.Split(line, metadataSeparator)
		if len(fields) != 9 {
			continue
		}
		if selected == nil || fields[1] == "Playing" {
			selected = fields
		}
		if fields[1] == "Playing" {
			break
		}
	}
	if selected == nil {
		return
	}

	durationUS, _ := strconv.ParseFloat(selected[6], 64)
	position, err := strconv.ParseFloat(selected[8], 64)
	if err != nil {
		position = m.playerPosition(ctx, selected[0])
	}
	position = normalizeMPRISPosition(position)
	positionOK := true
	trackKey := selected[2] + "\x00" + selected[3] + "\x00" + selected[4] + "\x00" + selected[7]
	artURL := selected[5]
	if artURL == "" {
		artURL = youtubeThumbnailURL(selected[7])
	}
	duration := durationUS / 1_000_000
	if selected[7] != "" && duration <= 0 {
		positionOK = false
	}

	m.mu.Lock()
	if trackKey == m.positionTrackKey && selected[1] == "Playing" && !m.polledAt.IsZero() {
		expected := m.positionAtPoll + time.Since(m.polledAt).Seconds()
		if duration > 0 && expected > duration {
			expected = duration
		}
		if math.Abs(position-expected) <= 5 {
			position = expected
		}
	} else {
		m.positionTrackKey = trackKey
	}
	if trackKey == m.mediaTrackKey {
		if duration <= 0 && m.mediaDuration > 0 {
			duration = m.mediaDuration
		}
		if artURL == "" && m.mediaArtURL != "" {
			artURL = m.mediaArtURL
		}
	}
	m.snapshot.Available = true
	m.snapshot.Player = selected[0]
	m.snapshot.Status = selected[1]
	m.snapshot.Title = selected[2]
	m.snapshot.Artist = selected[3]
	m.snapshot.Album = selected[4]
	m.snapshot.ArtURL = m.resolveArtURL(artURL)
	m.snapshot.Duration = duration
	m.snapshot.PositionOK = positionOK
	m.positionAtPoll = position
	m.polledAt = time.Now()
	needsLyrics := trackKey != m.lyricsTrackKey
	if needsLyrics {
		m.lyricsTrackKey = trackKey
		m.snapshot.Lyrics = nil
		m.snapshot.PlainLyrics = ""
		m.snapshot.LyricsStatus = "loading"
	}
	needsMediaInfo := m.snapshot.Duration <= 0 && selected[7] != "" && trackKey != m.mediaTrackKey
	if needsMediaInfo {
		m.mediaTrackKey = trackKey
		m.mediaDuration = 0
		m.mediaArtURL = ""
	}
	snapshot := m.snapshot
	mediaURL := selected[7]
	m.mu.Unlock()

	if needsLyrics && snapshot.Title != "" {
		go m.fetchLyrics(ctx, trackKey, snapshot.Title, snapshot.Artist, snapshot.Duration)
	}
	if needsMediaInfo {
		go m.fetchMediaInfo(ctx, trackKey, mediaURL)
	}
}

func (m *Monitor) playerPosition(ctx context.Context, player string) float64 {
	output, err := exec.CommandContext(ctx, "playerctl", "--player="+player, "position").Output()
	if err != nil {
		return 0
	}
	position, _ := strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
	return position
}

func normalizeMPRISPosition(position float64) float64 {
	if position > 100000 {
		return position / 1_000_000
	}
	return position
}

func (m *Monitor) resolveArtURL(raw string) string {
	if raw == "" {
		m.artSource = ""
		m.artResolved = ""
		return ""
	}
	if raw == m.artSource {
		return m.artResolved
	}
	resolved := browserSafeArtURL(raw)
	m.artSource = raw
	m.artResolved = resolved
	return resolved
}

func browserSafeArtURL(raw string) string {
	if strings.HasPrefix(raw, "data:") || strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		return raw
	}

	path := raw
	if parsed, err := url.Parse(raw); err == nil && parsed.Scheme != "" {
		if parsed.Scheme != "file" {
			return raw
		}
		path = parsed.Path
	}
	if !filepath.IsAbs(path) {
		return raw
	}

	info, err := os.Stat(path)
	if err != nil || info.IsDir() || info.Size() > 5*1024*1024 {
		return raw
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return raw
	}
	contentType := mime.TypeByExtension(filepath.Ext(path))
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}
	if !strings.HasPrefix(contentType, "image/") {
		return raw
	}
	return fmt.Sprintf("data:%s;base64,%s", contentType, base64.StdEncoding.EncodeToString(data))
}

var youtubeIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

func youtubeThumbnailURL(raw string) string {
	id := youtubeVideoID(raw)
	if id == "" {
		return ""
	}
	return "https://i.ytimg.com/vi/" + id + "/hqdefault.jpg"
}

func youtubeVideoID(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	host := strings.TrimPrefix(strings.ToLower(parsed.Hostname()), "www.")
	switch host {
	case "youtube.com", "music.youtube.com", "m.youtube.com":
		id := parsed.Query().Get("v")
		if youtubeIDPattern.MatchString(id) {
			return id
		}
	case "youtu.be":
		id := strings.Trim(strings.TrimPrefix(parsed.Path, "/"), "/")
		if youtubeIDPattern.MatchString(id) {
			return id
		}
	}
	return ""
}

type mediaInfo struct {
	Duration  float64 `json:"duration"`
	Thumbnail string  `json:"thumbnail"`
}

func (m *Monitor) fetchMediaInfo(ctx context.Context, trackKey, mediaURL string) {
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		return
	}
	mediaURL = canonicalMediaInfoURL(mediaURL)
	infoCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	output, err := exec.CommandContext(infoCtx, "yt-dlp", "--dump-json", "--skip-download", "--no-warnings", "--no-playlist", mediaURL).Output()
	if err != nil {
		return
	}
	var info mediaInfo
	if err := json.Unmarshal(output, &info); err != nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if trackKey != m.mediaTrackKey {
		return
	}
	if m.snapshot.Duration <= 0 && info.Duration > 0 {
		m.mediaDuration = info.Duration
		m.snapshot.Duration = info.Duration
	}
	if m.snapshot.ArtURL == "" && info.Thumbnail != "" {
		m.mediaArtURL = info.Thumbnail
		m.snapshot.ArtURL = m.resolveArtURL(info.Thumbnail)
	}
}

func canonicalMediaInfoURL(raw string) string {
	if id := youtubeVideoID(raw); id != "" {
		return "https://www.youtube.com/watch?v=" + id
	}
	return raw
}

type lyricsResponse struct {
	SyncedLyrics *string `json:"syncedLyrics"`
	PlainLyrics  *string `json:"plainLyrics"`
}

type lyricsSearchResult struct {
	TrackName    string  `json:"trackName"`
	ArtistName   string  `json:"artistName"`
	AlbumName    string  `json:"albumName"`
	Duration     float64 `json:"duration"`
	Instrumental bool    `json:"instrumental"`
	SyncedLyrics *string `json:"syncedLyrics"`
	PlainLyrics  *string `json:"plainLyrics"`
}

func (m *Monitor) fetchLyrics(ctx context.Context, trackKey, title, artist string, duration float64) {
	lookupTitle := cleanLyricsTitle(title)
	lookupArtist := cleanLyricsArtist(artist)
	cacheKey := lyricsCacheKey(lookupTitle, lookupArtist, duration)
	if cached, ok := loadLyricsCache(cacheKey, time.Now()); ok {
		m.finishLyrics(trackKey, cached.Lyrics, cached.PlainLyrics, cached.Status)
		return
	}

	entry, found, temporary := m.fetchExactLyrics(ctx, lookupTitle, lookupArtist, duration)
	if temporary {
		m.finishLyrics(trackKey, nil, "", "unavailable")
		return
	}
	if !found {
		entry, found, temporary = m.searchLyrics(ctx, lookupTitle, lookupArtist, title, duration)
		if temporary {
			m.finishLyrics(trackKey, nil, "", "unavailable")
			return
		}
	}
	if !found {
		entry = lyricsCacheEntry{Status: "not_found"}
	}
	_ = storeLyricsCache(cacheKey, entry)
	m.finishLyrics(trackKey, entry.Lyrics, entry.PlainLyrics, entry.Status)
}

func (m *Monitor) fetchExactLyrics(ctx context.Context, title, artist string, duration float64) (lyricsCacheEntry, bool, bool) {
	params := url.Values{
		"track_name":  {title},
		"artist_name": {artist},
	}
	if duration > 0 {
		params.Set("duration", strconv.Itoa(int(duration+0.5)))
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://lrclib.net/api/get?"+params.Encode(), nil)
	if err != nil {
		return lyricsCacheEntry{}, false, true
	}
	request.Header.Set("User-Agent", "sensorpanel/1.0 (music dashboard)")
	response, err := m.httpClient.Do(request)
	if err != nil {
		return lyricsCacheEntry{}, false, true
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return lyricsCacheEntry{}, false, false
	}
	if response.StatusCode != http.StatusOK {
		return lyricsCacheEntry{}, false, true
	}

	var result lyricsResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return lyricsCacheEntry{}, false, true
	}
	entry := lyricsEntryFromPayload(result.SyncedLyrics, result.PlainLyrics, false)
	return entry, entry.Status != "not_found", false
}

func (m *Monitor) searchLyrics(ctx context.Context, title, artist, originalTitle string, duration float64) (lyricsCacheEntry, bool, bool) {
	queries := uniqueNonEmpty(
		title+" "+artist,
		title+" "+lyricsSearchContext(originalTitle),
		title,
	)
	var best lyricsSearchResult
	bestScore := -1
	for _, query := range queries {
		params := url.Values{"q": {query}}
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://lrclib.net/api/search?"+params.Encode(), nil)
		if err != nil {
			continue
		}
		request.Header.Set("User-Agent", "sensorpanel/1.0 (music dashboard)")
		response, err := m.httpClient.Do(request)
		if err != nil {
			return lyricsCacheEntry{}, false, true
		}
		if response.StatusCode != http.StatusOK {
			response.Body.Close()
			return lyricsCacheEntry{}, false, true
		}
		var results []lyricsSearchResult
		err = json.NewDecoder(response.Body).Decode(&results)
		response.Body.Close()
		if err != nil {
			return lyricsCacheEntry{}, false, true
		}
		for _, result := range results {
			score := lyricsSearchScore(result, title, artist, originalTitle, duration)
			if score > bestScore {
				best = result
				bestScore = score
			}
		}
	}
	if bestScore < 45 {
		return lyricsCacheEntry{}, false, false
	}
	entry := lyricsEntryFromPayload(best.SyncedLyrics, best.PlainLyrics, best.Instrumental)
	return entry, entry.Status != "not_found", false
}

func lyricsEntryFromPayload(syncedLyrics, plainLyrics *string, instrumental bool) lyricsCacheEntry {
	synced := ""
	if syncedLyrics != nil {
		synced = *syncedLyrics
	}
	plain := ""
	if plainLyrics != nil {
		plain = *plainLyrics
	}
	lines := ParseLRC(synced)
	status := "plain"
	if len(lines) > 0 {
		status = "synced"
	} else if plain == "" || instrumental {
		status = "not_found"
	}
	return lyricsCacheEntry{
		Status:      status,
		Lyrics:      lines,
		PlainLyrics: plain,
	}
}

var bracketedLyricsNoisePattern = regexp.MustCompile(`(?i)\s*[\[(][^\])]*(official|video|visualizer|lyrics?|live|performance|audio|remaster|remastered|hd|4k)[^\])]*[\])]`)

func cleanLyricsTitle(title string) string {
	title = strings.TrimSpace(title)
	if len(title) >= 2 {
		for opener, closer := range map[rune]rune{'"': '"', '\'': '\'', '“': '”', '‘': '’'} {
			if []rune(title)[0] == opener {
				runes := []rune(title)
				for index := 1; index < len(runes); index++ {
					if runes[index] == closer {
						return strings.TrimSpace(string(runes[1:index]))
					}
				}
			}
		}
	}
	if strings.Contains(title, " | ") {
		title = bestLyricsTitleSegment(strings.Split(title, " | "))
	}
	lower := strings.ToLower(title)
	for _, marker := range []string{" from ", " - official", " [official", " (official"} {
		if index := strings.Index(lower, marker); index >= 0 {
			title = title[:index]
			lower = strings.ToLower(title)
		}
	}
	title = bracketedLyricsNoisePattern.ReplaceAllString(title, "")
	return strings.Trim(strings.TrimSpace(title), `"'“”‘’`)
}

func bestLyricsTitleSegment(segments []string) string {
	best := strings.TrimSpace(segments[0])
	bestScore := -1
	for _, segment := range segments {
		candidate := strings.TrimSpace(bracketedLyricsNoisePattern.ReplaceAllString(segment, ""))
		if candidate == "" {
			continue
		}
		score := 100
		words := strings.Fields(candidate)
		wordCount := len(words)
		for _, word := range words {
			if isLyricsNoiseWord(strings.ToLower(strings.Trim(word, `.,:;!?()[]{}"'“”‘’`))) {
				score -= 24
			}
		}
		if wordCount <= 4 {
			score += 20
		} else if wordCount >= 8 {
			score -= 20
		}
		if len(candidate) <= 28 {
			score += 12
		} else if len(candidate) >= 45 {
			score -= 12
		}
		if score > bestScore || (score == bestScore && len(candidate) < len(best)) {
			best = candidate
			bestScore = score
		}
	}
	return best
}

func isLyricsNoiseWord(word string) bool {
	switch word {
	case "", "from", "official", "music", "video", "visualizer", "lyrics", "lyric",
		"audio", "live", "performance", "remaster", "remastered", "hd", "4k", "mv",
		"feat", "ft":
		return true
	default:
		return false
	}
}

func cleanLyricsArtist(artist string) string {
	artist = strings.TrimSpace(artist)
	for _, suffix := range []string{" - Topic", "VEVO"} {
		artist = strings.TrimSuffix(artist, suffix)
	}
	return strings.TrimSpace(artist)
}

func lyricsSearchContext(originalTitle string) string {
	cleaned := bracketedLyricsNoisePattern.ReplaceAllString(originalTitle, " ")
	cleaned = strings.NewReplacer("|", " ", ":", " ", "\"", " ", "'", " ").Replace(cleaned)
	words := strings.Fields(cleaned)
	var kept []string
	for _, word := range words {
		lower := strings.ToLower(strings.Trim(word, `.,:;!?()[]{}"'“”‘’`))
		if isLyricsNoiseWord(lower) {
			continue
		}
		kept = append(kept, word)
		if len(kept) >= 5 {
			break
		}
	}
	return strings.Join(kept, " ")
}

func lyricsSearchScore(result lyricsSearchResult, title, artist, originalTitle string, duration float64) int {
	score := 0
	resultTitle := normalizeLyricsMatchText(result.TrackName)
	wantTitle := normalizeLyricsMatchText(title)
	if resultTitle == wantTitle {
		score += 80
	} else if strings.Contains(resultTitle, wantTitle) || strings.Contains(wantTitle, resultTitle) {
		score += 45
	}
	resultArtist := normalizeLyricsMatchText(result.ArtistName)
	wantArtist := normalizeLyricsMatchText(artist)
	if wantArtist != "" && (resultArtist == wantArtist || strings.Contains(resultArtist, wantArtist) || strings.Contains(wantArtist, resultArtist)) {
		score += 25
	}
	original := normalizeLyricsMatchText(originalTitle)
	for _, token := range strings.Fields(normalizeLyricsMatchText(result.AlbumName + " " + result.ArtistName)) {
		if len(token) >= 4 && strings.Contains(original, token) {
			score += 8
		}
	}
	if result.SyncedLyrics != nil && *result.SyncedLyrics != "" {
		score += 30
	} else if result.PlainLyrics != nil && *result.PlainLyrics != "" {
		score += 10
	}
	if duration > 0 && result.Duration > 0 {
		delta := math.Abs(result.Duration - duration)
		if delta <= 3 {
			score += 25
		} else if delta <= 12 {
			score += 12
		} else if delta > 45 {
			score -= 15
		}
	}
	return score
}

func normalizeLyricsMatchText(value string) string {
	value = strings.ToLower(cleanLyricsTitle(value))
	var builder strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == ' ' {
			builder.WriteRune(r)
		} else {
			builder.WriteRune(' ')
		}
	}
	return strings.Join(strings.Fields(builder.String()), " ")
}

func uniqueNonEmpty(values ...string) []string {
	seen := map[string]bool{}
	var result []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func (m *Monitor) finishLyrics(trackKey string, lines []LyricLine, plain, status string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if trackKey != m.lyricsTrackKey {
		return
	}
	m.snapshot.Lyrics = lines
	m.snapshot.PlainLyrics = plain
	m.snapshot.LyricsStatus = status
}

// ParseLRC parses mm:ss.xx timestamped lyrics.
func ParseLRC(input string) []LyricLine {
	var result []LyricLine
	for _, line := range strings.Split(input, "\n") {
		var timestamps []float64
		for strings.HasPrefix(line, "[") {
			end := strings.IndexByte(line, ']')
			if end < 0 {
				break
			}
			timestamp := line[1:end]
			parts := strings.SplitN(timestamp, ":", 2)
			if len(parts) != 2 {
				break
			}
			minutes, errMinutes := strconv.ParseFloat(parts[0], 64)
			seconds, errSeconds := strconv.ParseFloat(parts[1], 64)
			if errMinutes != nil || errSeconds != nil {
				break
			}
			timestamps = append(timestamps, minutes*60+seconds)
			line = line[end+1:]
		}
		text := strings.TrimSpace(line)
		for _, timestamp := range timestamps {
			if text != "" {
				result = append(result, LyricLine{Time: timestamp, Text: text})
			}
		}
	}
	return result
}
