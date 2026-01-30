// cmd/run.go - Main dashboard run command
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/alperen/sensorpanel/pkg/browser"
	"github.com/alperen/sensorpanel/pkg/config"
	"github.com/alperen/sensorpanel/pkg/panel"
	"github.com/alperen/sensorpanel/pkg/renderer"
	"github.com/alperen/sensorpanel/pkg/sensors"
	"github.com/alperen/sensorpanel/pkg/server"
	"github.com/alperen/sensorpanel/pkg/theme"
)

var (
	runInterval    float64
	runBrightness  int
	runShowCPU     bool
	runShowGPU     bool
	runShowRAM     bool
	runShowDisk    bool
	runShowNetwork bool
	runDiskMounts  []string
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the sensor panel dashboard",
	Long: `Start the sensor panel dashboard, displaying system metrics
on the USB display in a continuous loop.

The dashboard shows:
  - CPU: temperature, load, frequency
  - GPU: temperature, load, memory, power (NVIDIA/AMD)
  - RAM: usage percentage and GB
  - Disk: usage per mount point
  - Network: throughput per interface

Press Ctrl+C to stop. The backlight will be turned off on exit.`,
	RunE: runDashboard,
}

func init() {
	rootCmd.AddCommand(runCmd)

	runCmd.Flags().Float64VarP(&runInterval, "interval", "i", 1.0, "Update interval in seconds (min 0.5)")
	runCmd.Flags().IntVarP(&runBrightness, "brightness", "b", 7, "Backlight brightness (0-7)")
	runCmd.Flags().BoolVar(&runShowCPU, "cpu", true, "Show CPU stats")
	runCmd.Flags().BoolVar(&runShowGPU, "gpu", true, "Show GPU stats")
	runCmd.Flags().BoolVar(&runShowRAM, "ram", true, "Show RAM stats")
	runCmd.Flags().BoolVar(&runShowDisk, "disk", true, "Show disk stats")
	runCmd.Flags().BoolVar(&runShowNetwork, "network", true, "Show network stats")
	runCmd.Flags().StringSliceVarP(&runDiskMounts, "mounts", "m", []string{"/"}, "Disk mount points to monitor")
}

func runDashboard(cmd *cobra.Command, args []string) error {
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
	if !cmd.Flags().Changed("mounts") && len(cfg.DiskMounts) > 0 {
		runDiskMounts = cfg.DiskMounts
	}

	// Validate interval
	if runInterval < 0.5 {
		runInterval = 0.5
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

	// Configure sensors
	sensorConfig := &sensors.Config{
		ShowCPU:          runShowCPU,
		ShowGPU:          runShowGPU,
		ShowRAM:          runShowRAM,
		ShowDisk:         runShowDisk,
		ShowNetwork:      runShowNetwork,
		DiskMounts:       runDiskMounts,
		NetworkInterface: "*",
		GPUMethod:        "auto",
	}
	collector := sensors.NewCollector(sensorConfig)

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Check if a theme is selected
	themeName := cfg.Theme
	if themeName != "" {
		return runWithTheme(dev, collector, cfg, themeName, sigChan)
	}

	// No theme selected - use built-in renderer
	return runWithBuiltinRenderer(dev, collector, sigChan)
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

	// Start headless browser
	browserRenderer, err := browser.NewRenderer(t.Metadata.Width, t.Metadata.Height)
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
	collector.Collect()
	time.Sleep(100 * time.Millisecond)

	// Main loop
	ticker := time.NewTicker(time.Duration(runInterval * float64(time.Second)))
	defer ticker.Stop()

	frameCount := 0
	startTime := time.Now()

	// Render first frame immediately
	if err := renderThemeFrame(dev, collector, srv, browserRenderer, &frameCount); err != nil {
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
			if err := renderThemeFrame(dev, collector, srv, browserRenderer, &frameCount); err != nil {
				fmt.Printf("Frame error: %v\n", err)
			}
		}
	}
}

// runWithBuiltinRenderer runs the dashboard using the built-in bitmap renderer.
func runWithBuiltinRenderer(dev *panel.Device, collector *sensors.Collector, sigChan chan os.Signal) error {
	// Configure renderer
	render := renderer.New(nil) // Use defaults (480x320)

	fmt.Printf("Dashboard running (%.1fs interval). Press Ctrl+C to stop.\n", runInterval)

	// Initial collection to prime CPU load calculation
	collector.Collect()
	time.Sleep(100 * time.Millisecond)

	// Main loop
	ticker := time.NewTicker(time.Duration(runInterval * float64(time.Second)))
	defer ticker.Stop()

	frameCount := 0
	startTime := time.Now()

	// Render first frame immediately
	renderFrame(dev, collector, render, &frameCount)

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
			renderFrame(dev, collector, render, &frameCount)
		}
	}
}

