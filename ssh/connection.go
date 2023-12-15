package ssh

import (
	"context"

	"golang.org/x/crypto/ssh"
)

type contextKey string

const contextKeyChannelID = contextKey("channel-id")

// Work blocks until the connection is served.
func (s *SSHServer) Connection(ctx context.Context, sshConn ssh.Conn, chans <-chan ssh.NewChannel) error {
	chanCounter := 0
	for newChannel := range chans {
		chanCtx := context.WithValue(ctx, contextKeyChannelID, chanCounter)
		handler, ok := s.ChannelHandlers[newChannel.ChannelType()]
		if !ok {
			newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
			continue
		}
		chanCounter++
		if err := handler(chanCtx, newChannel); err != nil {
			return err
		}
	}
	return nil
}

func (s *SSHServer) DefaultSessionHandler(ctx context.Context, channel ssh.NewChannel) error {
	accepted, requests, err := channel.Accept()
	if err != nil {
		return err
	}
	for req := range requests {
		requestHandler, ok := s.RequestHandlers[req.Type]
		if !ok {
			requestHandler = s.RequestHandlers["default"]
		}
		go requestHandler(ctx, accepted, req)
	}
	return nil
}
