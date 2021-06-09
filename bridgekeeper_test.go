package bridgekeeper

import (
	"context"
	"net/http"
	"reflect"
	"testing"
	"time"
)

type tcase struct {
	client  *httpclient
	request *http.Request
	success tstruct
}

func cases(t *testing.T, req func() *http.Request) map[string]tcase {
	return map[string]tcase{
		"ValidWValidClient": {
			&httpclient{
				requests: 1,
				status:   http.StatusOK,
			},
			req(),
			tstruct{false},
		},
		"ValidWValidClientConcurrency": {
			&httpclient{
				requests:    1,
				status:      http.StatusOK,
				concurrency: 10,
			},
			req(),
			tstruct{false},
		},
		"ValidWValidClientConcurrency_0_Default": {
			&httpclient{
				requests:    1,
				status:      http.StatusOK,
				concurrency: 0,
			},
			req(),
			tstruct{false},
		},
		"ValidWValidClientDelay": {
			&httpclient{
				delay:    time.Millisecond * 25,
				requests: 1,
				status:   http.StatusOK,
			},
			req(),
			tstruct{false},
		},
		"ValidWValidClientDelay_0": {
			&httpclient{
				delay:    0,
				requests: 1,
				status:   http.StatusOK,
			},
			req(),
			tstruct{false},
		},
		"ValidWValidClientDelay_neg": {
			&httpclient{
				delay:    -1,
				requests: 1,
				status:   http.StatusOK,
			},
			req(),
			tstruct{false},
		},
		"ValidWValidClientDelayAndConcurrency": {
			&httpclient{
				delay:       time.Millisecond * 25,
				requests:    1,
				status:      http.StatusOK,
				concurrency: 10,
			},
			req(),
			tstruct{false},
		},
		"ValidWValidClientW5Retries": {
			&httpclient{
				requests: 1,
				status:   http.StatusOK,
				retries:  5,
			},
			req(),
			tstruct{false},
		},
		"ValidWValidClientW0Retries": {
			&httpclient{
				requests: 1,
				status:   http.StatusOK,
				retries:  0,
			},
			req(),
			tstruct{false},
		},
		"FailWValidClientW5Retries4Attempts": {
			&httpclient{
				requests: 1,
				status:   http.StatusOK,
				retries:  5,
				attempts: -1,
			},
			req(),
			tstruct{true},
		},
		"FailWBadStatus": {
			&httpclient{
				requests: 1,
				status:   http.StatusBadRequest,
			},
			req(),
			tstruct{true},
		},
		"FailByCancellation": {
			&httpclient{
				requests: 1,
				status:   http.StatusOK,
				cancel:   true,
			},
			req(),
			tstruct{true},
		},
		"FailByCancellation_Millisecond": {
			&httpclient{
				delay:    time.Millisecond,
				requests: 1,
				status:   http.StatusOK,
				cancel:   true,
			},
			req(),
			tstruct{true},
		},
		"FailByNilRequest": {
			&httpclient{
				requests: 1,
				status:   http.StatusOK,
				cancel:   true,
			},
			nil,
			tstruct{true},
		},
	}
}

func Test_Do(t *testing.T) {
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
					test.client,
					test.client.delay,
					test.client.retries,
					test.client.concurrency,
					time.Minute,
				)

				// Cancellation test
				if test.client.cancel {
					cancel()
				}

				resp, err := client.Do(test.request)
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

func Test_DoBadClient(t *testing.T) {
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
				test.client,
				test.client.delay,
				test.client.retries,
				test.client.concurrency,
				time.Minute,
			)

			resp, err := client.Do(test.request)
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

func Test_Do_FailOpen(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	code := http.StatusContinue

	wrapper := &keeper{
		ctx:    ctx,
		cancel: cancel,
		client: &httpclient{
			status: code,
		},
		concurrencyticker: make(chan bool),
		requests:          make(chan *requestWrapper),
		requestTimeout:    time.Minute,
	}

	// cancel the context to trigger passthrough
	cancel()

	resp, err := wrapper.Do(newGetReqWCtx())
	if err != nil {
		t.Fatalf("error %s", err)
	}

	if resp.StatusCode != code {
		t.Fatalf("Expected status code %v got %v", code, resp.StatusCode)
	}
}

