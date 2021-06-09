package bridgekeeper

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type keeper struct {
	ctx               context.Context
	cancel            context.CancelFunc
	client            Client
	retries           int
	ticker            *time.Ticker
	delay             time.Duration
	concurrency       int
	concurrencyticker chan bool
	requests          chan<- *requestWrapper
	requestTimeout    time.Duration
}

// Receive creates a new listening channel for http request wrappers. After
// creating the request channel it then monitors the delay timer (aka ticker)
// for each tick then checks for an available concurrency entry on the
// concurrency channel to process work. Once it's cleared the ticker and
// concurrency channel it then pulls an available request from the request
// channel and executes the http request against the endpoint and returns the
// response across the response channel of the request along with any errors
// that occurred when making the request
func (k *keeper) receive() chan<- *requestWrapper {
	reqs := make(chan *requestWrapper)

	go func(reqs chan *requestWrapper) {
		defer func() {
			if r := recover(); r != nil {
				k.cancel()
			}
		}()

		k.reqHandler(reqs)
	}(reqs)

	return reqs
}

func (k *keeper) reqHandler(reqs chan *requestWrapper) {
	for {
		select {
		case <-k.ctx.Done():
			return
		case _, ok := <-k.ticker.C:
			if !ok {
				return
			}

			select {
			case <-k.ctx.Done():
				return
			case _, ok = <-k.concurrencyticker:
				if !ok {
					return
				}

				k.process(reqs)
			}
		}
	}
}

func (k *keeper) process(requests chan *requestWrapper) {
	defer func() {
		_ = recover()
	}()

	select {
	case <-k.ctx.Done():
	case req, ok := <-requests:
		if !ok {
			return
		}

		go k.handleRequest(req)
	}
}

func (k *keeper) handleRequest(req *requestWrapper) {
	defer func() {
		if r := recover(); r != nil {
			return
		}

		// Must have the context done in the select here otherwise if the ctx is
		// closed then this will cause a panic because of sending on a closed
		// channel
		select {
		case <-k.ctx.Done():
		case k.concurrencyticker <- true:
		}
	}()

	// Execute a call against the endpoint handling any potential panics from
	// the http client
	resp, err := k.execute(req)
	if resp == nil {
		select {
		case <-req.ctx.Done():
		case req.response <- responseWrapper{
			response: resp,
			err:      fmt.Errorf("request returned a null response"),
		}:
			return
		}
	}

	// Successful Request
	if err == nil && resp.StatusCode < 300 {
		select {
		case <-req.ctx.Done():
		case req.response <- responseWrapper{resp, err}:
			return
		}
	}

	// If the request received a 404 respnose or the retries have run out then
	// return the response
	if resp.StatusCode == http.StatusNotFound || // 404 Response
		k.retries == 0 || // No Retries
		(k.retries > 0 && req.attempts >= k.retries) { // Retries exceeded
		select {
		case <-req.ctx.Done():
		case req.response <- responseWrapper{
			resp,
			fmt.Errorf(
				"retries exceeded for request | err: %s",
				err,
			)}:
		}
		return
	}

	// Read and close the body of the response
	readAndClose(resp.Body)

	go k.resend(req, timer(resp.Header.Get("Retry-After")))
}

func timer(retryHeader string) *time.Timer {
	if retryHeader == "" {
		return time.NewTimer(0)
	}

	t, err := time.Parse(time.RFC1123, retryHeader)
	if err == nil {
		return time.NewTimer(t.Sub(time.Now()))
	}

	if !strings.HasSuffix(retryHeader, "s") {
		retryHeader = retryHeader + "s"
	}

	rdelay, err := time.ParseDuration(retryHeader)
	if err == nil {
		return time.NewTimer(rdelay)
	}

	return time.NewTimer(0)
}

func (k *keeper) resend(req *requestWrapper, timer *time.Timer) {
	defer timer.Stop()

	select {
	case <-req.ctx.Done():
	case <-timer.C:

		// Send the request back on the channel
		select {
		case <-req.ctx.Done():
		case k.requests <- req:
		}
	}
}

func (k *keeper) execute(req *requestWrapper) (resp *http.Response, err error) {
	defer func(req *requestWrapper) {
		if r := recover(); r != nil {
			err = errors.New("panic occurred while executing http request")
			return
		}

		// increment the attempt counter
		req.attempts++
	}(req)

	// TODO: Not sure this is necessary? Causes the TCP connection to not be
	// reused...
	// req.request.Close = true

	// Execute the http request and return the response to the requester
	return k.client.Do(req.request)
}

// readAndClose reads all of the contents of the readcloser and closes
//
// As per the Go stdlib documentation for net/http.Response
//
// The default HTTP client's Transport may not reuse HTTP/1.x
// "keep-alive" TCP connections if the Body is	not read to
// completion and closed.
func readAndClose(rc io.ReadCloser) {
	if rc != nil {
		defer func() {
			if r := recover(); r != nil {
				return
			}
		}()

		_, _ = io.ReadAll(rc)
		_ = rc.Close()
	}
}

// wrapper for transporting requests along a channel along with a response
// channel for returning the response from the endpoint as well as an attempt
// counter for tracking the number of times a request has been attempted in the
// event that it continues to fail
type requestWrapper struct {
	ctx      context.Context
	request  *http.Request
	response chan<- responseWrapper
	attempts int
}

// wrapper for tracking the response of executing a client.Do against an
// http request. This returns any errors from the bridgekeeper attempting to execute
// the request as well as the http response in the event of a response
type responseWrapper struct {
	response *http.Response
	err      error
}
