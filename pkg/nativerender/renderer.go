package nativerender

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/oae/sensorpanel/pkg/jpegcodec"
	apppaths "github.com/oae/sensorpanel/pkg/paths"
	bitmap "github.com/oae/sensorpanel/pkg/renderer"
)

// Renderer renders a native theme to RGBA.
type Renderer struct {
	theme  *Theme
	width  int
	height int

	outputWidth    int
	outputHeight   int
	wireAngle      int
	wireOutput     bool
	logicalScratch *image.RGBA

	bg        color.RGBA
	accent    color.RGBA
	accent2   color.RGBA
	accent3   color.RGBA
	text      color.RGBA
	muted     color.RGBA
	panel     color.RGBA
	panelLine color.RGBA

	backgroundPaths    []string
	backgroundJPEG     [][]byte
	backgroundCache    map[int]*image.RGBA
	backgroundOrder    []int
	backgroundFree     []*image.RGBA
	backgroundDecoder  jpegcodec.Decoder
	backgroundCacheMax int
	backgroundFPS      float64
	backgroundOpacity  float64
	backgroundPrefetch int
	backgroundMu       sync.RWMutex
	prefetchRequests   chan int
}

// New creates a native renderer for the target logical display size.
func New(theme *Theme, width, height int) *Renderer {
	r := &Renderer{
		theme:        theme,
		width:        width,
		height:       height,
		outputWidth:  width,
		outputHeight: height,
		bg:           parseColor(theme.Background),
		accent:       parseColor(theme.Accent),
		accent2:      parseColor(theme.Accent2),
		accent3:      parseColor(theme.Accent3),
		text:         parseColor(theme.Text),
		muted:        parseColor(theme.Muted),
		panel:        parseColor(theme.Panel),
		panelLine:    parseColor(theme.PanelLine),
	}
	if theme.BackgroundSequence != nil {
		r.backgroundFPS = theme.BackgroundSequence.FPS
		r.backgroundOpacity = theme.BackgroundSequence.Opacity
	}
	if theme.Performance != nil {
		r.backgroundPrefetch = theme.Performance.PrefetchFrames
	}
	return r
}

// ConfigureWire makes the renderer produce the panel's physical JPEG
// orientation. Background JPEGs are transformed once into the application
// cache and overlays are rotated only when their sensor values change.
func (r *Renderer) ConfigureWire(width, height, clockwiseDegrees int) error {
	clockwiseDegrees = ((clockwiseDegrees % 360) + 360) % 360
	if clockwiseDegrees != 0 && clockwiseDegrees != 90 && clockwiseDegrees != 180 && clockwiseDegrees != 270 {
		return fmt.Errorf("wire rotation must be 0, 90, 180, or 270")
	}
	r.outputWidth = width
	r.outputHeight = height
	r.wireAngle = clockwiseDegrees
	r.wireOutput = true
	return nil
}

func (r *Renderer) OutputWidth() int  { return r.outputWidth }
func (r *Renderer) OutputHeight() int { return r.outputHeight }
func (r *Renderer) WireOutput() bool  { return r.wireOutput }

// LoadBackgroundSequence loads pre-rendered background frames relative to baseDir.
func (r *Renderer) LoadBackgroundSequence(baseDir string) error {
	seq := r.theme.BackgroundSequence
	if seq == nil || seq.Path == "" {
		return nil
	}
	dir := seq.Path
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(baseDir, dir)
	}
	frameCount := seq.Frames
	if frameCount <= 0 {
		matches, err := filepath.Glob(filepath.Join(dir, strings.ReplaceAll(seq.Pattern, "%04d", "*")))
		if err != nil {
			return fmt.Errorf("load background sequence: %w", err)
		}
		frameCount = len(matches)
	}
	paths := make([]string, 0, frameCount)
	for i := 1; i <= frameCount; i++ {
		path := filepath.Join(dir, fmt.Sprintf(seq.Pattern, i))
		if _, err := os.Stat(path); err != nil {
			if seq.Frames <= 0 && os.IsNotExist(err) {
				continue
			}
			return err
		}
		paths = append(paths, path)
	}
	if len(paths) == 0 {
		return fmt.Errorf("background sequence %s has no frames", dir)
	}
	if r.wireOutput {
		transformed, err := r.prepareWireBackgrounds(paths)
		if err != nil {
			r.backgroundJPEG, err = r.transformWireBackgroundsInMemory(paths)
			if err != nil {
				return fmt.Errorf("prepare wire backgrounds: %w", err)
			}
		} else {
			paths = transformed
		}
	}
	r.backgroundPaths = paths
	r.backgroundCacheMax = 2
	r.backgroundMu.Lock()
	r.backgroundCache = make(map[int]*image.RGBA)
	r.backgroundOrder = nil
	r.backgroundFree = nil
	r.backgroundMu.Unlock()
	var first []byte
	if len(r.backgroundJPEG) == len(paths) {
		first = r.backgroundJPEG[0]
	} else {
		first, _ = os.ReadFile(paths[0])
	}
	if len(first) > 0 {
		if config, _, err := image.DecodeConfig(bytes.NewReader(first)); err == nil &&
			config.Width == r.outputWidth && config.Height == r.outputHeight {
			decoder, err := jpegcodec.NewDecoder(jpegcodec.Config{
				Width:   r.outputWidth,
				Height:  r.outputHeight,
				Backend: "auto",
			})
			if err == nil {
				r.backgroundDecoder = decoder
				const maxCompressedCache = 64 << 20
				total := int64(0)
				for _, path := range paths {
					if info, statErr := os.Stat(path); statErr == nil {
						total += info.Size()
					}
				}
				if r.backgroundJPEG == nil && total > 0 && total <= maxCompressedCache {
					r.backgroundJPEG = make([][]byte, len(paths))
					r.backgroundJPEG[0] = first
					for i := 1; i < len(paths); i++ {
						data, readErr := os.ReadFile(paths[i])
						if readErr != nil {
							r.backgroundJPEG = nil
							break
						}
						r.backgroundJPEG[i] = data
					}
				}
			}
		}
	}
	if len(paths) > 1 && r.backgroundDecoder == nil && r.backgroundPrefetch > 0 {
		r.prefetchRequests = make(chan int, 1)
		go r.prefetchLoop()
		r.requestPrefetch(0)
	}
	return nil
}

