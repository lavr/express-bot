package httputil

import (
	"fmt"
	"net/http"
	"runtime"
	"time"
)

// version is set by SetVersion during startup.
var version = "dev"

// SetVersion sets the application version used in the User-Agent header.
func SetVersion(v string) {
	version = v
}

// UserAgent returns the User-Agent string:
//
//	express-botx/1.2.3 Go/1.23.4 linux/amd64
func UserAgent() string {
	return fmt.Sprintf("express-botx/%s %s %s/%s",
		version, runtime.Version(), runtime.GOOS, runtime.GOARCH)
}

// Transport returns an http.RoundTripper that sets the User-Agent header
// on every outgoing request. It wraps the provided base transport;
// if base is nil, http.DefaultTransport is used.
func Transport(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return &uaTransport{base: base}
}

type uaTransport struct {
	base http.RoundTripper
}

func (t *uaTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Header.Get("User-Agent") == "" {
		req = req.Clone(req.Context())
		req.Header.Set("User-Agent", UserAgent())
	}
	return t.base.RoundTrip(req)
}

// NewClient returns an *http.Client with the User-Agent transport and the given timeout.
func NewClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:   timeout,
		Transport: Transport(nil),
	}
}
