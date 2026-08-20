package adminapi

import (
	"context"
	"errors"
	adminauth "github.com/xsyetopz/go-mamacord/internal/adminapi/auth"
	adminguilds "github.com/xsyetopz/go-mamacord/internal/adminapi/guilds"
	adminplugins "github.com/xsyetopz/go-mamacord/internal/adminapi/plugins"
	adminservice "github.com/xsyetopz/go-mamacord/internal/adminapi/service"
	adminstatus "github.com/xsyetopz/go-mamacord/internal/adminapi/status"
	storage "github.com/xsyetopz/go-mamacord/internal/storage"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	sessionCookieName = "mamacord_admin_session"
	stateCookieName   = "mamacord_admin_state"
	sessionTTL        = 12 * time.Hour
)

type Options struct {
	Addr          string
	Logger        *slog.Logger
	Service       *adminservice.Service
	GuildService  *adminguilds.Service
	SessionSecret string
	ClientID      string
	ClientSecret  string
	OwnerStatus   func() adminservice.OwnerStatus
	OAuthClient   adminauth.OAuthClient
	SessionStore  storage.AdminSessionStore
}

type serverServices struct {
	logger  *slog.Logger
	svc     *adminservice.Service
	guilds  *adminguilds.Handler
	plugins *adminplugins.Handler
	status  *adminstatus.Handler
}

type serverAuth struct {
	clientID     string
	clientSecret string
	ownerStatus  func() adminservice.OwnerStatus
	oauth        adminauth.OAuthClient
	secret       []byte
	sessions     storage.AdminSessionStore
}

type oauthStates struct {
	stateMu    sync.Mutex
	stateStore map[string]oauthState
}

type httpRuntime struct {
	addr     string
	mu       sync.Mutex
	listener net.Listener
	server   *http.Server
}

type Server struct {
	serverServices
	serverAuth
	oauthStates
	httpRuntime
}

type oauthState struct {
	RedirectURL string
	ReturnBase  string
}

func New(opts Options) (*Server, error) {
	if strings.TrimSpace(opts.Addr) == "" {
		return nil, nil
	}
	if opts.Logger == nil {
		return nil, errors.New("logger is required")
	}
	if opts.OAuthClient == nil {
		opts.OAuthClient = adminauth.NewDiscordOAuthClient(opts.ClientID, opts.ClientSecret)
	}
	sessionStore := opts.SessionStore
	if sessionStore == nil {
		sessionStore = newMemorySessionStore()
	}
	svc := opts.Service
	if svc == nil {
		svc = &adminservice.Service{}
	}
	guildService := opts.GuildService
	if guildService == nil {
		guildService = &adminguilds.Service{}
	}
	if strings.TrimSpace(guildService.ClientID) == "" {
		guildService.ClientID = strings.TrimSpace(opts.ClientID)
	}
	if guildService.OAuth == nil {
		guildService.OAuth = opts.OAuthClient
	}
	guildService.Init()
	server := &Server{
		serverServices: serverServices{logger: opts.Logger.With(slog.String("component", "admin_api")), svc: svc},
		serverAuth: serverAuth{
			clientID: strings.TrimSpace(opts.ClientID), clientSecret: strings.TrimSpace(opts.ClientSecret),
			ownerStatus: opts.OwnerStatus, oauth: opts.OAuthClient, secret: []byte(opts.SessionSecret), sessions: sessionStore,
		},
		oauthStates: oauthStates{stateStore: map[string]oauthState{}},
		httpRuntime: httpRuntime{addr: strings.TrimSpace(opts.Addr)},
	}
	server.guilds = adminguilds.New(adminguilds.Options{
		Service: guildService, Logger: server.logger, Responder: httpResponder{},
		DashboardBaseURL: server.dashboardBaseURL, RequestBaseURL: requestBaseURL,
	})
	server.plugins = adminplugins.New(svc, server.logger, httpResponder{})
	server.status = adminstatus.New(svc, server.logger, httpResponder{})
	return server, nil
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
	sessions map[string]storage.AdminSession
}

func newMemorySessionStore() *memorySessionStore {
	return &memorySessionStore{sessions: map[string]storage.AdminSession{}}
}

func (s *memorySessionStore) GetAdminSession(_ context.Context, id string) (storage.AdminSession, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[id]
	return sess, ok, nil
}

func (s *memorySessionStore) PutAdminSession(_ context.Context, sess storage.AdminSession) error {
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