func (r *Renderer) transformWireBackgroundsInMemory(source []string) ([][]byte, error) {
	result := make([][]byte, len(source))
	for i, input := range source {
		data, err := os.ReadFile(input)
		if err != nil {
			return nil, err
		}
		result[i], err = jpegcodec.RotateJPEG(data, r.wireAngle)
		if err != nil {
			return nil, err
		}
		config, _, err := image.DecodeConfig(bytes.NewReader(result[i]))
		if err != nil || config.Width != r.outputWidth || config.Height != r.outputHeight {
			return nil, fmt.Errorf("rotated frame %s is %dx%d, expected %dx%d",
				input, config.Width, config.Height, r.outputWidth, r.outputHeight)
		}
	}
	return result, nil
}

func (r *Renderer) prepareWireBackgrounds(source []string) ([]string, error) {
	cacheRoot, err := apppaths.CacheDir()
	if err != nil {
		return nil, err
	}
	hash := sha256.New()
	_, _ = fmt.Fprintf(hash, "wire-v1:%dx%d:%d", r.outputWidth, r.outputHeight, r.wireAngle)
	for _, path := range source {
		info, statErr := os.Stat(path)
		if statErr != nil {
			return nil, statErr
		}
		_, _ = fmt.Fprintf(hash, "\x00%s:%d:%d", path, info.Size(), info.ModTime().UnixNano())
	}
	key := hex.EncodeToString(hash.Sum(nil))[:20]
	dir := filepath.Join(cacheRoot, "native-background", key)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	result := make([]string, len(source))
	for i, input := range source {
		output := filepath.Join(dir, fmt.Sprintf("frame_%06d.jpg", i))
		if validJPEGSize(output, r.outputWidth, r.outputHeight) {
			result[i] = output
			continue
		}
		data, err := os.ReadFile(input)
		if err != nil {
			return nil, err
		}
		transformed, err := jpegcodec.RotateJPEG(data, r.wireAngle)
		if err != nil {
			return nil, fmt.Errorf("rotate %s: %w", input, err)
		}
		config, _, err := image.DecodeConfig(bytes.NewReader(transformed))
		if err != nil || config.Width != r.outputWidth || config.Height != r.outputHeight {
			return nil, fmt.Errorf("rotated frame %s is %dx%d, expected %dx%d",
				input, config.Width, config.Height, r.outputWidth, r.outputHeight)
		}
		temporary := output + ".tmp"
		if err := os.WriteFile(temporary, transformed, 0o644); err != nil {
			return nil, err
		}
		if err := os.Rename(temporary, output); err != nil {
			return nil, err
		}
		result[i] = output
	}
	return result, nil
}

func validJPEGSize(path string, width, height int) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	config, _, err := image.DecodeConfig(file)
	return err == nil && config.Width == width && config.Height == height
}

// Close releases codec and prefetch resources owned by the renderer.
func (r *Renderer) Close() error {
	if r.prefetchRequests != nil {
		close(r.prefetchRequests)
		r.prefetchRequests = nil
	}
	if r.backgroundDecoder != nil {
		err := r.backgroundDecoder.Close()
		r.backgroundDecoder = nil
		return err
	}
	return nil
}

// BackgroundDecoder returns the effective decoder backend.
func (r *Renderer) BackgroundDecoder() string {
	if r.backgroundDecoder == nil {
		return "image"
	}
	return r.backgroundDecoder.Name()
}

// Render draws the current frame.
func (r *Renderer) Render(data map[string]interface{}, now time.Time) *image.RGBA {
	img := r.RenderBackground(now)
	overlay := r.RenderOverlay(data, now)
	r.Composite(img, overlay)
	return img
}

// RenderBackground draws only the animated/static background layer.
func (r *Renderer) RenderBackground(now time.Time) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, r.outputWidth, r.outputHeight))
	r.RenderBackgroundInto(img, now)
	return img
}

