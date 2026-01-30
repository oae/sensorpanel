// Package server provides HTTP server for theme rendering with WebSocket sensor data.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Server serves theme files and streams sensor data via WebSocket.
type Server struct {
	mu       sync.Mutex
	listener net.Listener
	server   *http.Server
	distDir  string

	// WebSocket connections
	clients   map[*websocket.Conn]bool
	clientsMu sync.RWMutex

	upgrader websocket.Upgrader
}

// New creates a new theme server.
func New(distDir string) *Server {
	return &Server{
		distDir: distDir,
		clients: make(map[*websocket.Conn]bool),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for local development
			},
		},
	}
}

// Start starts the server on a random available port.
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.listener != nil {
		return fmt.Errorf("server already running")
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	s.listener = listener

	mux := http.NewServeMux()

	// Serve static files from dist directory
	fs := http.FileServer(http.Dir(s.distDir))
	mux.Handle("/", fs)

	// WebSocket endpoint for sensor data
	mux.HandleFunc("/ws", s.handleWebSocket)

	s.server = &http.Server{
		Handler: mux,
	}

	go s.server.Serve(s.listener)
	return nil
}

// Port returns the port the server is listening on.
func (s *Server) Port() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.listener == nil {
		return 0
	}
	return s.listener.Addr().(*net.TCPAddr).Port
}

// URL returns the base URL of the server.
func (s *Server) URL() string {
	return fmt.Sprintf("http://127.0.0.1:%d", s.Port())
}

// Stop stops the server.
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Close all WebSocket clients
	s.clientsMu.Lock()
	for client := range s.clients {
		client.Close()
	}
	s.clients = make(map[*websocket.Conn]bool)
	s.clientsMu.Unlock()

	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		s.server.Shutdown(ctx)
		s.server = nil
	}

	if s.listener != nil {
		s.listener.Close()
		s.listener = nil
	}

	return nil
}

// handleWebSocket handles WebSocket connections for sensor data.
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	s.clientsMu.Lock()
	s.clients[conn] = true
	s.clientsMu.Unlock()

	// Keep connection alive, remove on close
	defer func() {
		s.clientsMu.Lock()
		delete(s.clients, conn)
		s.clientsMu.Unlock()
		conn.Close()
	}()

	// Read loop (to detect disconnection)
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

// BroadcastSensorData sends sensor data to all connected WebSocket clients.
func (s *Server) BroadcastSensorData(data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	// Use full lock since websocket.Conn.WriteMessage is not concurrent-safe
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()

	for client := range s.clients {
		err := client.WriteMessage(websocket.TextMessage, jsonData)
		if err != nil {
			// Client disconnected, will be cleaned up by read loop
			continue
		}
	}

	return nil
}

// ClientCount returns the number of connected WebSocket clients.
func (s *Server) ClientCount() int {
	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()
	return len(s.clients)
}
