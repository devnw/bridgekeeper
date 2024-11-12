// package bk is intended to create a client side load balancer or
// rate limiter for API integrations. This library is specifically designed to
// wrap the `Do` method of the http.Client but since it uses an interface
// abstraction it can wrap any interface and limit requests.
//
// Controls
// - Delay between requests
// - Number of retries per request
// - Concurrency limit for the client
package bk

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// Client defines an interface with a Do method for use with http.Client or other
// mocked instances that implement a do method that accept a request and return a response error combo
type Client interface {
	Do(req *http.Request) (*http.Response, error)
}

// New creates a new instance of the bridgekeeper for use with an api. New
// returns an interface implementation of Client which replaces the
// implementation of an http.Client interface so that it looks like an
// http.Client and can perform the same functions but it limits the requests
// using the parameters defined when created. NOTE: If a request timeout is not
// set at creation then the default HTTP client request timeout will be used
func New(
	ctx context.Context,
	client Client,
	delay time.Duration,
	retries int,
	concurrency int,
	requestTimeout time.Duration,
) Client {
	if requestTimeout < time.Nanosecond {
		requestTimeout = http.DefaultClient.Timeout
	}

	if retries < 0 {
		retries = 0
	}

	if delay <= 0 {
		delay = time.Nanosecond
	}

	// ensure the concurrency is setup above zero
	if concurrency < 1 {
		concurrency = 1
	}

	// Setup a background context if no context is passed
	if ctx == nil {
		ctx = context.Background()
	}

	ctx, cancel := context.WithCancel(ctx)

	// If a nil client is passed to the bridgekeeper then initialize using the
	// default http client
	if client == nil {
		client = http.DefaultClient
	}

	k := &keeper{
		ctx:               ctx,
		cancel:            cancel,
		client:            client,
		retries:           retries,
		delay:             delay,
		ticker:            time.NewTicker(delay),
		concurrency:       concurrency,
		concurrencyticker: make(chan bool, concurrency),
		requestTimeout:    requestTimeout,
	}

	// Initialize the concurrency channel for managing concurrent calls
	for i := 0; i < k.concurrency; i++ {
		select {
		case <-k.ctx.Done():
		case k.concurrencyticker <- true:
		}
	}

	// Setup requests channel
	k.requests = k.receive()

	go k.cleanup()

	return k
}

// cleanup deals with cleaning any struct values for the keeper
func (k *keeper) cleanup() {
	<-k.ctx.Done()

	if k.ticker != nil {
		k.ticker.Stop()
	}
}

// RoundTrip is a wrapper for the Do method of the bridgekeeper. This is
// necessary for the bridgekeeper to implement the http.RoundTripper
func (k *keeper) RoundTrip(req *http.Request) (*http.Response, error) {
	return k.Do(req)
}

// Do sends the http request through the bridgekeeper to be executed against the
// endpoint when there are available threads to do so. This returns an http
// response which is returned from the execution of the http request as well
// as an error
//
// XXX: Possibly add in defer here that determines if the response is nil
// and executes the wrapped `Do` method directly
func (k *keeper) Do(request *http.Request) (*http.Response, error) {
	if request == nil {
		return nil, errors.New("request cannot be nil")
	}

	// Fail open if the bridgekeeper request was canceled
	select {
	case <-k.ctx.Done():
		return k.client.Do(request)
	default:

		// If the request has a context then use it
		ctx := request.Context()
		if ctx == nil || ctx == context.Background() {
			ctx = k.ctx

			// Add the context to the request if it didn't already
			// have one assigned
			request = request.WithContext(ctx)
		}

		// Enforce request specific timeout
		ctx, cancel := context.WithTimeout(ctx, k.requestTimeout)
		defer cancel()

		var responsechan = make(chan responseWrapper)
		defer close(responsechan)

		// Create the request wrapper to send to receive
		req := &requestWrapper{
			ctx:      ctx,
			request:  request,
			response: responsechan,
		}

		// Send the request to the processing channel of the bridgekeeper
		go func() {
			select {
			case <-ctx.Done():
				return
			case k.requests <- req:
			}
		}()

		// Wait for the response from the request
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case resp, ok := <-responsechan:
			if !ok {
				return nil, fmt.Errorf("response channel closed prematurely")
			}

			return resp.response, resp.err
		}
	}
}