func Test_New_Defaults(t *testing.T) {
	// Setting the value for default http timeout
	http.DefaultClient.Timeout = time.Millisecond

	client := New(nil, nil, -1, -1, -1, -1)

	k, ok := client.(*keeper)
	if !ok {
		t.Fatalf("Invalid client type")
	}

	if k.requestTimeout != http.DefaultClient.Timeout {
		t.Fatal("Expected request timeout to default to http.DefaultClient.Timeout")
	}

	if k.retries != 0 {
		t.Fatalf("Expected retries to be 0; got %v", k.retries)
	}

	if k.delay != time.Nanosecond {
		t.Fatalf("Expected delay to be %v; got %v", time.Nanosecond, k.delay)
	}

	if k.concurrency != 1 {
		t.Fatalf("Expected concurrency 1; got %v", k.concurrency)
	}

	if k.ctx == nil || k.cancel == nil {
		t.Fatalf(
			"Invalid context fallback ctx: %v cancelfunc: %v",
			k.ctx,
			k.cancel,
		)
	}

	if k.client != http.DefaultClient {
		t.Fatal("Expected client to be http.DefaultClient")
	}

	if k.ticker == nil {
		t.Fatal("Nil keeper ticker")
	}

	if k.concurrencyticker == nil {
		t.Fatal("Nil keeper concurrency ticker")
	}
}

func Test_Do_Throughput(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p := &passthrough{
		ctx,
		make(chan *http.Request),
	}

	client := New(ctx, p, 0, 0, 0, time.Minute)
	r := newGetReqWCtx()

	go func(r *http.Request) {
		client.Do(r)
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

// NOTE: The timer tests use for the most part a 1 second tolerance
// due to the fact that the timer has to be scheduled and by the time
// the current time is checked it actually may have elapsed an entire second
// this is meant to be as close as possible to real-world

func Test_timer_intHeader(t *testing.T) {
	testdata := map[string]struct {
		retryHeader string
		expected    time.Duration
		tolerance   time.Duration
	}{
		"integer w/s - 1s": {
			"1s",
			time.Second,
			time.Second,
		},
		"integer w/s - 2s": {
			"2s",
			time.Second * 2,
			time.Second,
		},
		"integer w/s - 5s": {
			"5s",
			time.Second * 5,
			time.Second,
		},
		"integer w/s - 10s": {
			"10s",
			time.Second * 10,
			time.Second,
		},
		"integer wout/s - 1s": {
			"1",
			time.Second,
			time.Second,
		},
		"integer wout/s - 2s": {
			"2",
			time.Second * 2,
			time.Second,
		},
		"integer wout/s - 5s": {
			"5",
			time.Second * 5,
			time.Second,
		},
		"integer wout/s - 10s": {
			"10",
			time.Second * 10,
			time.Second,
		},
		"invalid parse": {
			"not a valid time to parse",
			0,
			time.Microsecond * 500,
		},
		"empty header": {
			"",
			0,
			time.Microsecond * 500,
		},
	}

	for name, test := range testdata {
		t.Run(name, func(t *testing.T) {
			timer := timer(test.retryHeader)
			defer timer.Stop()

			tstart := time.Now()

			<-timer.C
			diff := time.Now().Sub(tstart)
			expPos := test.expected + test.tolerance
			expNeg := test.expected - test.tolerance
			if diff < expNeg || diff > expPos {
				t.Fatalf("timer exceeded tolerance %s < %s < %s", expNeg, diff, expPos)
			}
		})
	}
}

func timedelay(format string, delay time.Duration) string {
	return time.Now().Add(delay).Format(format)
}

func Test_timer_timeHeader(t *testing.T) {
	testdata := map[string]struct {
		format    string
		expected  time.Duration
		tolerance time.Duration
	}{
		"RFC1123 - 1s": {
			time.RFC1123,
			time.Second,
			time.Second,
		},
		"RFC1123 - 2s": {
			time.RFC1123,
			time.Second * 2,
			time.Second,
		},
		"RFC1123 - 5s": {
			time.RFC1123,
			time.Second * 5,
			time.Second,
		},
		"RFC1123 - 10s": {
			time.RFC1123,
			time.Second * 5,
			time.Second,
		},
		"invalid parse": {
			time.RFC3339,
			0,
			time.Microsecond * 500,
		},
	}

	for name, test := range testdata {
		t.Run(name, func(t *testing.T) {
			timer := timer(timedelay(test.format, test.expected))
			defer timer.Stop()

			tstart := time.Now()

			<-timer.C
			diff := time.Now().Sub(tstart)
			expPos := test.expected + test.tolerance
			expNeg := test.expected - test.tolerance
			if diff < expNeg || diff > expPos {
				t.Fatalf("timer exceeded tolerance %s < %s < %s", expNeg, diff, expPos)
			}
		})
	}
}

func Benchmark_Do_ZeroConcurrency(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p := &passthrough{
		ctx,
		make(chan *http.Request),
	}

	client := New(ctx, p, 0, 0, 0, time.Minute)
	r := newGetReqWCtx()

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		go func(r *http.Request) {
			client.Do(r)
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