// RenderBackgroundInto renders into a caller-owned buffer so animated themes
// can reuse their canvas instead of allocating several megabytes every frame.
func (r *Renderer) RenderBackgroundInto(img *image.RGBA, now time.Time) {
	if img.Bounds().Dx() != r.outputWidth || img.Bounds().Dy() != r.outputHeight {
		return
	}
	if len(r.backgroundPaths) == 0 || r.backgroundFPS <= 0 {
		draw.Draw(img, img.Bounds(), &image.Uniform{r.bg}, image.Point{}, draw.Src)
		return
	}
	r.drawBackgroundSequence(img, now)
}

// RenderOutputOverlayInto produces an overlay in the configured output
// orientation. Wire rotation happens only when this method is called.
func (r *Renderer) RenderOutputOverlayInto(img *image.RGBA, data map[string]interface{}, now time.Time) {
	if !r.wireOutput {
		r.RenderOverlayInto(img, data, now)
		return
	}
	if r.logicalScratch == nil {
		r.logicalScratch = image.NewRGBA(image.Rect(0, 0, r.width, r.height))
	}
	r.RenderOverlayInto(r.logicalScratch, data, now)
	rotateRGBAInto(img, r.logicalScratch, r.wireAngle)
}

// RenderOverlay draws only the sensor overlay layer.
func (r *Renderer) RenderOverlay(data map[string]interface{}, now time.Time) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, r.width, r.height))
	r.RenderOverlayInto(img, data, now)
	return img
}

// RenderOverlayInto draws into a caller-owned transparent canvas.
func (r *Renderer) RenderOverlayInto(img *image.RGBA, data map[string]interface{}, now time.Time) {
	clear(img.Pix)
	switch r.theme.Layout {
	case "trofeo_vertical_v1":
		r.drawTrofeoVertical(img, data, now)
	default:
		r.textAt(img, 24, 24, 3, "Unsupported native layout", r.text)
		r.textAt(img, 24, 58, 2, r.theme.Layout, r.muted)
	}
}

// RenderLowPower draws a static AMOLED-black, monochrome dashboard. It is
// intended for idle mode and is regenerated only when sensor data changes.
func (r *Renderer) RenderLowPower(data map[string]interface{}, now time.Time) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, r.outputWidth, r.outputHeight))
	r.RenderLowPowerInto(img, data, now)
	return img
}

// RenderLowPowerInto renders the layout directly with a monochrome palette,
// avoiding a color overlay allocation and a second full-screen grayscale pass.
func (r *Renderer) RenderLowPowerInto(img *image.RGBA, data map[string]interface{}, now time.Time) {
	if r.wireOutput {
		if r.logicalScratch == nil {
			r.logicalScratch = image.NewRGBA(image.Rect(0, 0, r.width, r.height))
		}
		r.renderLowPowerLogical(r.logicalScratch, data, now)
		rotateRGBAInto(img, r.logicalScratch, r.wireAngle)
		return
	}
	r.renderLowPowerLogical(img, data, now)
}

func (r *Renderer) renderLowPowerLogical(img *image.RGBA, data map[string]interface{}, now time.Time) {
	draw.Draw(img, img.Bounds(), image.Black, image.Point{}, draw.Src)
	accent, accent2, accent3, text, muted, panel, panelLine :=
		r.accent, r.accent2, r.accent3, r.text, r.muted, r.panel, r.panelLine
	r.accent = color.RGBA{0xd8, 0xd8, 0xd8, 0xff}
	r.accent2 = color.RGBA{0xb8, 0xb8, 0xb8, 0xff}
	r.accent3 = color.RGBA{0xc8, 0xc8, 0xc8, 0xff}
	r.text = color.RGBA{0xf2, 0xf2, 0xf2, 0xff}
	r.muted = color.RGBA{0x91, 0x91, 0x91, 0xff}
	r.panel = color.RGBA{0x08, 0x08, 0x08, 0xd8}
	r.panelLine = color.RGBA{0x58, 0x58, 0x58, 0x88}
	switch r.theme.Layout {
	case "trofeo_vertical_v1":
		r.drawTrofeoVertical(img, data, now)
	default:
		r.textAt(img, 24, 24, 3, "Unsupported native layout", r.text)
	}
	r.accent, r.accent2, r.accent3, r.text, r.muted, r.panel, r.panelLine =
		accent, accent2, accent3, text, muted, panel, panelLine
}

