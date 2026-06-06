package restate

import (
	"context"

	restate "github.com/restatedev/sdk-go"
	"github.com/restatedev/sdk-go/server"
)

// Server is a wrapper around the Restate Go SDK server.
type Server struct {
	rawServer *server.Restate
	config    *Config
}

// NewServer initializes a new Server instance using the provided Config.
func NewServer(cfg *Config) *Server {
	return &Server{
		rawServer: server.NewRestate(),
		config:    cfg,
	}
}

// Bind registers a service or virtual object struct in the server.
func (s *Server) Bind(service any) *Server {
	s.rawServer.Bind(restate.Reflect(service))
	return s
}

// Start starts the standalone HTTP/2 server on the configured address.
func (s *Server) Start(ctx context.Context) error {
	return s.rawServer.Start(ctx, s.config.HostPort)
}

// RawServer returns the underlying Restate instance.
func (s *Server) RawServer() *server.Restate {
	return s.rawServer
}
