package ops

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Server struct {
	logger   *slog.Logger
	addr     string
	snapshot SnapshotFunc

	mu       sync.Mutex
	listener net.Listener
	server   *http.Server
}

func New(addr string, logger *slog.Logger, snapshot SnapshotFunc) (*Server, error) {
	if strings.TrimSpace(addr) == "" {
		return nil, nil
	}
	return &Server{
		logger:   logger,
		addr:     strings.TrimSpace(addr),
		snapshot: snapshot,
	}, nil
}

func NewHandler(snapshot SnapshotFunc) http.Handler {
	mux := http.NewServeMux()
	readSnapshot := func() Snapshot {
		if snapshot == nil {
			return Snapshot{}
		}
		return snapshot()
	}

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})

	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		if readSnapshot().Runtime.Ready {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ready\n"))
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("not ready\n"))
	})

	mux.HandleFunc("/metrics", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		snap := readSnapshot()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(RenderPrometheus(snap, time.Now().UTC())))
	})

	return mux
}

func (s *Server) Start() error {
	if s == nil || s.addr == "" {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.server != nil {
		return nil
	}

	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}

	server := &http.Server{
		Handler:           NewHandler(s.snapshot),
		ReadHeaderTimeout: 5 * time.Second,
	}

	s.listener = listener
	s.server = server

	go func() {
		err := server.Serve(listener)
		if err == nil || errors.Is(err, http.ErrServerClosed) {
			return
		}
		if s.logger != nil {
			s.logger.Error("ops server stopped unexpectedly", slog.String("err", err.Error()))
		}
	}()

	if s.logger != nil {
		s.logger.Info("ops server listening", slog.String("addr", listener.Addr().String()))
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s == nil {
		return nil
	}

	s.mu.Lock()
	server := s.server
	s.server = nil
	s.listener = nil
	s.mu.Unlock()

	if server == nil {
		return nil
	}
	return server.Shutdown(ctx)
}

func (s *Server) Close(ctx context.Context) error {
	return s.Shutdown(ctx)
}