// ViewSignature contains exactly the values rendered by the Trofeo layout,
// rounded to their displayed precision. It lets idle mode skip unchanged
// frames even when raw sensor readings jitter.
func (r *Renderer) ViewSignature(data map[string]interface{}, now time.Time) string {
	cpu := nested(data, "cpu")
	board := nested(data, "motherboard")
	gpu := nested(data, "nvidia_gpu")
	if len(gpu) == 0 {
		gpu = nested(data, "amd_gpu")
	}
	mem := nested(data, "memory")
	disk := firstItem(nested(data, "disk"), "disks", "_items")
	net := firstItem(nested(data, "network"), "interfaces", "_items")
	cpuFan := valueAny(cpu, "fan_speed", "fan_rpm")
	if cpuFan == 0 {
		cpuFan = valueAny(board, "cpu_fan", "system_fan1")
	}
	values := []string{
		now.Format("2006-01-02 15:04"),
		stringAny(cpu, "name", "model"),
		stringAny(gpu, "name", "model"),
		strconv.FormatFloat(valueAny(cpu, "temperature", "temperature_c"), 'f', 0, 64),
		strconv.FormatFloat(valueAny(cpu, "load", "load_percent"), 'f', 0, 64),
		formatClock(valueAny(cpu, "frequency", "frequency_mhz")),
		formatRPM(cpuFan),
		strconv.FormatFloat(valueAny(gpu, "temperature", "temperature_c"), 'f', 0, 64),
		strconv.FormatFloat(valueAny(gpu, "load", "load_percent"), 'f', 0, 64),
		formatGB(valueAny(gpu, "memory_used", "memory_used_mb") * 1024 * 1024),
		strconv.FormatFloat(valueAny(gpu, "power", "power_watts"), 'f', 0, 64),
		strconv.FormatFloat(valueAny(mem, "percent"), 'f', 0, 64),
		strconv.FormatFloat(valueAny(mem, "used", "used_mb")/1024, 'f', 1, 64),
		strconv.FormatFloat(valueAny(mem, "total", "total_mb")/1024, 'f', 0, 64),
		stringAny(disk, "mount_point", "mount", "name", "label"),
		strconv.FormatFloat(valueAny(disk, "percent", "used_percent"), 'f', 0, 64),
		formatBPS(valueAny(net, "rx_bytes_per_sec", "rx_rate")),
		formatBPS(valueAny(net, "tx_bytes_per_sec", "tx_rate")),
	}
	for _, key := range []string{"dimm1_temp", "dimm2_temp", "dimm3_temp", "dimm4_temp"} {
		values = append(values, strconv.FormatFloat(valueAny(board, key), 'f', 0, 64))
	}
	return strings.Join(values, "\x00")
}

// PreferredFPS returns the renderer animation cadence.
func (r *Renderer) PreferredFPS() float64 {
	if len(r.backgroundPaths) > 1 && r.backgroundFPS > 0 {
		return r.backgroundFPS
	}
	return 0
}

// BackgroundFrameCount returns the number of configured animated background frames.
func (r *Renderer) BackgroundFrameCount() int {
	return len(r.backgroundPaths)
}

func (r *Renderer) drawBackgroundSequence(img *image.RGBA, now time.Time) {
	if len(r.backgroundPaths) == 0 || r.backgroundFPS <= 0 {
		return
	}
	idx := int(math.Floor(float64(now.UnixMilli())/1000*r.backgroundFPS)) % len(r.backgroundPaths)
	r.requestPrefetch(idx)
	opacity := clamp(r.backgroundOpacity, 0, 1)
	if r.backgroundDecoder != nil && opacity >= 1 {
		data, err := r.backgroundData(idx)
		if err == nil {
			err = r.backgroundDecoder.DecodeInto(data, img)
		}
		if err != nil {
			draw.Draw(img, img.Bounds(), &image.Uniform{r.bg}, image.Point{}, draw.Src)
		}
		return
	}
	frame, err := r.backgroundFrame(idx)
	if err != nil {
		return
	}
	// The canvas starts black, so opacity is a direct brightness multiplier.
	// Sensor panels provide their own dark backing; don't apply a second full
	// screen black veil here or the video becomes invisible behind tinted glass.
	if opacity >= 1 {
		for y := 0; y < r.outputHeight; y++ {
			src, dst := y*frame.Stride, y*img.Stride
			copy(img.Pix[dst:dst+r.outputWidth*4], frame.Pix[src:src+r.outputWidth*4])
		}
		return
	}
	factor := uint16(opacity * 256)
	for y := 0; y < r.outputHeight; y++ {
		src, dst := y*frame.Stride, y*img.Stride
		for x := 0; x < r.outputWidth; x++ {
			img.Pix[dst] = uint8(uint16(frame.Pix[src]) * factor >> 8)
			img.Pix[dst+1] = uint8(uint16(frame.Pix[src+1]) * factor >> 8)
			img.Pix[dst+2] = uint8(uint16(frame.Pix[src+2]) * factor >> 8)
			img.Pix[dst+3] = 255
			src, dst = src+4, dst+4
		}
	}
}

func (r *Renderer) backgroundData(idx int) ([]byte, error) {
	if len(r.backgroundJPEG) == len(r.backgroundPaths) {
		return r.backgroundJPEG[idx], nil
	}
	return os.ReadFile(r.backgroundPaths[idx])
}

func (r *Renderer) backgroundFrame(idx int) (*image.RGBA, error) {
	r.backgroundMu.RLock()
	cached := r.backgroundCache[idx]
	r.backgroundMu.RUnlock()
	if cached != nil {
		return cached, nil
	}
	var frame *image.RGBA
	r.backgroundMu.Lock()
	if count := len(r.backgroundFree); count > 0 {
		frame = r.backgroundFree[count-1]
		r.backgroundFree = r.backgroundFree[:count-1]
	}
	r.backgroundMu.Unlock()
	if frame == nil {
		frame = image.NewRGBA(image.Rect(0, 0, r.outputWidth, r.outputHeight))
	}
	var err error
	if r.backgroundDecoder != nil {
		var data []byte
		data, err = r.backgroundData(idx)
		if err == nil {
			err = r.backgroundDecoder.DecodeInto(data, frame)
		}
	} else {
		frame, err = loadRGBA(r.backgroundPaths[idx], r.outputWidth, r.outputHeight)
	}
	if err != nil {
		r.backgroundMu.Lock()
		r.backgroundFree = append(r.backgroundFree, frame)
		r.backgroundMu.Unlock()
		return nil, err
	}
	r.cacheBackgroundFrame(idx, frame)
	return frame, nil
}

