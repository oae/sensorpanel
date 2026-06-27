# Media and Music Modes

SensorPanel can replace the sensor dashboard with a static image, animated GIF,
or now-playing music display. These modes are mutually exclusive.

## Static images

Display a local PNG, JPEG, or GIF:

```bash
sensorpanel run --image /path/to/image.png
```

HTTP and HTTPS URLs are supported:

```bash
sensorpanel run --image https://example.com/cover.jpg
```

Images retain their aspect ratio and are letterboxed with black when their
aspect ratio differs from the panel. Transparent pixels are flattened onto
black because RGB565 panels do not have an alpha channel. GIF files supplied
through `--image` display their first frame.

## Animated GIFs

Play and continuously loop a local or remote GIF:

```bash
sensorpanel run --gif /path/to/animation.gif
sensorpanel run --gif https://example.com/animation.gif
```

Frame delays and GIF disposal behavior are preserved. Frames retain their
aspect ratio, and transparent pixels are flattened onto black to prevent frame
remnants. Remote images and GIFs have a 32 MiB download limit and a 30-second
request timeout.

## Music dashboard

Start the now-playing display:

```bash
sensorpanel run --music
```

The dashboard is designed for a 480×320 display and includes:

- full-screen, heavily blurred cover artwork as the background;
- foreground cover artwork;
- song title, artist, album, player, and playback state;
- elapsed and remaining playback time;
- a stable song-specific waveform with playback progress overlaid;
- synchronized lyrics with automatic scrolling and a large active line.

Lyric changes are intentionally instantaneous rather than animated. This keeps
the active line crisp on low-refresh-rate USB displays.

The waveform is generated deterministically from track metadata. It remains
unchanged for the duration of the song, avoiding unnecessary updates on
low-refresh-rate USB panels. It is a visual progress representation, not an
analysis of protected audio from services such as Spotify.

### Requirements

Music mode currently targets Linux players that expose MPRIS metadata. Install
`playerctl` and verify that it can see the active player:

```bash
playerctl -a status
playerctl -a metadata
```

Spotify, VLC, and many browser-based players support MPRIS. Cover artwork comes
from the player's `mpris:artUrl` metadata. HTTP(S) artwork is loaded directly.
Local `file://` artwork, including extensionless temporary images produced by
browsers, is embedded before rendering so the dashboard can load it reliably.

### Lyrics

SensorPanel requests lyrics from LRCLIB when the track changes. Timed LRC lyrics
are synchronized with the MPRIS playback position and automatically centered on
the active line. If only plain lyrics are available, the dashboard shows a
fallback excerpt. Missing lyrics do not stop playback metadata from updating.

Lyrics require internet access and may take several seconds to appear. Results
depend on the title, artist, and duration reported by the player.

### Lyrics cache

Fetched lyrics are cached under the platform SensorPanel cache directory. On
Linux this is `$XDG_CACHE_HOME/sensorpanel/lyrics`, or
`~/.cache/sensorpanel/lyrics` when `XDG_CACHE_HOME` is unset. Cache filenames
are SHA-256 hashes of normalized title, artist, and duration metadata.

Synchronized and plain lyrics are retained for 90 days. A “not found” result is
retained for 24 hours to avoid repeatedly querying the service for the same
track. Temporary network and server failures are not cached. Running
`sensorpanel prune` clears the cache, including cached lyrics.

### Refresh interval

Music mode defaults to a 0.5-second display interval. The minimum is 0.25
seconds:

```bash
sensorpanel run --music --interval 0.5
```

## Autostart service

Install or update the service for a media mode:

```bash
sensorpanel service install --music
sensorpanel service install --gif https://example.com/animation.gif
sensorpanel service install --image /path/to/wallpaper.png
```

Optional display settings can be persisted in the service:

```bash
sensorpanel service install --music --interval 0.5 --brightness 7
```

Only one of `--music`, `--gif`, and `--image` may be used. Re-running
`service install` replaces the existing service command. Restart it to apply the
change:

```bash
sensorpanel service stop
sensorpanel service start
sensorpanel service status
```

On Linux, inspect the effective command with:

```bash
systemctl --user cat sensorpanel.service
```

## Interaction with themes and sensors

Media modes skip normal sensor collection and the selected web theme. Remove the
media flag to return to the configured theme or built-in sensor dashboard.
