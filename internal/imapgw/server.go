package imapgw

import (
	"crypto/tls"
	"fmt"
	"net"
	"strings"
)

type ServerOptions struct {
	Addr              string
	Backend           Backend
	TLSConfig         *tls.Config
	AllowInsecureAuth bool
}

type Server struct {
	options ServerOptions
}

func NewServer(opts ServerOptions) (*Server, error) {
	addr := strings.TrimSpace(opts.Addr)
	if addr == "" {
		return nil, fmt.Errorf("imap server address is required")
	}
	if strings.ContainsAny(addr, "\r\n") {
		return nil, fmt.Errorf("imap server address cannot contain line breaks")
	}
	if _, _, err := net.SplitHostPort(addr); err != nil {
		return nil, fmt.Errorf("imap server address must be a TCP host:port address: %w", err)
	}
	if opts.Backend == nil {
		return nil, fmt.Errorf("imap backend is required")
	}
	if !opts.AllowInsecureAuth && opts.TLSConfig == nil {
		return nil, fmt.Errorf("imap TLS config is required when insecure auth is disabled")
	}
	opts.Addr = addr
	return &Server{options: opts}, nil
}

func (s *Server) Options() ServerOptions {
	if s == nil {
		return ServerOptions{}
	}
	return s.options
}
