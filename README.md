# Bridgekeeper - HTTP Request Limiter and Retrier

[![Build & Test](https://github.com/devnw/bridgekeeper/actions/workflows/build.yml/badge.svg)](https://github.com/devnw/bridgekeeper/actions/workflows/build.yml)
[![Go Reference](https://pkg.go.dev/badge/go.devnw.com/bk/v2.svg)](https://pkg.go.dev/go.devnw.com/bk)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

## Using Bridgekeeper

```go
go get -u go.devnw.com/bk/v2@latest
```

## Version 2 Changes

Version 2 changes the API for Bridgekeeper to using a direct function literal
in the `New` function so that it can accept both the `Do` and `RoundTrip`
functions from the `http.Client` and `http.Transport` respectively. This
allows for Bridgekeeper to support connections where a custom `http.Transport`
is may be required, for example, when using a custom TLS configuration.

### HTTP Client Example

```go
    client := bk.New(
        ctx, // Your application context
        http.DefaultClient.Do, // Your HTTP Client Do function (http.Client.Do)
        time.Millisecond, // Delay between requests
        5, // Retry count
        10, // Concurrent request limit
        http.DefaultClient.Timeout, // Request timeout
    )

    resp, err := client.Do(http.NewRequest(http.MethodGet, "localhost:5555"))
    if err != nil {
        log.Fatal(err)
    }
```

### HTTP Round Tripper Example

```go
    client := bk.New(
        ctx, // Your application context
        http.DefaultTransport.RoundTrip, // Your HTTP Transport
        time.Millisecond, // Delay between requests
        5, // Retry count
        10, // Concurrent request limit
        http.DefaultClient.Timeout, // Request timeout
    )

    resp, err := client.RoundTrip(http.NewRequest(http.MethodGet, "localhost:5555"))
    if err != nil {
        log.Fatal(err)
    }
```

> NOTE: Bridgekeeper Returns a Do / RoundTrip Compliant HTTP Client
