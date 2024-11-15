# Bridgekeeper - What is your (Re)Quest?

[![Build & Test](https://github.com/devnw/bridgekeeper/actions/workflows/build.yml/badge.svg)](https://github.com/devnw/bridgekeeper/actions/workflows/build.yml)
[![Go Report Card](https://goreportcard.com/badge/go.devnw.com/bk)](https://goreportcard.com/report/go.devnw.com/bk)
[![codecov](https://codecov.io/gh/devnw/bridgekeeper/branch/main/graph/badge.svg)](https://codecov.io/gh/devnw/bridgekeeper)
[![GoDoc](https://godoc.org/go.devnw.com/bk?status.svg)](https://pkg.go.dev/go.devnw.com/bk)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](http://makeapullrequest.com)

## Bridgekeeper is an HTTP Request Limiter and Retrier


### Using Bridgekeeper

```go
go get -u go.devnw.com/bk/v2@latest
```

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
