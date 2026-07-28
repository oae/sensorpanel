# Native renderer performance

The Trofeo native renderer is designed around the LY panel's actual protocol:
each update is a complete JPEG delivered as 512-byte records in 4096-byte USB
writes. The firmware does not expose rectangular framebuffer updates, so the
host must reduce work before the final full-frame transfer.

## Pipeline

At theme load, SensorPanel rotates source JPEGs into the panel's physical
orientation and stores them in the application cache. TurboJPEG builds use a
lossless DCT transform; the portable fallback performs a one-time high-quality
re-encode. The cache key includes
source path, size, modification time, output dimensions, rotation, and cache
format version. Changing the video invalidates the cache automatically.

During active playback:

1. A persistent TurboJPEG decoder writes the selected background directly into
   a reusable physical RGBA canvas.
2. The sensor overlay is rendered and rotated only when its formatted values
   change, normally once per second.
3. The cached overlay is composited by the optimized native blender.
4. A persistent TurboJPEG encoder writes into a preallocated output buffer.
5. The LY packetizer fills a reusable packet buffer and sends protocol-defined
   4096-byte transfers using one frame deadline.

The scheduler uses monotonic deadlines. If work misses a deadline it drops the
obsolete animation position instead of building a queue.

In idle mode the video decoder is not used. The renderer redraws directly with
a monochrome palette only after a visible value/minute change, then resends the
cached pixels at 1 FPS to keep the panel firmware from restoring its splash
screen.

## Measuring the physical device

Stop any service that owns the USB panel, then run:

```bash
sensorpanel benchmark --native-theme trofeo --orientation 90 \
  --mode active --duration 30s

sensorpanel benchmark --native-theme trofeo --orientation 90 \
  --mode idle --duration 60s
```

Use `--json` for machine-readable results. CPU is reported as a percentage of
one logical core using process user+system CPU time, not lifetime `ps` average.
The stage table separates sensor, overlay, background decode, composite, JPEG
encode, packetization, USB write, and ACK latency.

For profiling:

```bash
sensorpanel benchmark --native-theme trofeo --orientation 90 \
  --duration 60s --cpu-profile /tmp/sensorpanel.cpu \
  --heap-profile /tmp/sensorpanel.heap

go tool pprof /path/to/sensorpanel /tmp/sensorpanel.cpu
```

Run a benchmark twice after replacing the source animation. The first run may
include one-time wire-cache creation; subsequent runs represent steady state.

Reference measurements on the development Trofeo panel (1920×462, portrait
theme, JPEG quality 68) were:

| Mode | Before | Optimized | Delivered FPS |
|---|---:|---:|---:|
| Active | 34.8% CPU | 9.44% CPU | 24.00 |
| Idle | 4.43% CPU | 0.45% CPU | 1.00 |

These percentages represent one logical CPU core. Exact values vary by CPU,
animation complexity, JPEG quality, and USB controller.

## Build requirements

Linux packages and release builds should include `libjpeg-turbo` and use:

```bash
go build -tags turbojpeg .
```

The portable standard-library codec remains available for other platforms and
for explicit `jpeg_encoder: "stdlib"` themes, but consumes more CPU and may
allocate while decoding.
