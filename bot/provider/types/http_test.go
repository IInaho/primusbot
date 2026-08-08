package types

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

const streamReturnTimeout = 2 * time.Second

type blockingStreamBody struct {
	reader    *strings.Reader
	closed    chan struct{}
	closeOnce sync.Once
}

func (b *blockingStreamBody) Read(p []byte) (int, error) {
	if b.reader.Len() > 0 {
		return b.reader.Read(p)
	}
	<-b.closed
	return 0, io.EOF
}

func (b *blockingStreamBody) Close() error {
	b.closeOnce.Do(func() { close(b.closed) })
	return nil
}

func TestStreamSSEStopsAtDoneWithoutWaitingForEOF(t *testing.T) {
	body := &blockingStreamBody{
		reader: strings.NewReader("data: payload\n\ndata: [DONE]\n\n"),
		closed: make(chan struct{}),
	}
	resp := &http.Response{StatusCode: http.StatusOK, Body: body}
	errCh := make(chan error, 1)
	returned := make(chan struct{})
	parsed := 0

	go func() {
		StreamSSE(context.Background(), resp, make(chan StreamToken), errCh,
			func(data string, _ chan<- StreamToken) error {
				parsed++
				return nil
			})
		close(returned)
	}()

	select {
	case <-returned:
	case <-time.After(streamReturnTimeout):
		_ = body.Close()
		t.Fatal("StreamSSE waited for EOF after receiving [DONE]")
	}
	if parsed != 1 {
		t.Fatalf("parsed events = %d, want 1", parsed)
	}
	select {
	case err := <-errCh:
		t.Fatalf("unexpected stream error: %v", err)
	default:
	}
}

func TestStreamSSEStopsOnProviderTerminalEvent(t *testing.T) {
	body := &blockingStreamBody{
		reader: strings.NewReader("data: message-stop\n\n"),
		closed: make(chan struct{}),
	}
	resp := &http.Response{StatusCode: http.StatusOK, Body: body}
	returned := make(chan struct{})

	go func() {
		StreamSSE(context.Background(), resp, make(chan StreamToken), make(chan error, 1),
			func(string, chan<- StreamToken) error { return ErrStreamDone })
		close(returned)
	}()

	select {
	case <-returned:
	case <-time.After(streamReturnTimeout):
		_ = body.Close()
		t.Fatal("StreamSSE waited for EOF after a provider terminal event")
	}
}
