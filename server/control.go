package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"relay/config"
)

// StatusResponse is returned by GET /status.
type StatusResponse struct {
	Status       string   `json:"status"`
	PID          int      `json:"pid"`
	Uptime       string   `json:"uptime"`
	WorkingDir   string   `json:"working_dir"`
	ShellType    string   `json:"shell_type"`
	AllowedUsers []int64  `json:"allowed_users"`
	Version      string   `json:"version"`
}

// ControlServer handles local CLI commands (status, reload, stop) over HTTP.
type ControlServer struct {
	cfg        *config.Config
	startTime  time.Time
	httpServer *http.Server
	shutdownCh chan os.Signal
	actualPort int
}

// NewControlServer initializes the local loopback IPC server.
func NewControlServer(cfg *config.Config, shutdownCh chan os.Signal) *ControlServer {
	return &ControlServer{
		cfg:        cfg,
		startTime:  time.Now(),
		shutdownCh: shutdownCh,
	}
}

// Start binds to 127.0.0.1 and begins serving requests.
func (s *ControlServer) Start() (int, error) {
	snap := s.cfg.Get()
	addr := fmt.Sprintf("127.0.0.1:%d", snap.ControlPort)

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		// If configured port is busy, fallback to random available loopback port
		listener, err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return 0, fmt.Errorf("failed to bind local control server: %w", err)
		}
	}

	s.actualPort = listener.Addr().(*net.TCPAddr).Port

	mux := http.NewServeMux()
	mux.HandleFunc("/status", s.handleStatus)
	mux.HandleFunc("/reload", s.handleReload)
	mux.HandleFunc("/stop", s.handleStop)

	s.httpServer = &http.Server{
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	go func() {
		if err := s.httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Printf("[ERROR] Control server failed: %v", err)
		}
	}()

	return s.actualPort, nil
}

// Stop gracefully shuts down the control server.
func (s *ControlServer) Stop(ctx context.Context) error {
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

func (s *ControlServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	snap := s.cfg.Get()
	var users []int64
	for u := range snap.AllowedUserIDs {
		users = append(users, u)
	}

	resp := StatusResponse{
		Status:       "running",
		PID:          os.Getpid(),
		Uptime:       time.Since(s.startTime).Round(time.Second).String(),
		WorkingDir:   snap.WorkingDir,
		ShellType:    snap.ShellType,
		AllowedUsers: users,
		Version:      "1.0.0",
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *ControlServer) handleReload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := s.cfg.Reload(); err != nil {
		http.Error(w, fmt.Sprintf("Reload failed: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("[INFO] Configuration reloaded via CLI IPC")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"message": "Configuration reloaded successfully",
	})
}

func (s *ControlServer) handleStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	log.Printf("[INFO] Shutdown requested via CLI IPC")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"message": "Shutting down relay daemon...",
	})

	go func() {
		time.Sleep(200 * time.Millisecond)
		if s.shutdownCh != nil {
			s.shutdownCh <- os.Interrupt
		}
	}()
}
