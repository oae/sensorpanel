// Package browser - headless rendering using Chrome DevTools Protocol.
package browser

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
)

// Renderer manages a headless Chrome instance for rendering themes.
type Renderer struct {
	mu           sync.Mutex
	width        int
	height       int
	chromePath   string
	allocCtx     context.Context
	allocCancel  context.CancelFunc
	ctx          context.Context
	cancel       context.CancelFunc
	server       *http.Server
	listener     net.Listener
	themeDistDir string
}

// NewRenderer creates a new headless browser renderer.
func NewRenderer(width, height int) (*Renderer, error) {
	chromePath, err := GetChromePath()
	if err != nil {
		return nil, fmt.Errorf("Chrome not found: %w (run 'sensorpanel theme install-browser' to download)", err)
	}

	return &Renderer{
		width:      width,
		height:     height,
		chromePath: chromePath,
	}, nil
}

// Start starts the headless browser and serves the theme.
func (r *Renderer) Start(ctx context.Context, themeDistDir string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.ctx != nil {
		return fmt.Errorf("renderer already started")
	}

	r.themeDistDir = themeDistDir

	// Start HTTP server for theme files
	if err := r.startServer(); err != nil {
		return fmt.Errorf("failed to start theme server: %w", err)
	}

	// Start Chrome with custom executable
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(r.chromePath),
		chromedp.WindowSize(r.width, r.height),
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("disable-background-networking", true),
		chromedp.Flag("disable-sync", true),
		chromedp.Flag("disable-translate", true),
		chromedp.Flag("mute-audio", true),
		chromedp.Flag("hide-scrollbars", true),
		chromedp.Flag("force-device-scale-factor", "1"),
	)

	r.allocCtx, r.allocCancel = chromedp.NewExecAllocator(ctx, opts...)
	r.ctx, r.cancel = chromedp.NewContext(r.allocCtx)

	// Navigate to the theme
	addr := r.listener.Addr().(*net.TCPAddr)
	url := fmt.Sprintf("http://127.0.0.1:%d/index.html", addr.Port)
	if err := chromedp.Run(r.ctx,
		chromedp.EmulateViewport(int64(r.width), int64(r.height), chromedp.EmulateScale(1)),
		chromedp.Navigate(url),
		chromedp.WaitReady("body"),
	); err != nil {
		r.Stop()
		return fmt.Errorf("failed to load theme: %w", err)
	}

	return nil
}

// startServer starts an HTTP server to serve theme files.
func (r *Renderer) startServer() error {
	// Find a free port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	r.listener = listener

	// Create file server
	fs := http.FileServer(http.Dir(r.themeDistDir))

	// Add WebSocket handler for sensor data (future enhancement)
	mux := http.NewServeMux()
	mux.Handle("/", fs)

	r.server = &http.Server{
		Handler: mux,
	}

	go func() {
		r.server.Serve(r.listener)
	}()

	return nil
}

// ServerPort returns the port the theme server is running on.
func (r *Renderer) ServerPort() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.listener == nil {
		return 0
	}
	return r.listener.Addr().(*net.TCPAddr).Port
}

// Stop stops the renderer and cleans up.
func (r *Renderer) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.cancel != nil {
		r.cancel()
		r.cancel = nil
	}
	if r.allocCancel != nil {
		r.allocCancel()
		r.allocCancel = nil
	}
	if r.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		r.server.Shutdown(ctx)
		r.server = nil
	}
	if r.listener != nil {
		r.listener.Close()
		r.listener = nil
	}
	r.ctx = nil
}

// Capture takes a screenshot and returns it as an image.
func (r *Renderer) Capture() (image.Image, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.ctx == nil {
		return nil, fmt.Errorf("renderer not started")
	}

	var buf []byte
	if err := chromedp.Run(r.ctx,
		chromedp.CaptureScreenshot(&buf),
	); err != nil {
		return nil, fmt.Errorf("screenshot failed: %w", err)
	}

	// Decode PNG
	img, err := png.Decode(bytes.NewReader(buf))
	if err != nil {
		return nil, fmt.Errorf("failed to decode screenshot: %w", err)
	}

	return img, nil
}

// CaptureToFile takes a screenshot and saves it to a file.
func (r *Renderer) CaptureToFile(path string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.ctx == nil {
		return fmt.Errorf("renderer not started")
	}

	var buf []byte
	if err := chromedp.Run(r.ctx,
		chromedp.CaptureScreenshot(&buf),
	); err != nil {
		return fmt.Errorf("screenshot failed: %w", err)
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	return os.WriteFile(path, buf, 0644)
}

// SendSensorData sends sensor data to the theme via JavaScript.
func (r *Renderer) SendSensorData(jsonData string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.ctx == nil {
		return fmt.Errorf("renderer not started")
	}

	script := fmt.Sprintf(`
		window.postMessage({ type: 'sensorData', data: %s }, '*');
	`, jsonData)

	return chromedp.Run(r.ctx, chromedp.Evaluate(script, nil))
}
