// cmd/run.go - Main dashboard run command
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/oae/sensorpanel/pkg/animation"
	"github.com/oae/sensorpanel/pkg/browser"
	"github.com/oae/sensorpanel/pkg/config"
	"github.com/oae/sensorpanel/pkg/music"
	"github.com/oae/sensorpanel/pkg/panel"
	"github.com/oae/sensorpanel/pkg/renderer"
	"github.com/oae/sensorpanel/pkg/sensors"
	"github.com/oae/sensorpanel/pkg/server"
	"github.com/oae/sensorpanel/pkg/theme"
)

var (
	runInterval       float64
	runBrightness     int
	runSensors        []string
	runExcludeSensors []string
	runOpts           []string
	runGIF            string
	runImage          string
	runMusic          bool
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the sensor panel dashboard",
	Long: `Start the sensor panel dashboard, displaying system metrics
on the USB display in a continuous loop.

By default, all available sensors are enabled. Use --sensors to enable only
specific sensors, or --exclude to disable specific sensors.

Use --gif to play an animated GIF instead of displaying sensor data. GIF
playback skips sensor collection and the selected theme.

Use --image to display a PNG, JPEG, or GIF file or URL as a static image.

Use --music for a now-playing dashboard with cover art, track and artist
information, a song-specific progress waveform, and auto-scrolling timed lyrics.

Available sensors (varies by platform):
  cpu        - CPU temperature, load, frequency
  memory     - RAM usage
  nvidia_gpu - NVIDIA GPU stats
  amd_gpu    - AMD GPU stats
  disk       - Disk usage per mount point
  network    - Network throughput per interface

Sensor options (use --opt key=value):
  disk.mounts=/,/home      - Disk mount points to monitor
  network.interface=eth*   - Network interface filter pattern
  nvidia_gpu.smi_path=...  - Custom path to nvidia-smi

Examples:
  sensorpanel run                                    # All sensors
  sensorpanel run -s cpu,memory,disk                 # Only CPU, memory, and disk
  sensorpanel run -x network                         # All except network
  sensorpanel run --opt disk.mounts=/,/home          # Monitor specific mounts
  sensorpanel run --gif animation.gif                # Play a local GIF continuously
  sensorpanel run --gif https://example.com/a.gif    # Play a remote GIF
  sensorpanel run --image wallpaper.png              # Show a static image
  sensorpanel run --music                            # Show now-playing dashboard

Press Ctrl+C to stop. The backlight will be turned off on exit.`,
	RunE: runDashboard,
}

func init() {
	rootCmd.AddCommand(runCmd)

	runCmd.Flags().Float64VarP(&runInterval, "interval", "i", 1.0, "Update interval in seconds (min 0.5; music min 0.25)")
	runCmd.Flags().IntVarP(&runBrightness, "brightness", "b", 7, "Backlight brightness (0-7)")
	runCmd.Flags().StringSliceVarP(&runSensors, "sensors", "s", nil, "Sensors to enable (e.g., cpu,memory,disk). Default: all available")
	runCmd.Flags().StringSliceVarP(&runExcludeSensors, "exclude", "x", nil, "Sensors to exclude (e.g., network,nvidia_gpu)")
	runCmd.Flags().StringSliceVarP(&runOpts, "opt", "o", nil, "Sensor options in key=value format (e.g., disk.mounts=/,/home)")
	runCmd.Flags().StringVar(&runGIF, "gif", "", "Play an animated GIF file or URL instead of sensor data")
	runCmd.Flags().StringVar(&runImage, "image", "", "Display a PNG, JPEG, or GIF file or URL instead of sensor data")
	runCmd.Flags().BoolVar(&runMusic, "music", false, "Show now-playing music dashboard instead of sensor data")
}