func (r *Renderer) requestPrefetch(idx int) {
	if r.prefetchRequests == nil {
		return
	}
	select {
	case r.prefetchRequests <- idx:
	default:
	}
}

func (r *Renderer) prefetchLoop() {
	for idx := range r.prefetchRequests {
		for offset := 1; offset <= r.backgroundPrefetch; offset++ {
			next := (idx + offset) % len(r.backgroundPaths)
			r.backgroundMu.RLock()
			cached := r.backgroundCache[next] != nil
			r.backgroundMu.RUnlock()
			if cached {
				continue
			}
			frame, err := loadRGBA(r.backgroundPaths[next], r.outputWidth, r.outputHeight)
			if err == nil {
				r.cacheBackgroundFrame(next, frame)
			}
		}
	}
}

func (r *Renderer) cacheBackgroundFrame(idx int, frame *image.RGBA) {
	r.backgroundMu.Lock()
	defer r.backgroundMu.Unlock()
	if r.backgroundCache == nil || r.backgroundCache[idx] != nil {
		return
	}
	r.backgroundCache[idx] = frame
	r.backgroundOrder = append(r.backgroundOrder, idx)
	maxFrames := r.backgroundCacheMax
	if maxFrames <= 0 {
		maxFrames = r.backgroundPrefetch + 2
		if maxFrames < 4 {
			maxFrames = 4
		}
	}
	for len(r.backgroundOrder) > maxFrames {
		evict := r.backgroundOrder[0]
		r.backgroundOrder = r.backgroundOrder[1:]
		r.backgroundFree = append(r.backgroundFree, r.backgroundCache[evict])
		delete(r.backgroundCache, evict)
	}
}

func loadRGBA(path string, width, height int) (*image.RGBA, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	src, _, err := image.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("decode background frame %s: %w", path, err)
	}
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	sb := src.Bounds()
	if sb.Dx() == width && sb.Dy() == height {
		draw.Draw(dst, dst.Bounds(), src, sb.Min, draw.Src)
		return dst, nil
	}
	scaleCover(dst, src)
	return dst, nil
}

func scaleCover(dst *image.RGBA, src image.Image) {
	db := dst.Bounds()
	sb := src.Bounds()
	dw, dh := db.Dx(), db.Dy()
	sw, sh := sb.Dx(), sb.Dy()
	if sw <= 0 || sh <= 0 || dw <= 0 || dh <= 0 {
		return
	}
	scale := math.Max(float64(dw)/float64(sw), float64(dh)/float64(sh))
	scaledW := int(math.Ceil(float64(sw) * scale))
	scaledH := int(math.Ceil(float64(sh) * scale))
	offsetX := (scaledW - dw) / 2
	offsetY := (scaledH - dh) / 2
	for y := 0; y < dh; y++ {
		sy := sb.Min.Y + int(float64(y+offsetY)/scale)
		if sy >= sb.Max.Y {
			sy = sb.Max.Y - 1
		}
		for x := 0; x < dw; x++ {
			sx := sb.Min.X + int(float64(x+offsetX)/scale)
			if sx >= sb.Max.X {
				sx = sb.Max.X - 1
			}
			dst.Set(x, y, src.At(sx, sy))
		}
	}
}

func rotateRGBAInto(dst, src *image.RGBA, degrees int) {
	degrees = ((degrees % 360) + 360) % 360
	width, height := src.Bounds().Dx(), src.Bounds().Dy()
	expectedWidth, expectedHeight := width, height
	if degrees == 90 || degrees == 270 {
		expectedWidth, expectedHeight = height, width
	}
	if dst.Bounds().Dx() != expectedWidth || dst.Bounds().Dy() != expectedHeight {
		return
	}
	if degrees == 0 {
		for y := 0; y < height; y++ {
			copy(dst.Pix[y*dst.Stride:y*dst.Stride+width*4], src.Pix[y*src.Stride:y*src.Stride+width*4])
		}
		return
	}
	for dy := 0; dy < expectedHeight; dy++ {
		dstOffset := dy * dst.Stride
		for dx := 0; dx < expectedWidth; dx++ {
			var sx, sy int
			switch degrees {
			case 90:
				sx, sy = dy, height-1-dx
			case 180:
				sx, sy = width-1-dx, height-1-dy
			case 270:
				sx, sy = width-1-dy, dx
			}
			srcOffset := sy*src.Stride + sx*4
			copy(dst.Pix[dstOffset:dstOffset+4], src.Pix[srcOffset:srcOffset+4])
			dstOffset += 4
		}
	}
}

