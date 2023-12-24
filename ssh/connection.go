package ssh

import (
	"context"
	"sync"

	"golang.org/x/crypto/ssh"
)

type contextKey string

const contextKeyChannelID = contextKey("channel-id")

// Work blocks until the connection is served.
func (s *Server) WorkConnect(ctx context.Context, sshConn ssh.Conn, chans <-chan ssh.NewChannel) error {
	chanCounter := 0
	waitGroup := sync.WaitGroup{}
	for newChannel := range chans {
		chanCtx := context.WithValue(ctx, contextKeyChannelID, chanCounter)
		handler, ok := s.ChannelHandlers[newChannel.ChannelType()]
		if !ok {
			handler = s.DefaultChannelHandler
		}
		chanCounter++
		waitGroup.Add(1)
		go func(channel ssh.NewChannel) {
			if err := handler(chanCtx, channel); err != nil {
				s.logger.Error(chanCtx, "Error handling channel %s", err)
			}
			waitGroup.Done()
		}(newChannel)
	}
	waitGroup.Wait()
	return nil
}

func (s *Server) DefaultChannelHandler(ctx context.Context, channel ssh.NewChannel) error {
	accepted, requests, err := channel.Accept()
	if err != nil {
		return err
	}
	waitGroup := sync.WaitGroup{}
	for req := range requests {
		requestHandler, ok := s.RequestHandlers[req.Type]
		if !ok {
			requestHandler = s.DefaultRequestHandler
		}
		waitGroup.Add(1)
		go func(r *ssh.Request) {
			if err := requestHandler(ctx, accepted, r); err != nil {
				s.logger.Error(ctx, "Error handling request %s", err)
			}
			waitGroup.Done()
		}(req)
	}
	waitGroup.Wait()
	return nil
}

func (s *Server) DefaultRequestHandler(ctx context.Context, channel ssh.Channel, req *ssh.Request) error {
	s.logger.Debug(ctx, "Request %s", req.Type)
	s.logger.Debug(ctx, "Request Payload %s", req.Payload)
	s.logger.Debug(ctx, "Request WantReply %v", req.WantReply)

	req.Reply(false, nil)
	return nil
}