func runDashboard(cmd *cobra.Command, args []string) error {
	mediaModes := 0
	if runGIF != "" {
		mediaModes++
	}
	if runImage != "" {
		mediaModes++
	}
	if runMusic {
		mediaModes++
	}
	if mediaModes > 1 {
		return fmt.Errorf("--gif, --image, and --music cannot be used together")
	}

	// Load config for defaults
	cfg, err := config.Load()
	if err != nil {
		cfg = config.DefaultConfig()
	}

	// Use config defaults if flags not explicitly set
	if !cmd.Flags().Changed("interval") && cfg.UpdateInterval > 0 {
		runInterval = cfg.UpdateInterval
	}
	if !cmd.Flags().Changed("brightness") {
		runBrightness = cfg.Brightness
	}

	if runMusic && !cmd.Flags().Changed("interval") {
		runInterval = 0.5
	}

	// Validate interval. Music mode uses a moderate cadence for progress updates.
	minimumInterval := 0.5
	if runMusic {
		minimumInterval = 0.25
	}
	if runInterval < minimumInterval {
		runInterval = minimumInterval
	}

	// Clamp brightness
	if runBrightness < 0 {
		runBrightness = 0
	}
	if runBrightness > 7 {
		runBrightness = 7
	}

	// Create and open device using config
	dev, err := openConfiguredDevice()
	fmt.Println("Opening USB display...")
	if err != nil {
		return fmt.Errorf("failed to open device: %w", err)
	}
	defer dev.Close()

	fmt.Printf("Connected: %s\n", dev.Info.String())

	// Set brightness
	if err := dev.SetBacklight(runBrightness); err != nil {
		fmt.Printf("Warning: failed to set brightness: %v\n", err)
	}

	// Setup signal handling for graceful shutdown.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	if runGIF != "" {
		return runGIFPlayback(dev, runGIF, sigChan)
	}
	if runImage != "" {
		return runStaticImage(dev, runImage, sigChan)
	}
	if runMusic {
		return runMusicDashboard(dev, sigChan)
	}

	// Configure sensors
	var enabledSensors map[string]bool
	if len(runSensors) > 0 {
		// If --sensors flag is provided, only enable those sensors
		enabledSensors = make(map[string]bool)
		for _, s := range runSensors {
			enabledSensors[s] = true
		}
	}
	// If no --sensors flag, enabledSensors stays nil (all enabled)

	// Start with sensor options from config, then override with CLI flags
	var options map[string]interface{}
	if cfg.SensorOptions != nil {
		options = make(map[string]interface{})
		for k, v := range cfg.SensorOptions {
			options[k] = v
		}
	}

	// Parse sensor options from --opt flags (override config)
	if len(runOpts) > 0 {
		if options == nil {
			options = make(map[string]interface{})
		}
		for _, opt := range runOpts {
			key, value, ok := strings.Cut(opt, "=")
			if !ok {
				return fmt.Errorf("invalid option format %q (expected key=value)", opt)
			}
			// If value contains commas, treat it as a string slice
			if strings.Contains(value, ",") {
				options[key] = strings.Split(value, ",")
			} else {
				options[key] = value
			}
		}
	}

	sensorConfig := &sensors.Config{
		EnabledSensors:  enabledSensors,
		DisabledSensors: runExcludeSensors,
		Options:         options,
	}
	collector := sensors.NewCollector(sensorConfig)

	// Check if a theme is selected
	themeName := cfg.Theme
	if themeName != "" {
		return runWithTheme(dev, collector, cfg, themeName, sigChan)
	}

	// No theme selected - use built-in renderer
	return runWithBuiltinRenderer(dev, collector, sigChan)
}

