package bk

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type httpclient struct {
	delay       time.Duration
	requests    int
	status      int
	retries     int
	attempts    int
	concurrency int
	cancel      bool
}

func (client *httpclient) Do(*http.Request) (*http.Response, error) {
	if client.delay > 0 {
		time.Sleep(client.delay)
	}

	status := client.status
	if client.retries > 0 {
		client.attempts++

		if client.attempts < client.retries {
			status = http.StatusBadRequest
		}
	}

	return &http.Response{
		StatusCode: status,
		Body:       &fakeReadCloser{},
	}, nil
}

type fakeReadCloser struct{}

func (rc *fakeReadCloser) Read([]byte) (int, error) { return 0, io.EOF }
func (rc *fakeReadCloser) Close() error             { return nil }

type badclient struct {
	panic       bool
	delay       time.Duration
	requests    int
	status      int
	retries     int
	attempts    int
	concurrency int
}

func (client *badclient) Do(*http.Request) (*http.Response, error) {
	if client.panic {
		panic("panic")
	}

	return nil, errors.New("error")
}

type tstruct struct {
	error bool
}

func (t *tstruct) correct(err error, _ bool) error {
	if t.error && err == nil {
		return errors.New("expected error but success instead")
	}

	if !t.error && err != nil {
		return fmt.Errorf("expected success but errored instead | %s", err)
	}

	return nil
}

func newGetReqWOutCtx() *http.Request {
	r, _ := http.NewRequest(
		http.MethodGet,
		"",
		strings.NewReader(""),
	)

	return r
}

func newGetReqWCtx() *http.Request {
	r, _ := http.NewRequestWithContext(
		context.TODO(),
		http.MethodGet,
		"",
		strings.NewReader(""),
	)

	return r
}

type passthrough struct {
	ctx context.Context
	out chan *http.Request
}

func (pass *passthrough) Do(r *http.Request) (*http.Response, error) {
	select {
	case <-pass.ctx.Done():
	case pass.out <- r:
	}

	return nil, nil
}