func (r *Renderer) drawTrofeoVertical(img *image.RGBA, data map[string]interface{}, now time.Time) {
	host, _ := os.Hostname()
	if host == "" {
		host = "sensorpanel"
	}
	cpu := nested(data, "cpu")
	board := nested(data, "motherboard")
	gpu := nested(data, "nvidia_gpu")
	if len(gpu) == 0 {
		gpu = nested(data, "amd_gpu")
	}
	mem := nested(data, "memory")
	disk := firstItem(nested(data, "disk"), "disks", "_items")
	net := firstItem(nested(data, "network"), "interfaces", "_items")

	pad := 18
	y := pad
	r.textAt(img, pad, y, 3, strings.ToUpper(short(host, 13)), r.text)
	r.textRight(img, r.width-pad, y, 4, now.Format("15:04"), r.text)
	y += 58
	r.textAt(img, pad, y, 2, strings.ToUpper(now.Format("Mon, Jan 2")), r.muted)
	y += 58

	r.panelBox(img, pad, y, r.width-pad*2, 420)
	cpuTemp := valueAny(cpu, "temperature", "temperature_c")
	gpuTemp := valueAny(gpu, "temperature", "temperature_c")
	cpuLoad := valueAny(cpu, "load", "load_percent")
	gpuLoad := valueAny(gpu, "load", "load_percent")
	r.gauge(img, pad+38, y+62, 150, cpuLoad, r.accent)
	r.gauge(img, pad+238, y+62, 150, gpuLoad, r.accent2)
	r.centerText(img, pad+113, y+132, 4, fmt.Sprintf("%.0fC", cpuTemp), r.text)
	r.centerText(img, pad+313, y+132, 4, fmt.Sprintf("%.0fC", gpuTemp), r.text)
	r.centerText(img, pad+113, y+205, 2, fmt.Sprintf("CPU %.0f%%", cpuLoad), r.muted)
	r.centerText(img, pad+313, y+205, 2, fmt.Sprintf("GPU %.0f%%", gpuLoad), r.muted)
	r.textAt(img, pad+24, y+292, 2, short(stringAny(cpu, "name", "model"), 30), r.muted)
	r.textAt(img, pad+24, y+334, 2, short(stringAny(gpu, "name", "model"), 30), r.muted)
	y += 444

	r.panelBox(img, pad, y, r.width-pad*2, 620)
	r.textAt(img, pad+24, y+24, 3, "CPU", r.accent)
	r.metricBarWide(img, pad+24, y+76, r.width-pad*2-48, "LOAD", cpuLoad, fmt.Sprintf("%.0f%%", cpuLoad), r.accent)
	r.metricBarWide(img, pad+24, y+156, r.width-pad*2-48, "CLOCK", percent(valueAny(cpu, "frequency", "frequency_mhz"), 6000), formatClock(valueAny(cpu, "frequency", "frequency_mhz")), r.accent)
	cpuFan := valueAny(cpu, "fan_speed", "fan_rpm")
	if cpuFan == 0 {
		cpuFan = valueAny(board, "cpu_fan", "system_fan1")
	}
	r.metricBarWide(img, pad+24, y+236, r.width-pad*2-48, "FAN", percent(cpuFan, 6000), formatRPM(cpuFan), r.accent)

	r.textAt(img, pad+24, y+326, 3, "GPU", r.accent2)
	r.metricBarWide(img, pad+24, y+378, r.width-pad*2-48, "LOAD", gpuLoad, fmt.Sprintf("%.0f%%", gpuLoad), r.accent2)
	r.metricBarWide(img, pad+24, y+458, r.width-pad*2-48, "VRAM", percent(valueAny(gpu, "memory_used", "memory_used_mb"), valueAny(gpu, "memory_total", "memory_total_mb")), formatGB(valueAny(gpu, "memory_used", "memory_used_mb")*1024*1024), r.accent2)
	r.metricBarWide(img, pad+24, y+538, r.width-pad*2-48, "POWER", percent(valueAny(gpu, "power", "power_watts"), 650), fmt.Sprintf("%.0fW", valueAny(gpu, "power", "power_watts")), r.accent2)
	y += 644

	r.panelBox(img, pad, y, r.width-pad*2, 380)
	memPercent := valueAny(mem, "percent")
	r.textAt(img, pad+24, y+24, 3, "MEMORY", r.accent)
	r.textAt(img, pad+24, y+86, 8, fmt.Sprintf("%.0f%%", memPercent), r.text)
	r.textAt(img, pad+24, y+176, 3, fmt.Sprintf("%.1f / %.0f GB", valueAny(mem, "used", "used_mb")/1024, valueAny(mem, "total", "total_mb")/1024), r.muted)
	r.bar(img, pad+24, y+238, r.width-pad*2-48, 22, memPercent, r.accent)
	r.memorySockets(img, pad+24, y+280, r.width-pad*2-48, board)
	y += 404

	r.panelBox(img, pad, y, r.width-pad*2, r.height-y-pad)
	diskName := stringAny(disk, "mount_point", "mount", "name", "label")
	if diskName == "" {
		diskName = "DISK"
	}
	diskPercent := valueAny(disk, "percent", "used_percent")
	r.textAt(img, pad+24, y+24, 3, "STORAGE / NETWORK", r.accent2)
	r.textAt(img, pad+24, y+74, 2, short(diskName, 14), r.text)
	r.textRight(img, r.width-pad-24, y+66, 3, fmt.Sprintf("%.0f%%", diskPercent), r.text)
	r.bar(img, pad+24, y+122, r.width-pad*2-48, 22, diskPercent, r.accent3)
	r.textAt(img, pad+24, y+180, 2, "DOWN", r.muted)
	r.textRight(img, r.width-pad-24, y+174, 3, formatBPS(valueAny(net, "rx_bytes_per_sec", "rx_rate")), r.text)
	r.textAt(img, pad+24, y+236, 2, "UP", r.muted)
	r.textRight(img, r.width-pad-24, y+230, 3, formatBPS(valueAny(net, "tx_bytes_per_sec", "tx_rate")), r.text)
}

