package base

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/myLogic207/gotils/config"
	"github.com/stretchr/testify/assert"
)

var testMsg = []byte("Hello World")

const (
	tADDRESS = "127.0.0.1"
	tPORT    = "0"
)

func testHandle(ctx context.Context, conn net.Conn) error {
	println("Handling connection")
	if i, err := conn.Write(testMsg); err != nil {
		println("Error writing to connection: ", err)
		return err
	} else if i != len(testMsg) {
		println("Wrote wrong amount of bytes: ", i, " expected: ", len(testMsg))
		return errors.New("Wrote wrong amount of bytes")
	}
	println("Finished handling connection")
	return nil
}

func TestServe(t *testing.T) {
	ctx := context.Background()
	conf, err := config.WithInitialValues(ctx, map[string]interface{}{
		"ADDRESS": tADDRESS,
		"PORT":    tPORT,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Log(conf.Sprint())
	server := &Server{}
	if err := server.Listen(ctx, conf); err != nil {
		t.Fatal(err)
	}

	go func() {
		if err := server.Serve(ctx, testHandle); err != nil {
			panic(err)
		}
		println("Server stopped")
	}()

	<-time.After(1 * time.Millisecond)
	// stop signal
	if err := server.Stop(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestConnect(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	conf, err := config.WithInitialValues(ctx, map[string]interface{}{
		"ADDRESS": tADDRESS,
		"PORT":    tPORT,
		"WORKERS": 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	server := &Server{}

	if err := server.Listen(ctx, conf); err != nil {
		t.Fatal(err)
	}
	defer cancel()

	go func() {
		if err := server.Serve(ctx, testHandle); err != nil {
			panic(err)
		}
	}()

	conn, err := net.Dial("tcp", server.GetListener().Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	<-time.After(1 * time.Millisecond)

	buff, err := io.ReadAll(conn)
	if err != nil {
		t.Fatal(err)
	}

	if assert.Equal(t, testMsg, buff) {
		t.Log("Test finished successfully")
	} else {
		t.Fatal("Test failed")
	}
}
