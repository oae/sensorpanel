package theme

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/oae/sensorpanel/pkg/sensors"
)

// DevServer orchestrates the theme development experience.
type DevServer struct {
	ThemeDir      string
	WSPort        int // WebSocket sensor server port (default 19847)
	VitePort      int // Vite dev server port (default 15173)
	NoBrowser     bool
	Interval      float64                // Sensor update interval in seconds
	SensorOptions map[string]interface{} // Sensor provider options

	// Internal state
	wsServer    *http.Server
	wsClients   map[*websocket.Conn]bool
	wsClientsMu sync.RWMutex
	viteCmd     *exec.Cmd
	collector   *sensors.Collector
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewDevServer creates a new dev server for a theme.
func NewDevServer(themeDir string) *DevServer {
	return &DevServer{
		ThemeDir:  themeDir,
		WSPort:    19847,
		VitePort:  15173,
		Interval:  1.0,
		wsClients: make(map[*websocket.Conn]bool),
	}
}

// Start starts the dev server.
func (d *DevServer) Start(ctx context.Context) error {
	d.ctx, d.cancel = context.WithCancel(ctx)

	// Detect package manager
	pm := DetectPackageManager(d.ThemeDir)
	fmt.Printf("[dev] Using package manager: %s\n", pm)

	// Install dependencies if needed
	if !HasNodeModules(d.ThemeDir) {
		fmt.Println("[dev] Installing dependencies...")
		cmdArgs := pm.InstallCmd()
		cmd := exec.CommandContext(d.ctx, cmdArgs[0], cmdArgs[1:]...)
		cmd.Dir = d.ThemeDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to install dependencies: %w", err)
		}
	}

	// Find available ports
	wsPort, err := d.findAvailablePort(d.WSPort, 5)
	if err != nil {
		return fmt.Errorf("failed to find WebSocket port: %w", err)
	}
	d.WSPort = wsPort

	// Start WebSocket sensor server
	if err := d.startWSServer(); err != nil {
		return fmt.Errorf("failed to start WebSocket server: %w", err)
	}

	// Start Vite dev server
	if err := d.startVite(pm); err != nil {
		d.Stop()
		return fmt.Errorf("failed to start Vite: %w", err)
	}

	// Start sensor collection
	d.startSensorCollection()

	// Wait for Vite to be ready
	viteURL := fmt.Sprintf("http://localhost:%d", d.VitePort)
	if err := d.waitForVite(viteURL, 30*time.Second); err != nil {
		fmt.Printf("[dev] Warning: Vite may not be ready: %v\n", err)
	}

	// Open browser
	if !d.NoBrowser && CanOpenBrowser() {
		// Add ws query param so the theme connects to our sensor server
		browserURL := fmt.Sprintf("%s?ws=%d", viteURL, d.WSPort)
		fmt.Printf("[dev] Opening browser: %s\n", browserURL)
		if err := OpenBrowser(browserURL); err != nil {
			fmt.Printf("[dev] Failed to open browser: %v\n", err)
		}
	}

	fmt.Println()
	fmt.Printf("[dev] Theme dev server running\n")
	fmt.Printf("[dev] Vite:      http://localhost:%d\n", d.VitePort)
	fmt.Printf("[dev] WebSocket: ws://localhost:%d/ws\n", d.WSPort)
	fmt.Println("[dev] Press Ctrl+C to stop")
	fmt.Println()

	return nil
}

// Stop stops all dev server components.
func (d *DevServer) Stop() {
	if d.cancel != nil {
		d.cancel()
	}

	// Stop Vite
	if d.viteCmd != nil && d.viteCmd.Process != nil {
		d.viteCmd.Process.Kill()
		d.viteCmd.Wait()
	}

	// Stop WebSocket server
	if d.wsServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		d.wsServer.Shutdown(ctx)
	}

	// Close WebSocket clients
	d.wsClientsMu.Lock()
	for conn := range d.wsClients {
		conn.Close()
	}
	d.wsClients = make(map[*websocket.Conn]bool)
	d.wsClientsMu.Unlock()

	fmt.Println("[dev] Stopped")
}

