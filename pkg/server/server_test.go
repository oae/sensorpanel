package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestNew(t *testing.T) {
	s := New("/some/path")
	if s == nil {
		t.Fatal("expected non-nil server")
	}
	if s.distDir != "/some/path" {
		t.Errorf("expected distDir '/some/path', got %q", s.distDir)
	}
	if s.clients == nil {
		t.Error("expected clients map to be initialized")
	}
}

func TestServer_StartAndStop(t *testing.T) {
	// Create a temp directory with an index.html
	tmpDir := t.TempDir()
	indexPath := filepath.Join(tmpDir, "index.html")
	if err := os.WriteFile(indexPath, []byte("<html>test</html>"), 0644); err != nil {
		t.Fatalf("failed to create index.html: %v", err)
	}

	s := New(tmpDir)

	// Start server
	if err := s.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}

	// Verify port is assigned
	port := s.Port()
	if port == 0 {
		t.Error("expected non-zero port")
	}

	// Verify URL is formatted correctly
	url := s.URL()
	if url == "" {
		t.Error("expected non-empty URL")
	}
	if !strings.HasPrefix(url, "http://127.0.0.1:") {
		t.Errorf("expected URL to start with 'http://127.0.0.1:', got %q", url)
	}

	// Test that we can make HTTP request
	resp, err := http.Get(url + "/index.html")
	if err != nil {
		t.Fatalf("failed to GET index.html: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	// Stop server
	if err := s.Stop(); err != nil {
		t.Fatalf("failed to stop server: %v", err)
	}

	// Verify port returns 0 after stop
	if s.Port() != 0 {
		t.Error("expected port 0 after stop")
	}
}

func TestServer_StartTwice(t *testing.T) {
	tmpDir := t.TempDir()
	s := New(tmpDir)

	if err := s.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer s.Stop()

	// Starting again should fail
	err := s.Start()
	if err == nil {
		t.Error("expected error when starting already running server")
	}
	if err != nil && !strings.Contains(err.Error(), "already running") {
		t.Errorf("expected 'already running' error, got: %v", err)
	}
}

func TestServer_PortBeforeStart(t *testing.T) {
	s := New("/tmp")
	if s.Port() != 0 {
		t.Errorf("expected port 0 before start, got %d", s.Port())
	}
}

func TestServer_StopWithoutStart(t *testing.T) {
	s := New("/tmp")
	// Stopping without starting should not panic or error
	if err := s.Stop(); err != nil {
		t.Errorf("unexpected error stopping unstarted server: %v", err)
	}
}

func TestServer_WebSocket(t *testing.T) {
	tmpDir := t.TempDir()
	s := New(tmpDir)

	if err := s.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer s.Stop()

	// Initially no clients
	if s.ClientCount() != 0 {
		t.Errorf("expected 0 clients, got %d", s.ClientCount())
	}

	// Connect WebSocket client
	wsURL := strings.Replace(s.URL(), "http://", "ws://", 1) + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect WebSocket: %v", err)
	}

	// Give server time to register connection
	time.Sleep(50 * time.Millisecond)

	// Now should have 1 client
	if s.ClientCount() != 1 {
		t.Errorf("expected 1 client, got %d", s.ClientCount())
	}

	// Close connection
	conn.Close()

	// Give server time to clean up
	time.Sleep(50 * time.Millisecond)

	// Back to 0 clients
	if s.ClientCount() != 0 {
		t.Errorf("expected 0 clients after disconnect, got %d", s.ClientCount())
	}
}

func TestServer_BroadcastSensorData(t *testing.T) {
	tmpDir := t.TempDir()
	s := New(tmpDir)

	if err := s.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer s.Stop()

	// Connect WebSocket client
	wsURL := strings.Replace(s.URL(), "http://", "ws://", 1) + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect WebSocket: %v", err)
	}
	defer conn.Close()

	// Give server time to register connection
	time.Sleep(50 * time.Millisecond)

	// Broadcast some data
	testData := map[string]interface{}{
		"cpu":    75.5,
		"memory": 60.2,
	}
	if err := s.BroadcastSensorData(testData); err != nil {
		t.Fatalf("failed to broadcast: %v", err)
	}

	// Read the message
	conn.SetReadDeadline(time.Now().Add(time.Second))
	_, message, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read WebSocket message: %v", err)
	}

	// Verify message contains expected data
	msgStr := string(message)
	if !strings.Contains(msgStr, "cpu") {
		t.Errorf("expected message to contain 'cpu', got %q", msgStr)
	}
	if !strings.Contains(msgStr, "75.5") {
		t.Errorf("expected message to contain '75.5', got %q", msgStr)
	}
}

func TestServer_BroadcastNoClients(t *testing.T) {
	tmpDir := t.TempDir()
	s := New(tmpDir)

	if err := s.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer s.Stop()

	// Broadcast with no clients should not error
	testData := map[string]string{"test": "data"}
	if err := s.BroadcastSensorData(testData); err != nil {
		t.Errorf("unexpected error broadcasting to no clients: %v", err)
	}
}

func TestServer_BroadcastInvalidData(t *testing.T) {
	tmpDir := t.TempDir()
	s := New(tmpDir)

	if err := s.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer s.Stop()

	// Broadcast unmarshalable data (channel can't be JSON marshaled)
	invalidData := make(chan int)
	err := s.BroadcastSensorData(invalidData)
	if err == nil {
		t.Error("expected error when broadcasting unmarshalable data")
	}
}