func runMusicDashboard(dev *panel.Device, sigChan chan os.Signal) error {
	if _, err := exec.LookPath("playerctl"); err != nil {
		return fmt.Errorf("music dashboard requires playerctl: %w", err)
	}
	if err := ensureBrowserAvailable(); err != nil {
		return err
	}

	dashboardDir, err := os.MkdirTemp("", "sensorpanel-music-*")
	if err != nil {
		return fmt.Errorf("create music dashboard: %w", err)
	}
	defer os.RemoveAll(dashboardDir)
	if err := os.WriteFile(filepath.Join(dashboardDir, "index.html"), music.DashboardHTML, 0o600); err != nil {
		return fmt.Errorf("write music dashboard: %w", err)
	}

	width, height := dev.Profile.Width(), dev.Profile.Height()
	browserRenderer, err := browser.NewRenderer(width, height)
	if err != nil {
		return fmt.Errorf("initialize music dashboard browser: %w", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := browserRenderer.Start(ctx, dashboardDir); err != nil {
		return fmt.Errorf("start music dashboard browser: %w", err)
	}
	defer browserRenderer.Stop()

	monitor := music.NewMonitor()
	monitor.Start(ctx)
	frameUpdater := panel.NewFrameUpdater(dev)
	fmt.Printf("Music dashboard running (%.2fs interval). Press Ctrl+C to stop.\n", runInterval)

	ticker := time.NewTicker(time.Duration(runInterval * float64(time.Second)))
	defer ticker.Stop()
	firstFrame := true
	for {
		if err := renderMusicFrame(dev, frameUpdater, monitor, browserRenderer, firstFrame); err != nil {
			fmt.Printf("Music frame error: %v\n", err)
		}
		firstFrame = false
		select {
		case <-sigChan:
			fmt.Println("\nStopping music dashboard...")
			return nil
		case <-ticker.C:
		}
	}
}

func renderMusicFrame(dev *panel.Device, frameUpdater *panel.FrameUpdater, monitor *music.Monitor, browserRenderer *browser.Renderer, firstFrame bool) error {
	data := map[string]interface{}{"music": monitor.Snapshot()}
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("encode music state: %w", err)
	}
	if err := browserRenderer.SendSensorData(string(jsonData)); err != nil {
		return fmt.Errorf("update music dashboard: %w", err)
	}
	if firstFrame {
		time.Sleep(150 * time.Millisecond)
	}
	img, err := browserRenderer.Capture()
	if err != nil {
		return fmt.Errorf("capture music dashboard: %w", err)
	}
	if _, err := frameUpdater.Display(dev.Profile.ConvertImage(img)); err != nil {
		return fmt.Errorf("display music dashboard: %w", err)
	}
	return nil
}

// runStaticImage displays one image and keeps it on screen until interrupted.
func runStaticImage(dev *panel.Device, source string, sigChan chan os.Signal) error {
	img, err := animation.LoadImage(source, dev.Profile.Width(), dev.Profile.Height())
	if err != nil {
		return err
	}

	if err := dev.DisplayBuffer(dev.Profile.ConvertImage(img)); err != nil {
		return fmt.Errorf("display image: %w", err)
	}

	fmt.Printf("Displaying image: %s. Press Ctrl+C to stop.\n", source)
	<-sigChan
	fmt.Println("\nStopping image display...")
	return nil
}

// runGIFPlayback displays an animated GIF continuously at its encoded frame rate.
func runGIFPlayback(dev *panel.Device, path string, sigChan chan os.Signal) error {
	animation, err := animation.LoadGIF(path, dev.Profile.Width(), dev.Profile.Height())
	if err != nil {
		return err
	}

	fmt.Printf("Playing GIF: %s (%d frames). Press Ctrl+C to stop.\n", path, len(animation.Frames))
	buffers := make([][]byte, len(animation.Frames))
	for i, image := range animation.Frames {
		buffers[i] = dev.Profile.ConvertImage(image)
	}

	// The panel applies each regional command independently and has no frame
	// commit/vsync operation. Keep GIF frames to one write so parts of
	// different animation frames are not visible at the same time.
	frameUpdater := panel.NewCoherentFrameUpdater(dev)
	frame := 0
	nextDeadline := time.Now()
	started := time.Now()
	displayedFrames := 0
	skippedFrames := 0
	bytesSent := 0
	regionalWrites := 0
	fullFrames := 0
	for {
		stats, err := frameUpdater.Display(buffers[frame])
		if err != nil {
			return fmt.Errorf("display GIF frame: %w", err)
		}
		displayedFrames++
		bytesSent += stats.Bytes
		regionalWrites += stats.Regions
		if stats.FullFrame {
			fullFrames++
		}

		nextDeadline = nextDeadline.Add(animation.Delays[frame])
		nextFrame := (frame + 1) % len(animation.Frames)
		for !nextDeadline.After(time.Now()) {
			skippedFrames++
			nextDeadline = nextDeadline.Add(animation.Delays[nextFrame])
			nextFrame = (nextFrame + 1) % len(animation.Frames)
		}

		timer := time.NewTimer(time.Until(nextDeadline))
		select {
		case <-sigChan:
			if !timer.Stop() {
				<-timer.C
			}
			elapsed := time.Since(started)
			fmt.Printf("\nGIF performance: %.2f FPS, %.1f KB/s, %d displayed, %d skipped, %d regional writes, %d full frames\n",
				float64(displayedFrames)/elapsed.Seconds(),
				float64(bytesSent)/elapsed.Seconds()/1024,
				displayedFrames, skippedFrames, regionalWrites, fullFrames)
			fmt.Println("\nStopping GIF playback...")
			return nil
		case <-timer.C:
			frame = nextFrame
		}
	}
}

// runWithTheme runs the dashboard using a theme with headless browser rendering.
func runWithTheme(dev *panel.Device, collector *sensors.Collector, cfg *config.Config, themeName string, sigChan chan os.Signal) error {
	// Load the theme
	t, err := theme.Load(themeName)
	if err != nil {
		return fmt.Errorf("failed to load theme '%s': %w", themeName, err)
	}

	if !t.HasDist {
		return fmt.Errorf("theme '%s' is not built (missing dist/index.html)\nRun 'cd %s && npm install && npm run build' to build it", themeName, t.Path)
	}

	// Check if theme is outdated
	if theme.IsOutdated(t.Path) {
		fmt.Printf("Warning: Theme '%s' may be outdated (src/ is newer than dist/)\n", themeName)
		fmt.Printf("Run 'sensorpanel theme build %s' to rebuild\n\n", themeName)
	}

	fmt.Printf("Using theme: %s\n", themeName)

	// Check if browser is available, auto-download if not
	if err := ensureBrowserAvailable(); err != nil {
		return err
	}

	// Start the theme server with WebSocket support
	srv := server.New(t.DistDir())
	if err := srv.Start(); err != nil {
		return fmt.Errorf("failed to start theme server: %w", err)
	}
	defer srv.Stop()

	fmt.Printf("Theme server running at %s\n", srv.URL())

	// Use device profile dimensions, or fall back to theme metadata
	width := dev.Profile.Width()
	height := dev.Profile.Height()
	if t.Metadata.Width > 0 && t.Metadata.Height > 0 {
		// Theme specifies its own dimensions - use those for browser rendering
		width = t.Metadata.Width
		height = t.Metadata.Height
	}

	// Start headless browser
	browserRenderer, err := browser.NewRenderer(width, height)
	if err != nil {
		return fmt.Errorf("failed to initialize browser: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := browserRenderer.Start(ctx, t.DistDir()); err != nil {
		return fmt.Errorf("failed to start browser: %w", err)
	}
	defer browserRenderer.Stop()

	fmt.Printf("Dashboard running with theme (%.1fs interval). Press Ctrl+C to stop.\n", runInterval)

	// Initial collection to prime CPU load calculation
	collector.CollectAll()
	time.Sleep(100 * time.Millisecond)

	// Main loop
	ticker := time.NewTicker(time.Duration(runInterval * float64(time.Second)))
	defer ticker.Stop()

	frameCount := 0
	startTime := time.Now()
	frameUpdater := panel.NewFrameUpdater(dev)

	// Render first frame immediately
	if err := renderThemeFrame(dev, frameUpdater, collector, srv, browserRenderer, &frameCount); err != nil {
		fmt.Printf("Frame error: %v\n", err)
	}

	for {
		select {
		case <-sigChan:
			fmt.Println("\nShutting down...")
			elapsed := time.Since(startTime).Seconds()
			if elapsed > 0 {
				fps := float64(frameCount) / elapsed
				fmt.Printf("Rendered %d frames in %.1fs (%.2f FPS)\n", frameCount, elapsed, fps)
			}
			return nil

		case <-ticker.C:
			if err := renderThemeFrame(dev, frameUpdater, collector, srv, browserRenderer, &frameCount); err != nil {
				fmt.Printf("Frame error: %v\n", err)
			}
		}
	}
}

// runWithBuiltinRenderer runs the dashboard using the built-in bitmap renderer.
func runWithBuiltinRenderer(dev *panel.Device, collector *sensors.Collector, sigChan chan os.Signal) error {
	// Configure renderer with device profile dimensions
	renderConfig := &renderer.Config{
		Width:  dev.Profile.Width(),
		Height: dev.Profile.Height(),
	}
	render := renderer.New(renderConfig)

	fmt.Printf("Dashboard running (%.1fs interval). Press Ctrl+C to stop.\n", runInterval)

	// Initial collection to prime CPU load calculation
	collector.CollectAll()
	time.Sleep(100 * time.Millisecond)

	// Main loop
	ticker := time.NewTicker(time.Duration(runInterval * float64(time.Second)))
	defer ticker.Stop()

	frameCount := 0
	startTime := time.Now()
	frameUpdater := panel.NewFrameUpdater(dev)

	// Render first frame immediately
	renderFrame(dev, frameUpdater, collector, render, &frameCount)

	for {
		select {
		case <-sigChan:
			fmt.Println("\nShutting down...")
			elapsed := time.Since(startTime).Seconds()
			if elapsed > 0 {
				fps := float64(frameCount) / elapsed
				fmt.Printf("Rendered %d frames in %.1fs (%.2f FPS)\n", frameCount, elapsed, fps)
			}
			return nil

		case <-ticker.C:
			renderFrame(dev, frameUpdater, collector, render, &frameCount)
		}
	}
}

func renderFrame(dev *panel.Device, frameUpdater *panel.FrameUpdater, collector *sensors.Collector, render *renderer.Renderer, frameCount *int) {
	// Collect sensor data
	data := collector.CollectAll()

	// Render to image
	img := render.Render(data)

	// Convert to RGB565 and send to display
	buffer := dev.Profile.ConvertImage(img)
	if _, err := frameUpdater.Display(buffer); err != nil {
		fmt.Printf("Display error: %v\n", err)
		return
	}

	*frameCount++
}

// renderThemeFrame collects sensor data, broadcasts to theme, captures screenshot, and sends to display.
func renderThemeFrame(dev *panel.Device, frameUpdater *panel.FrameUpdater, collector *sensors.Collector, srv *server.Server, browserRenderer *browser.Renderer, frameCount *int) error {
	// Collect sensor data
	data := collector.CollectAll()

	// Broadcast sensor data to theme via WebSocket
	if err := srv.BroadcastSensorData(data); err != nil {
		// Non-fatal: theme might not have connected yet
		_ = err
	}

	// Also inject via postMessage for themes that use that method
	jsonBytes, err := json.Marshal(data)
	if err == nil {
		_ = browserRenderer.SendSensorData(string(jsonBytes))
	}

	// Give the theme a moment to render (if first frame)
	// This is only needed for initial load
	if *frameCount == 0 {
		time.Sleep(100 * time.Millisecond)
	}

	// Capture screenshot from browser
	img, err := browserRenderer.Capture()
	if err != nil {
		return fmt.Errorf("capture failed: %w", err)
	}

	// Convert to RGB565 and send to display
	buffer := dev.Profile.ConvertImage(img)
	if _, err := frameUpdater.Display(buffer); err != nil {
		return fmt.Errorf("display failed: %w", err)
	}

	*frameCount++
	return nil
}

// ensureBrowserAvailable checks if a browser is available and downloads one if needed.
func ensureBrowserAvailable() error {
	// Check if browser is already available
	if _, err := browser.GetChromePath(); err == nil {
		return nil
	}

	// Browser not found - auto-download
	fmt.Println("Browser not found. Downloading Chrome for Testing...")
	fmt.Printf("Version: %s\n", browser.Version())

	var lastPercent int
	err := browser.Download(context.Background(), func(downloaded, total int64) {
		if total > 0 {
			percent := int(downloaded * 100 / total)
			// Only print every 10%
			if percent/10 > lastPercent/10 {
				lastPercent = percent
				fmt.Printf("  %d%% (%d / %d MB)\n", percent, downloaded/(1024*1024), total/(1024*1024))
			}
		}
	})

	if err != nil {
		return fmt.Errorf("failed to download browser: %w", err)
	}

	chromePath, err := browser.GetChromePath()
	if err != nil {
		return fmt.Errorf("browser download succeeded but not found: %w", err)
	}

	fmt.Printf("Browser installed: %s\n", chromePath)
	return nil
}