func (r *Renderer) panelBox(img *image.RGBA, x, y, w, h int) {
	r.rect(img, x, y, w, h, r.panel)
	r.rect(img, x, y, w, 2, r.panelLine)
	r.rect(img, x, y+h-2, w, 2, withAlpha(r.panelLine, 80))
	r.rect(img, x, y, 2, h, withAlpha(r.panelLine, 80))
	r.rect(img, x+w-2, y, 2, h, withAlpha(r.panelLine, 80))
}

func (r *Renderer) metricBar(img *image.RGBA, x, y int, label string, pct float64, value string, c color.RGBA) {
	r.textAt(img, x, y, 2, label, r.muted)
	r.textRight(img, x+176, y-4, 3, value, r.text)
	r.bar(img, x, y+42, 180, 18, pct, c)
}

func (r *Renderer) metricBarWide(img *image.RGBA, x, y, w int, label string, pct float64, value string, c color.RGBA) {
	r.textAt(img, x, y, 2, label, r.muted)
	r.textRight(img, x+w, y-4, 3, value, r.text)
	r.bar(img, x, y+42, w, 18, pct, c)
}

func (r *Renderer) memorySockets(img *image.RGBA, x, y, w int, board map[string]interface{}) {
	labels := []string{"A1", "A2", "B1", "B2"}
	keys := []string{"dimm1_temp", "dimm2_temp", "dimm3_temp", "dimm4_temp"}
	cellW := (w - 16) / 2
	for i, label := range labels {
		col := i % 2
		row := i / 2
		cx := x + col*(cellW+16)
		cy := y + row*38
		r.rect(img, cx, cy, cellW, 32, withAlpha(r.text, 28))
		temp := valueAny(board, keys[i])
		value := "--"
		if temp > 0 {
			value = fmt.Sprintf("%.0fC", temp)
		}
		r.textAt(img, cx+10, cy+8, 2, label, r.accent)
		r.textRight(img, cx+cellW-10, cy+8, 2, value, r.text)
	}
}

func (r *Renderer) gauge(img *image.RGBA, x, y, size int, pct float64, c color.RGBA) {
	cx := x + size/2
	cy := y + size/2
	outer := float64(size) / 2
	inner := outer - 14
	start := -220.0 * math.Pi / 180
	end := 40.0 * math.Pi / 180
	pct = clamp(pct, 0, 100)
	limit := start + (end-start)*pct/100
	for py := y; py < y+size; py++ {
		for px := x; px < x+size; px++ {
			dx := float64(px - cx)
			dy := float64(py - cy)
			dist := math.Hypot(dx, dy)
			if dist < inner || dist > outer {
				continue
			}
			a := math.Atan2(dy, dx)
			if a < start {
				a += 2 * math.Pi
			}
			normEnd := end
			if normEnd < start {
				normEnd += 2 * math.Pi
			}
			if a >= start && a <= normEnd {
				if a <= limit {
					img.SetRGBA(px, py, c)
				} else {
					img.SetRGBA(px, py, withAlpha(r.text, 35))
				}
			}
		}
	}
}

func (r *Renderer) bar(img *image.RGBA, x, y, w, h int, pct float64, c color.RGBA) {
	r.rect(img, x, y, w, h, withAlpha(r.text, 34))
	fill := int(float64(w) * clamp(pct, 0, 100) / 100)
	if fill > 0 {
		r.rect(img, x, y, fill, h, c)
	}
}

func (r *Renderer) textAt(img *image.RGBA, x, y, scale int, text string, c color.RGBA) {
	bitmap.DrawBitmapText(img, x, y, scale, ascii(text), c)
}

func (r *Renderer) centerText(img *image.RGBA, cx, y, scale int, text string, c color.RGBA) {
	w, _ := bitmap.MeasureBitmapText(ascii(text), scale)
	r.textAt(img, cx-w/2, y, scale, text, c)
}

func (r *Renderer) textRight(img *image.RGBA, x, y, scale int, text string, c color.RGBA) {
	w, _ := bitmap.MeasureBitmapText(ascii(text), scale)
	r.textAt(img, x-w, y, scale, text, c)
}