func TestServer_MultipleClients(t *testing.T) {
	tmpDir := t.TempDir()
	s := New(tmpDir)

	if err := s.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer s.Stop()

	wsURL := strings.Replace(s.URL(), "http://", "ws://", 1) + "/ws"

	// Connect 3 clients
	var conns []*websocket.Conn
	for i := 0; i < 3; i++ {
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("failed to connect WebSocket client %d: %v", i, err)
		}
		conns = append(conns, conn)
	}
	defer func() {
		for _, c := range conns {
			c.Close()
		}
	}()

	// Give server time to register connections
	time.Sleep(100 * time.Millisecond)

	if s.ClientCount() != 3 {
		t.Errorf("expected 3 clients, got %d", s.ClientCount())
	}

	// Broadcast data
	testData := map[string]string{"message": "hello all"}
	if err := s.BroadcastSensorData(testData); err != nil {
		t.Fatalf("failed to broadcast: %v", err)
	}

	// All clients should receive the message
	for i, conn := range conns {
		conn.SetReadDeadline(time.Now().Add(time.Second))
		_, message, err := conn.ReadMessage()
		if err != nil {
			t.Errorf("client %d failed to read message: %v", i, err)
			continue
		}
		if !strings.Contains(string(message), "hello all") {
			t.Errorf("client %d: expected message to contain 'hello all', got %q", i, message)
		}
	}
}

func TestServer_ConcurrentBroadcast(t *testing.T) {
	tmpDir := t.TempDir()
	s := New(tmpDir)

	if err := s.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer s.Stop()

	wsURL := strings.Replace(s.URL(), "http://", "ws://", 1) + "/ws"

	// Connect a client
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect WebSocket: %v", err)
	}
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	// Broadcast concurrently from multiple goroutines
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			data := map[string]int{"iteration": n}
			s.BroadcastSensorData(data)
		}(i)
	}
	wg.Wait()

	// Read all messages
	messagesReceived := 0
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	for messagesReceived < 10 {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
		messagesReceived++
	}

	if messagesReceived < 10 {
		t.Errorf("expected 10 messages, received %d", messagesReceived)
	}
}

func TestServer_StaticFileServing(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	files := map[string]string{
		"index.html":      "<html><body>Index</body></html>",
		"style.css":       "body { color: red; }",
		"app.js":          "console.log('hello');",
		"assets/logo.txt": "logo placeholder",
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create dir %s: %v", dir, err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write %s: %v", path, err)
		}
	}

	s := New(tmpDir)
	if err := s.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer s.Stop()

	// Test each file
	testCases := []struct {
		path            string
		expectedContent string
	}{
		{"/index.html", "Index"},
		{"/style.css", "color: red"},
		{"/app.js", "console.log"},
		{"/assets/logo.txt", "logo placeholder"},
	}

	for _, tc := range testCases {
		t.Run(tc.path, func(t *testing.T) {
			resp, err := http.Get(s.URL() + tc.path)
			if err != nil {
				t.Fatalf("failed to GET %s: %v", tc.path, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("expected status 200 for %s, got %d", tc.path, resp.StatusCode)
			}
		})
	}

	// Test 404 for missing file
	resp, err := http.Get(s.URL() + "/nonexistent.txt")
	if err != nil {
		t.Fatalf("failed to GET nonexistent file: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status 404 for missing file, got %d", resp.StatusCode)
	}
}

func TestServer_Upgrader_CheckOrigin(t *testing.T) {
	s := New("/tmp")

	// The upgrader should accept any origin
	req := httptest.NewRequest("GET", "/ws", nil)
	req.Header.Set("Origin", "http://different-origin.com")

	if !s.upgrader.CheckOrigin(req) {
		t.Error("expected CheckOrigin to return true for any origin")
	}
}

func TestServer_ClientCount_Concurrent(t *testing.T) {
	tmpDir := t.TempDir()
	s := New(tmpDir)

	if err := s.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer s.Stop()

	wsURL := strings.Replace(s.URL(), "http://", "ws://", 1) + "/ws"

	// Connect/disconnect clients concurrently while reading client count
	var wg sync.WaitGroup
	done := make(chan bool)

	// Reader goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-done:
				return
			default:
				_ = s.ClientCount() // Just ensure no race
			}
		}
	}()

	// Connect/disconnect goroutines
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
			if err != nil {
				return
			}
			time.Sleep(10 * time.Millisecond)
			conn.Close()
		}()
	}

	time.Sleep(200 * time.Millisecond)
	close(done)
	wg.Wait()
}

func TestServer_StopClosesClients(t *testing.T) {
	tmpDir := t.TempDir()
	s := New(tmpDir)

	if err := s.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}

	wsURL := strings.Replace(s.URL(), "http://", "ws://", 1) + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect WebSocket: %v", err)
	}

	time.Sleep(50 * time.Millisecond)
	if s.ClientCount() != 1 {
		t.Errorf("expected 1 client before stop, got %d", s.ClientCount())
	}

	// Stop server - should close all clients
	if err := s.Stop(); err != nil {
		t.Fatalf("failed to stop server: %v", err)
	}

	// Try to read from connection - should fail
	conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	_, _, err = conn.ReadMessage()
	if err == nil {
		t.Error("expected error reading from closed connection")
	}
}

func TestServer_URLFormat(t *testing.T) {
	s := New("/tmp")

	// Before start, port is 0, but URL still formats
	url := s.URL()
	if url != "http://127.0.0.1:0" {
		t.Errorf("expected 'http://127.0.0.1:0' before start, got %q", url)
	}

	if err := s.Start(); err != nil {
		t.Fatalf("failed to start: %v", err)
	}
	defer s.Stop()

	url = s.URL()
	if !strings.HasPrefix(url, "http://127.0.0.1:") {
		t.Errorf("expected URL starting with 'http://127.0.0.1:', got %q", url)
	}
	// Port should not be 0 now
	if strings.HasSuffix(url, ":0") {
		t.Errorf("expected non-zero port in URL, got %q", url)
	}
}
