package bk

import (
	"context"
	"net/http"
	"reflect"
	"testing"
	"time"
)

func Test_RoundTrip(t *testing.T) {
	alltests := map[string]map[string]tcase{
		"w/ctx-":    cases(t, newGetReqWCtx),
		"wout/ctx-": cases(t, newGetReqWOutCtx),
	}

	for key, tests := range alltests {
		for name, test := range tests {
			t.Run(key+name, func(t *testing.T) {
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("test [%s] had a panic | %s", name, r)
					}
				}()

				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				client := New(
					ctx,
					test.client.RoundTrip,
					test.client.delay,
					test.client.retries,
					test.client.concurrency,
					time.Minute,
				)

				// Cancellation test
				if test.client.cancel {
					cancel()
				}

				resp, err := client.RoundTrip(test.request)
				if err != nil {
					if test.client.retries > 0 &&
						test.client.retries != test.client.attempts &&
						!test.success.error {
						t.Fatalf("[%s] failed; number of attempts doesn't match the expected retries [%v:%v]", name, test.client.attempts, test.client.retries)
					} else {
						testErr := test.success.correct(err, false)
						if testErr != nil {
							t.Fatalf("[%s] failed; %s", name, testErr.Error())
						}
					}
				}

				if resp == nil {
					testErr := test.success.correct(err, false)
					if testErr != nil {
						t.Fatalf("[%s] failed; %s", name, testErr.Error())
					}
				}
			})
		}
	}
}

func Test_RoundTrip_BadClient(t *testing.T) {
	tests := map[string]struct {
		client  *badclient
		request *http.Request
		success tstruct
	}{
		"PanicyClient": {
			&badclient{
				panic:    true,
				requests: 1,
				status:   http.StatusOK,
			},
			newGetReqWCtx(),
			tstruct{true},
		},
		"ErroringClient": {
			&badclient{
				requests: 1,
				status:   http.StatusOK,
			},
			newGetReqWCtx(),
			tstruct{true},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("test [%s] had a panic | %s", name, r)
				}
			}()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			client := New(
				ctx,
				test.client.RoundTrip,
				test.client.delay,
				test.client.retries,
				test.client.concurrency,
				time.Minute,
			)

			resp, err := client.RoundTrip(test.request)
			if err != nil {
				if test.client.retries > 0 && test.client.retries != test.client.attempts && !test.success.error {
					t.Fatalf("[%s] failed; number of attempts doesn't match the expected retries [%v:%v]", name, test.client.attempts, test.client.retries)
				} else {
					testErr := test.success.correct(err, false)
					if testErr != nil {
						t.Fatalf("[%s] failed; %s", name, testErr.Error())
					}
				}
			}

			if resp == nil {
				testErr := test.success.correct(err, false)
				if testErr != nil {
					t.Fatalf("[%s] failed; %s", name, testErr.Error())
				}
			}
		})
	}
}

func Test_RoundTrip_FailOpen(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	code := http.StatusContinue
	client := &httpclient{
		status: code,
	}

	wrapper := &Keeper{
		ctx:               ctx,
		fn:                client.RoundTrip,
		cancel:            cancel,
		concurrencyticker: make(chan bool),
		requests:          make(chan *requestWrapper),
		requestTimeout:    time.Minute,
	}

	// cancel the context to trigger passthrough
	cancel()

	resp, err := wrapper.RoundTrip(newGetReqWCtx())
	if err != nil {
		t.Fatalf("error %s", err)
	}

	if resp.StatusCode != code {
		t.Fatalf("Expected status code %v got %v", code, resp.StatusCode)
	}
}

func Test_RoundTrip_Request_Timeout_ReqWOutCtx(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := &httpclient{
		delay: time.Minute,
	}
	client := New(
		ctx,
		c.RoundTrip,
		0,
		0,
		0,
		time.Second,
	)

	_, err := client.RoundTrip(newGetReqWOutCtx())
	if err != nil {
		if err != context.DeadlineExceeded {
			t.Fatalf("expected context.DeadlineExceeded; got %T", err)
		}
	} else {
		t.Fatal("Expected timeout error")
	}
}

func Test_RoundTrip_Request_Timeout_ReqWCtx(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := &httpclient{
		delay: time.Minute,
	}
	client := New(
		ctx,
		c.RoundTrip,
		0,
		0,
		0,
		time.Second,
	)

	_, err := client.RoundTrip(newGetReqWCtx())
	if err != nil {
		if err != context.DeadlineExceeded {
			t.Fatalf("expected context.DeadlineExceeded; got %T", err)
		}
	} else {
		t.Fatal("Expected timeout error")
	}
}

func Test_RoundTrip_Throughput(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p := &passthrough{
		ctx,
		make(chan *http.Request),
	}

	client := New(ctx, p.RoundTrip, 0, 0, 0, time.Minute)
	r := newGetReqWCtx()

	go func(r *http.Request) {
		client.RoundTrip(r)
	}(r)

	select {
	case <-ctx.Done():
		t.Fatal("context closed prematurely")
	case rout, ok := <-p.out:
		if !ok {
			t.Fatal("passthrough closed prematurely")
		}

		if !reflect.DeepEqual(r, rout) {
			t.Fatal("requests do not match")
		}
	}
}

func Benchmark_RoundTrip_ZeroConcurrency(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p := &passthrough{
		ctx,
		make(chan *http.Request),
	}

	client := New(ctx, p.RoundTrip, 0, 0, 0, time.Minute)
	r := newGetReqWCtx()

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		go func(r *http.Request) {
			client.RoundTrip(r)
		}(r)

		select {
		case <-ctx.Done():
			b.Fatal("context closed prematurely")
		case _, ok := <-p.out:
			if !ok {
				b.Fatal("passthrough closed prematurely")
			}
		}
	}
}