func (r *Renderer) rect(img *image.RGBA, x, y, w, h int, c color.RGBA) {
	if w <= 0 || h <= 0 {
		return
	}
	bounds := img.Bounds()
	x0 := max(x, bounds.Min.X)
	y0 := max(y, bounds.Min.Y)
	x1 := min(x+w, bounds.Max.X)
	y1 := min(y+h, bounds.Max.Y)
	for py := y0; py < y1; py++ {
		for px := x0; px < x1; px++ {
			blendPixel(img, px, py, c)
		}
	}
}

func (r *Renderer) line(img *image.RGBA, x0, y0, x1, y1 int, c color.RGBA, thickness int) {
	dx := math.Abs(float64(x1 - x0))
	dy := -math.Abs(float64(y1 - y0))
	sx := -1
	if x0 < x1 {
		sx = 1
	}
	sy := -1
	if y0 < y1 {
		sy = 1
	}
	err := dx + dy
	for {
		r.rect(img, x0-thickness/2, y0-thickness/2, thickness, thickness, c)
		if x0 == x1 && y0 == y1 {
			break
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
}

func nested(data map[string]interface{}, key string) map[string]interface{} {
	m, _ := data[key].(map[string]interface{})
	return m
}

func firstItem(data map[string]interface{}, keys ...string) map[string]interface{} {
	for _, key := range keys {
		if items, ok := data[key].([]map[string]interface{}); ok && len(items) > 0 {
			return chooseBestItem(items)
		}
		if items, ok := data[key].([]interface{}); ok && len(items) > 0 {
			maps := make([]map[string]interface{}, 0, len(items))
			for _, item := range items {
				if m, ok := item.(map[string]interface{}); ok {
					maps = append(maps, m)
				}
			}
			if len(maps) > 0 {
				return chooseBestItem(maps)
			}
		}
	}
	return map[string]interface{}{}
}

func chooseBestItem(items []map[string]interface{}) map[string]interface{} {
	best := items[0]
	bestScore := itemScore(best)
	for _, item := range items[1:] {
		if score := itemScore(item); score > bestScore {
			best = item
			bestScore = score
		}
	}
	return best
}

func itemScore(item map[string]interface{}) float64 {
	return valueAny(item, "percent", "used_percent") +
		valueAny(item, "rx_bytes_per_sec", "rx_rate") +
		valueAny(item, "tx_bytes_per_sec", "tx_rate")
}

func valueAny(m map[string]interface{}, keys ...string) float64 {
	for _, key := range keys {
		if v, ok := m[key]; ok {
			switch n := v.(type) {
			case float64:
				return n
			case float32:
				return float64(n)
			case int:
				return float64(n)
			case int64:
				return float64(n)
			case uint64:
				return float64(n)
			case *float64:
				if n != nil {
					return *n
				}
			}
		}
	}
	return 0
}

func stringAny(m map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if s, ok := m[key].(string); ok {
			return s
		}
	}
	return ""
}

func formatClock(mhz float64) string {
	if mhz <= 0 {
		return "--"
	}
	if mhz >= 1000 {
		return fmt.Sprintf("%.2fG", mhz/1000)
	}
	return fmt.Sprintf("%.0fM", mhz)
}

func formatRPM(rpm float64) string {
	if rpm <= 0 {
		return "--"
	}
	return fmt.Sprintf("%.0fRPM", rpm)
}

func formatGB(bytes float64) string {
	if bytes <= 0 {
		return "--"
	}
	return fmt.Sprintf("%.1fGB", bytes/1024/1024/1024)
}

func formatBPS(bytes float64) string {
	units := []string{"B/s", "KB/s", "MB/s", "GB/s"}
	for _, unit := range units {
		if bytes < 1024 {
			return fmt.Sprintf("%.1f%s", bytes, unit)
		}
		bytes /= 1024
	}
	return fmt.Sprintf("%.1fTB/s", bytes)
}

func percent(value, maxValue float64) float64 {
	if maxValue <= 0 {
		return 0
	}
	return value / maxValue * 100
}

func short(s string, n int) string {
	s = ascii(s)
	if len(s) <= n {
		return s
	}
	if n <= 3 {
		return s[:n]
	}
	return s[:n-3] + "..."
}

func ascii(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 32 && r <= 126 {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func withAlpha(c color.RGBA, a uint8) color.RGBA {
	c.A = a
	return c
}

func blendPixel(img *image.RGBA, x, y int, src color.RGBA) {
	if src.A == 255 {
		img.SetRGBA(x, y, src)
		return
	}
	if src.A == 0 {
		return
	}
	dst := img.RGBAAt(x, y)
	sa := uint32(src.A)
	da := uint32(dst.A)
	outA := sa + da*(255-sa)/255
	if outA == 0 {
		img.SetRGBA(x, y, color.RGBA{})
		return
	}
	img.SetRGBA(x, y, color.RGBA{
		R: uint8((uint32(src.R)*sa + uint32(dst.R)*da*(255-sa)/255) / outA),
		G: uint8((uint32(src.G)*sa + uint32(dst.G)*da*(255-sa)/255) / outA),
		B: uint8((uint32(src.B)*sa + uint32(dst.B)*da*(255-sa)/255) / outA),
		A: uint8(outA),
	})
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
