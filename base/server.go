package base

import (
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/myLogic207/gotils/config"
	log "github.com/myLogic207/gotils/logger"
	"github.com/myLogic207/gotils/workers"
)

var defaultServerConfig = map[string]interface{}{
	"LOGGER": map[string]interface{}{
		"PREFIX":       "SOCKET-SERVER",
		"PREFIXLENGTH": 20,
	},
	"ADDRESS": "127.0.0.1",
	"PORT":    8080,
	"WORKERS": 100,
	"TIMEOUT": "5s",
	"TYPE":    "tcp",
}

type Server struct {
	Logger   log.Logger
	Config   config.Config
	listener net.Listener
	workers  workers.WorkerPool
	pause    chan bool
	handle   Task
}

func NewServer(serverOptions config.Config, handle HandleFunc) (*Server, error) {
	cnf := config.NewWithInitialValues(defaultServerConfig)
	if err := cnf.Merge(serverOptions, true); err != nil {
		return nil, err
	}
	if err := cnf.HasAllKeys(defaultServerConfig); err != nil {
		return nil, err
	}

	loggerConfig, _ := cnf.GetConfig("LOGGER")
	logger, err := log.NewLogger(loggerConfig)
	if err != nil {
		return nil, err
	}

	server := &Server{
		Config: cnf,
		Logger: logger,
		pause:  make(chan bool),
		handle: NewBase(handle, logger),
	}

	return server, nil
}

// Listen starts listening on the configured address and port.
func (s *Server) Listen(ctx context.Context) error {
	lCtx, cancel := context.WithCancelCause(ctx)
	size, _ := s.Config.GetInt("WORKERS")
	pool, err := workers.InitPool(lCtx, size, s.Logger)
	if err != nil {
		cancel(err)
		return err
	}
	s.workers = pool

	if err := s.initListener(lCtx); err != nil {
		return err
	}

	go func() {
		for {
			<-ctx.Done()
			if err := s.Stop(ctx); err != nil {
				panic(err)
			}
		}
	}()

	return nil
}

func (s *Server) SetHandle(handle HandleFunc) {
	s.handle = NewBase(handle, s.Logger)
}

// Each connection is handled by a worker from the worker pool.
// The handleFunc is called for each connection that is accepted.
func (s *Server) Serve(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			if err := ctx.Err(); err != nil && !errors.Is(err, context.Canceled) {
				s.Logger.Error(ctx, err.Error())
				return err
			}
			return nil
		default:
			conn, err := s.listener.Accept()
			if err != nil {
				if ne, ok := err.(net.Error); ok && ne.Timeout() {
					s.Logger.Debug(ctx, "Accept timed out")
					continue
				} else if errors.Is(err, net.ErrClosed) {
					s.Logger.Debug(ctx, "Listener closed")
					return nil
				}
				return err
			}
			s.Logger.Info(ctx, "Accepted connection from %s", conn.RemoteAddr().String())

			worker := NewTask(s.handle, conn)
			if err := s.AddWorker(ctx, worker); err != nil {
				s.Logger.Error(ctx, err.Error())
				return err
			}
		}
	}
}

func (s *Server) AddWorker(ctx context.Context, task workers.Task) error {
	return s.workers.Add(ctx, task)
}

func (s *Server) GetListener() net.Listener {
	return s.listener
}

func (s *Server) initListener(ctx context.Context) error {
	addr, _ := s.Config.GetString("ADDRESS")
	port, _ := s.Config.GetInt("PORT")
	address := fmt.Sprintf("%s:%d", addr, port)
	timeout, _ := s.Config.GetDuration("TIMEOUT")
	listenConfig := net.ListenConfig{
		KeepAlive: timeout - (timeout / 10),
		Control:   nil,
	}
	connType, _ := s.Config.GetString("TYPE")
	listener, err := listenConfig.Listen(ctx, connType, address)
	if err != nil {
		return err
	}
	s.Logger.Info(ctx, "Listening on %s", address)
	s.listener = listener
	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	if err := ctx.Err(); err != nil && !errors.Is(err, context.Canceled) {
		s.Logger.Error(ctx, err.Error())
	}

	if err := s.listener.Close(); err != nil {
		s.Logger.Error(ctx, err.Error())
	}

	s.workers.Stop(ctx)

	if err := s.Logger.Shutdown(); err != nil {
		return err
	}
	return nil
}