// Wait blocks until the dev server is stopped.
func (d *DevServer) Wait() {
	if d.viteCmd != nil {
		d.viteCmd.Wait()
	}
}

// startWSServer starts the WebSocket sensor server.
func (d *DevServer) startWSServer() error {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all origins for dev
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}

		d.wsClientsMu.Lock()
		d.wsClients[conn] = true
		d.wsClientsMu.Unlock()

		fmt.Printf("[ws] Client connected from %s\n", r.RemoteAddr)

		// Keep connection open, handle disconnects
		go func() {
			defer func() {
				d.wsClientsMu.Lock()
				delete(d.wsClients, conn)
				d.wsClientsMu.Unlock()
				conn.Close()
				fmt.Printf("[ws] Client disconnected\n")
			}()

			for {
				_, _, err := conn.ReadMessage()
				if err != nil {
					return
				}
			}
		}()
	})

	d.wsServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", d.WSPort),
		Handler: mux,
	}

	go func() {
		fmt.Printf("[ws] Sensor server listening on port %d\n", d.WSPort)
		if err := d.wsServer.ListenAndServe(); err != http.ErrServerClosed {
			fmt.Printf("[ws] Server error: %v\n", err)
		}
	}()

	return nil
}

// startVite starts the Vite dev server.
func (d *DevServer) startVite(pm PackageManager) error {
	cmdArgs := pm.DevCmd()

	// Add port flag for vite
	cmdArgs = append(cmdArgs, "--", "--port", fmt.Sprintf("%d", d.VitePort))

	d.viteCmd = exec.CommandContext(d.ctx, cmdArgs[0], cmdArgs[1:]...)
	d.viteCmd.Dir = d.ThemeDir
	d.viteCmd.Env = append(os.Environ(), fmt.Sprintf("PORT=%d", d.VitePort))

	// Capture and prefix output
	stdout, err := d.viteCmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := d.viteCmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := d.viteCmd.Start(); err != nil {
		return err
	}

	// Stream output with prefix
	go d.prefixOutput(stdout, "[vite]")
	go d.prefixOutput(stderr, "[vite]")

	return nil
}

// prefixOutput reads from r and prints each line with a prefix.
func (d *DevServer) prefixOutput(r io.Reader, prefix string) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		fmt.Printf("%s %s\n", prefix, scanner.Text())
	}
}

// startSensorCollection starts collecting sensor data.
func (d *DevServer) startSensorCollection() {
	config := &sensors.Config{
		EnabledSensors:  nil, // All sensors enabled
		DisabledSensors: nil,
		Options:         d.SensorOptions,
	}
	d.collector = sensors.NewCollector(config)

	// Prime the collector
	d.collector.CollectAll()

	go func() {
		ticker := time.NewTicker(time.Duration(d.Interval * float64(time.Second)))
		defer ticker.Stop()

		for {
			select {
			case <-d.ctx.Done():
				return
			case <-ticker.C:
				d.broadcastSensorData()
			}
		}
	}()
}

// broadcastSensorData sends sensor data to all connected WebSocket clients.
func (d *DevServer) broadcastSensorData() {
	data := d.collector.CollectAll()

	msg, err := json.Marshal(data)
	if err != nil {
		return
	}

	d.wsClientsMu.RLock()
	defer d.wsClientsMu.RUnlock()

	for conn := range d.wsClients {
		if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			// Client will be cleaned up by the read goroutine
			continue
		}
	}
}

// findAvailablePort finds an available port starting from startPort.
func (d *DevServer) findAvailablePort(startPort, maxTries int) (int, error) {
	for i := 0; i < maxTries; i++ {
		port := startPort + i
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err != nil {
			continue
		}
		ln.Close()
		return port, nil
	}
	return 0, fmt.Errorf("no available port found in range %d-%d", startPort, startPort+maxTries-1)
}

// waitForVite waits for Vite to be ready.
func (d *DevServer) waitForVite(url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 1 * time.Second}

	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				return nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("timeout waiting for Vite at %s", url)
}
