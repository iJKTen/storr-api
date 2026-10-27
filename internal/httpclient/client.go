package httpclient

import (
	"net/http"
	"time"
)

// Default is a shared HTTP client with a 15-second timeout, replacing usage of
// http.DefaultClient which has no timeout and can hang indefinitely.
var Default = &http.Client{
	Timeout: 15 * time.Second,
}