func renderFrame(dev *panel.Device, collector *sensors.Collector, render *renderer.Renderer, frameCount *int) {
	// Collect sensor data
	data := collector.Collect()

	// Render to image
	img := render.Render(data)

	// Convert to RGB565 and send to display
	buffer := panel.ImageToRGB565Buffer(img)
	if err := dev.DisplayBuffer(buffer); err != nil {
		fmt.Printf("Display error: %v\n", err)
		return
	}

	*frameCount++
}

// renderThemeFrame collects sensor data, broadcasts to theme, captures screenshot, and sends to display.
func renderThemeFrame(dev *panel.Device, collector *sensors.Collector, srv *server.Server, browserRenderer *browser.Renderer, frameCount *int) error {
	// Collect sensor data
	data := collector.Collect()

	// Convert to JSON-friendly format for the theme
	jsonData := sensorDataToJSON(data)

	// Broadcast sensor data to theme via WebSocket
	if err := srv.BroadcastSensorData(jsonData); err != nil {
		// Non-fatal: theme might not have connected yet
		_ = err
	}

	// Also inject via postMessage for themes that use that method
	jsonBytes, err := json.Marshal(jsonData)
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
	buffer := panel.ImageToRGB565Buffer(img)
	if err := dev.DisplayBuffer(buffer); err != nil {
		return fmt.Errorf("display failed: %w", err)
	}

	*frameCount++
	return nil
}

// SensorDataJSON is the JSON format sent to themes via WebSocket.
type SensorDataJSON struct {
	CPU      CPUDataJSON       `json:"cpu"`
	GPU      GPUDataJSON       `json:"gpu"`
	Memory   MemoryDataJSON    `json:"memory"`
	Disks    []DiskDataJSON    `json:"disks"`
	Networks []NetworkDataJSON `json:"networks"`
}

type CPUDataJSON struct {
	LoadPercent  float64  `json:"load_percent"`
	Temperature  *float64 `json:"temperature,omitempty"`
	FrequencyMHz *float64 `json:"frequency_mhz,omitempty"`
	CoreCount    int      `json:"core_count"`
}

type GPUDataJSON struct {
	Available     bool     `json:"available"`
	Name          string   `json:"name,omitempty"`
	LoadPercent   *float64 `json:"load_percent,omitempty"`
	Temperature   *float64 `json:"temperature,omitempty"`
	MemoryUsedMB  *float64 `json:"memory_used_mb,omitempty"`
	MemoryTotalMB *float64 `json:"memory_total_mb,omitempty"`
	PowerWatts    *float64 `json:"power_watts,omitempty"`
}

type MemoryDataJSON struct {
	TotalMB     float64 `json:"total_mb"`
	UsedMB      float64 `json:"used_mb"`
	AvailableMB float64 `json:"available_mb"`
	Percent     float64 `json:"percent"`
}

type DiskDataJSON struct {
	MountPoint string  `json:"mount_point"`
	TotalGB    float64 `json:"total_gb"`
	UsedGB     float64 `json:"used_gb"`
	FreeGB     float64 `json:"free_gb"`
	Percent    float64 `json:"percent"`
}

type NetworkDataJSON struct {
	Interface     string  `json:"interface"`
	RxBytesPerSec float64 `json:"rx_bytes_per_sec"`
	TxBytesPerSec float64 `json:"tx_bytes_per_sec"`
}

// sensorDataToJSON converts sensor data to the JSON format expected by themes.
func sensorDataToJSON(data *sensors.Data) SensorDataJSON {
	result := SensorDataJSON{
		CPU: CPUDataJSON{
			LoadPercent:  data.CPU.LoadPercent,
			Temperature:  data.CPU.Temperature,
			FrequencyMHz: data.CPU.FrequencyMHz,
			CoreCount:    data.CPU.CoreCount,
		},
		GPU: GPUDataJSON{
			Available:     data.GPU.Available,
			Name:          data.GPU.Name,
			LoadPercent:   data.GPU.LoadPercent,
			Temperature:   data.GPU.Temperature,
			MemoryUsedMB:  data.GPU.MemoryUsedMB,
			MemoryTotalMB: data.GPU.MemoryTotalMB,
			PowerWatts:    data.GPU.PowerWatts,
		},
		Memory: MemoryDataJSON{
			TotalMB:     data.Memory.TotalMB,
			UsedMB:      data.Memory.UsedMB,
			AvailableMB: data.Memory.AvailableMB,
			Percent:     data.Memory.Percent,
		},
	}

	for _, disk := range data.Disks {
		result.Disks = append(result.Disks, DiskDataJSON{
			MountPoint: disk.MountPoint,
			TotalGB:    disk.TotalGB,
			UsedGB:     disk.UsedGB,
			FreeGB:     disk.FreeGB,
			Percent:    disk.Percent,
		})
	}

	for _, net := range data.Networks {
		result.Networks = append(result.Networks, NetworkDataJSON{
			Interface:     net.Interface,
			RxBytesPerSec: net.RxBytesPerSec,
			TxBytesPerSec: net.TxBytesPerSec,
		})
	}

	return result
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
