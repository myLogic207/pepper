package base

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/myLogic207/gotils/config"
	log "github.com/myLogic207/gotils/logger"
	"golang.org/x/sync/errgroup"
)

var defaultServerConfig = map[string]interface{}{
	"LOGGER": map[string]interface{}{
		"PREFIX":      "SOCKET-SERVER",
		"COLUMLENGTH": 20,
	},
	"ADDRESS": "127.0.0.1",
	"PORT":    8080,
	"MAXCONN": "-1",
	"TIMEOUT": "5s",
	"TYPE":    "tcp",
}

type HandleFunc func(context.Context, net.Conn) error

type Server struct {
	Logger   log.Logger
	Config   *config.Config
	listener net.Listener
	stop     chan bool
}

// Listen starts listening on the configured address and port.
func (s *Server) Listen(ctx context.Context, serverOptions *config.Config) error {
	if s.Config == nil {
		cnf, err := config.WithInitialValuesAndOptions(ctx, defaultServerConfig, serverOptions)
		if err != nil {
			return fmt.Errorf("could not initialize config: %w", err)
		}
		s.Config = cnf
	}

	if s.Logger == nil {
		loggerConfig, _ := s.Config.GetConfig(ctx, "LOGGER")
		logger, err := log.Init(ctx, loggerConfig)
		if err != nil {
			return fmt.Errorf("could not initializing logger: %w", err)
		}
		s.Logger = logger
	}

	if err := s.initListener(ctx); err != nil {
		return fmt.Errorf("could not initialize listener: %w", err)
	}

	return nil
}

// Each connection is handled by a worker from the worker pool.
// The handleFunc is called for each connection that is accepted.
func (s *Server) Serve(ctx context.Context, handle HandleFunc) error {
	maxconn, _ := s.Config.Get(ctx, "MAXCONN")
	max, err := strconv.Atoi(maxconn)
	if err != nil {
		return fmt.Errorf("could not parse max connections: %s", maxconn)
	}
	connections, eCtx := errgroup.WithContext(ctx)
	connections.SetLimit(max)
	s.stop = make(chan bool)
	defer close(s.stop)

	s.Logger.Info(ctx, "Starting server")
	connections.Go(func() error {
		return s.acceptConnections(eCtx, connections, handle)
	})

	connections.Go(func() error {
	running:
		for {
			select {
			case <-s.stop:
				break running
			case <-eCtx.Done():
				if err := eCtx.Err(); err != nil && !errors.Is(err, context.Canceled) {
					s.Logger.Error(ctx, err.Error())
				}
				break running
			}
		}
		return s.listener.Close()
	})
	return connections.Wait()
}

func (s *Server) acceptConnections(ctx context.Context, connections *errgroup.Group, handle HandleFunc) error {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				s.Logger.Debug(ctx, "Accept timed out")
				continue
			} else if errors.Is(err, net.ErrClosed) {
				s.Logger.Debug(ctx, "Listener closed")
				return nil
			}
			s.Logger.Error(ctx, err.Error())
			return fmt.Errorf("could not accept connection: %w", err)
		}

		s.Logger.Info(ctx, "Accepted connection from %s", conn.RemoteAddr().String())
		connCtx := context.WithValue(ctx, "connection", conn)
		connected := connections.TryGo(func() error {
			if err := handle(connCtx, conn); err != nil {
				s.Logger.Error(connCtx, err.Error())
			}
			if err := conn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
				s.Logger.Error(ctx, err.Error())
			}
			return nil
		})
		if !connected {
			s.Logger.Error(connCtx, "Could not connect, would exceed max")
		}
	}
}

func (s *Server) GetListener() net.Listener {
	return s.listener
}

func (s *Server) initListener(ctx context.Context) error {
	addr, _ := s.Config.Get(ctx, "ADDRESS")
	port, _ := s.Config.Get(ctx, "PORT")
	address := fmt.Sprintf("%s:%s", addr, port)
	timeoutRaw, _ := s.Config.Get(ctx, "TIMEOUT")
	timeout, err := time.ParseDuration(timeoutRaw)
	if err != nil {
		return fmt.Errorf("could not parse timeout: %w", err)
	}
	listenConfig := net.ListenConfig{
		KeepAlive: timeout - (timeout / 10),
		Control:   nil,
	}
	connType, _ := s.Config.Get(ctx, "TYPE")
	listener, err := listenConfig.Listen(ctx, connType, address)
	if err != nil {
		return err
	}
	s.listener = listener
	s.Logger.Info(ctx, "Listening on %s", s.listener.Addr().String())
	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	s.Logger.Info(ctx, "Stopping server")
	s.stop <- true
	<-s.stop
	return nil
}
