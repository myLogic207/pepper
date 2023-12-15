package base

import (
	"context"
	"net"

	log "github.com/myLogic207/gotils/logger"
)

type HandleFunc func(context.Context, net.Conn, ...interface{}) error

type Task struct {
	logger log.Logger
	conn   net.Conn
	Args   []interface{}
	Handle HandleFunc
}

func NewBase(handleFunc HandleFunc, logger log.Logger) Task {
	return Task{
		logger: logger,
		Handle: handleFunc,
	}
}

func NewTask(baseTask Task, conn net.Conn, callArgs ...interface{}) *Task {
	baseTask.conn = conn
	baseTask.Args = callArgs
	return &baseTask
}

func (t *Task) Do(ctx context.Context) error {
	return t.Handle(ctx, t.conn, t.Args...)
}

func (t *Task) OnFinish(ctx context.Context, err error) {
	if err != nil {
		t.logger.Error(ctx, err.Error())
	}

	if err := t.conn.Close(); err != nil {
		t.logger.Error(ctx, err.Error())
	}
	t.logger.Debug(ctx, "Connection from %s closed", t.conn.RemoteAddr().String())
}
