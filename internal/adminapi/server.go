package adminapi

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	store "github.com/xsyetopz/go-mamacord/internal/storage"
)

const (
	sessionCookieName = "mamacord_admin_session"
	stateCookieName   = "mamacord_admin_state"
	sessionTTL        = 12 * time.Hour
)

type OAuthUser struct {
	ID         string `json:"id"`
	Username   string `json:"username"`
	GlobalName string `json:"global_name"`
	Avatar     string `json:"avatar"`
}

type OAuthClient interface {
	ExchangeCode(ctx context.Context, code string, redirectURL string) (OAuthToken, error)
	FetchUser(ctx context.Context, accessToken string) (OAuthUser, error)
	FetchGuilds(ctx context.Context, accessToken string) ([]OAuthGuild, error)
}

type Options struct {
	Addr          string
	Logger        *slog.Logger
	Service       Service
	SessionSecret string
	ClientID      string
	ClientSecret  string
	OwnerStatus   func() OwnerStatus
	OAuthClient   OAuthClient
	SessionStore  store.AdminSessionStore
}

type Server struct {
	logger *slog.Logger
	addr   string
	svc    *Service

	clientID     string
	clientSecret string
	ownerStatus  func() OwnerStatus
	oauth        OAuthClient
	secret       []byte

	sessions store.AdminSessionStore

	stateMu    sync.Mutex
	stateStore map[string]oauthState

	mu       sync.Mutex
	listener net.Listener
	server   *http.Server
}

type oauthState struct {
	RedirectURL string
	ReturnBase  string
}

type session struct {
	ID          string
	UserID      uint64
	Username    string
	Name        string
	AvatarURL   string
	CSRFToken   string
	AccessToken string
	IsOwner     bool
	ExpiresAt   int64
}

func New(opts Options) (*Server, error) {
	if strings.TrimSpace(opts.Addr) == "" {
		return nil, nil
	}
	if opts.Logger == nil {
		return nil, errors.New("logger is required")
	}
	if opts.OAuthClient == nil {
		opts.OAuthClient = NewDiscordOAuthClient(opts.ClientID, opts.ClientSecret)
	}
	sessionStore := opts.SessionStore
	if sessionStore == nil {
		sessionStore = newMemorySessionStore()
	}
	svc := opts.Service
	svc.init()
	return &Server{
		logger:       opts.Logger.With(slog.String("component", "admin_api")),
		addr:         strings.TrimSpace(opts.Addr),
		svc:          &svc,
		clientID:     strings.TrimSpace(opts.ClientID),
		clientSecret: strings.TrimSpace(opts.ClientSecret),
		ownerStatus:  opts.OwnerStatus,
		oauth:        opts.OAuthClient,
		secret:       []byte(opts.SessionSecret),
		sessions:     sessionStore,
		stateStore:   map[string]oauthState{},
	}, nil
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
	httpServer := &http.Server{
		Handler:           s.handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	s.listener = listener
	s.server = httpServer
	go func() {
		err := httpServer.Serve(listener)
		if err == nil || errors.Is(err, http.ErrServerClosed) {
			return
		}
		s.logger.Error("admin server stopped unexpectedly", slog.String("err", err.Error()))
	}()
	s.logger.Info("admin server listening", slog.String("addr", listener.Addr().String()))
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

type memorySessionStore struct {
	mu       sync.Mutex
	sessions map[string]store.AdminSession
}

func newMemorySessionStore() *memorySessionStore {
	return &memorySessionStore{sessions: map[string]store.AdminSession{}}
}

func (s *memorySessionStore) GetAdminSession(_ context.Context, id string) (store.AdminSession, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[id]
	return sess, ok, nil
}

func (s *memorySessionStore) PutAdminSession(_ context.Context, sess store.AdminSession) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[sess.ID] = sess
	return nil
}

func (s *memorySessionStore) DeleteAdminSession(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, id)
	return nil
}

func (s *memorySessionStore) DeleteExpiredAdminSessions(_ context.Context, nowUnix int64) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var n int64
	for id, sess := range s.sessions {
		if sess.ExpiresAt <= nowUnix {
			delete(s.sessions, id)
			n++
		}
	}
	return n, nil
}
