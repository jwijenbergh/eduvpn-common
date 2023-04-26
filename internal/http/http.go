// Package http defines higher level helpers for the net/http package
package http

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/go-errors/errors"
)

// UserAgent is the user agent that is used for requests
var UserAgent string

// URLParameters is a type used for the parameters in the URL.
type URLParameters map[string]string

// OptionalParams is a structure that defines the optional parameters that are given when making a HTTP call.
type OptionalParams struct {
	Headers       http.Header
	URLParameters URLParameters
	Body          url.Values
	Timeout       time.Duration
}

func cleanPath(u *url.URL, trailing bool) string {
	if u.Path != "" {
		// Clean the path
		// https://pkg.go.dev/path#Clean
		u.Path = path.Clean(u.Path)
	}

	str := u.String()

	// Make sure the URL ends with a /
	if trailing && str[len(str)-1:] != "/" {
		str += "/"
	}
	return str
}

// EnsureValidURL ensures that the input URL is valid to be used internally
// It does the following
// - Sets the scheme to https if none is given
// - It 'cleans' up the path using path.Clean
// - It makes sure that the URL ends with a /
// It returns an error if the URL cannot be parsed.
func EnsureValidURL(s string, trailing bool) (string, error) {
	u, err := url.Parse(s)
	if err != nil {
		return "", errors.WrapPrefix(err, "failed parsing url", 0)
	}

	// Make sure the scheme is always https
	if u.Scheme != "https" {
		u.Scheme = "https"
	}
	return cleanPath(u, trailing), nil
}

// JoinURLPath joins url's path, in go 1.19 we can use url.JoinPath
func JoinURLPath(u string, p string) (string, error) {
	pu, err := url.Parse(u)
	if err != nil {
		return "", errors.WrapPrefix(err, "failed to parse url for joining paths", 0)
	}
	pp, err := url.Parse(p)
	if err != nil {
		return "", errors.WrapPrefix(err, "failed to parse path for joining paths", 0)
	}
	fp := pu.ResolveReference(pp)

	// We also clean the path for consistency
	return cleanPath(fp, false), nil
}

// ConstructURL creates a URL with the included parameters.
func ConstructURL(u *url.URL, params URLParameters) (string, error) {
	q := u.Query()

	for p, value := range params {
		q.Set(p, value)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// optionalURL ensures that the URL contains the optional parameters
// it returns the url (with parameters if success) and an error indicating success.
func optionalURL(urlStr string, opts *OptionalParams) (string, error) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return "", errors.WrapPrefix(err,
			fmt.Sprintf("failed to construct parse url '%s'", urlStr), 0)
	}
	// Make sure the scheme is always set to HTTPS
	if u.Scheme != "https" {
		u.Scheme = "https"
	}

	if opts == nil {
		return u.String(), nil
	}

	return ConstructURL(u, opts.URLParameters)
}

// optionalHeaders ensures that the HTTP request uses the optional headers if defined.
func optionalHeaders(req *http.Request, opts *OptionalParams) {
	// Add headers
	if opts != nil && req != nil && opts.Headers != nil {
		for k, v := range opts.Headers {
			req.Header.Add(k, v[0])
		}
	}
}

// optionalBodyReader returns a HTTP body reader if there is a body, otherwise nil.
func optionalBodyReader(opts *OptionalParams) io.Reader {
	if opts != nil && opts.Body != nil {
		return strings.NewReader(opts.Body.Encode())
	}
	return nil
}

// Client is a wrapper around http.Client with some convenience features
// - A default timeout of 5 seconds
// - A read limiter to prevent servers from sending large amounts of data
// - Checking on http code with custom errors
type Client struct {
	// Client is the HTTP Client that sends the request
	Client *http.Client
	// ReadLimit denotes the maximum amount of bytes that are read in HTTP responses
	// This is used to prevent servers from sending huge amounts of data
	// A limit of 16MB, although maybe much larger than needed, ensures that we do not run into problems
	ReadLimit int64

	// Timeout denotes the default timeout for each request
	Timeout time.Duration
}

// Returns a HTTP client with some default settings
func NewClient() *Client {
	c := &http.Client{}
	// ReadLimit denotes the maximum amount of bytes that are read in HTTP responses
	// This is used to prevent servers from sending huge amounts of data
	// A limit of 16MB, although maybe much larger than needed, ensures that we do not run into problems
	// The timeout is 10 seconds by default. We pass it here and not in the http client because we want to do it per request
	return &Client{Client: c, ReadLimit: 16 << 20, Timeout: 10 * time.Second}
}

// Get creates a Get request and returns the headers, body and an error.
func (c *Client) Get(ctx context.Context, url string) (http.Header, []byte, error) {
	return c.Do(ctx, http.MethodGet, url, nil)
}

// PostWithOpts creates a Post request with optional parameters and returns the headers, body and an error.
func (c *Client) PostWithOpts(ctx context.Context, url string, opts *OptionalParams) (http.Header, []byte, error) {
	return c.Do(ctx, http.MethodPost, url, opts)
}

// MethodWithOpts Do send a HTTP request using a method (e.g. GET, POST), an url and optional parameters
// It returns the HTTP headers, the body and an error if there is one.
func (c *Client) Do(ctx context.Context, method string, urlStr string, opts *OptionalParams) (http.Header, []byte, error) {
	// Make sure the url contains all the parameters
	// This can return an error,
	// it already has the right error, so we don't wrap it further
	urlStr, err := optionalURL(urlStr, opts)
	if err != nil {
		// No further type wrapping is needed here
		return nil, nil, err
	}

	// The timeout is configurable for each request
	timeout := c.Timeout
	if opts != nil && opts.Timeout.Seconds() > 0 {
		timeout = opts.Timeout
	}

	ctx, cncl := context.WithTimeout(ctx, timeout)
	defer cncl()

	// Create request object with the body reader generated from the optional arguments
	req, err := http.NewRequestWithContext(ctx, method, urlStr, optionalBodyReader(opts))
	if err != nil {
		return nil, nil, errors.WrapPrefix(err,
			fmt.Sprintf("failed HTTP request with method %s and url %s", method, urlStr), 0)
	}
	if UserAgent != "" {
		req.Header.Add("User-Agent", UserAgent)
	}

	// Make sure the headers contain all the parameters
	optionalHeaders(req, opts)

	// Do request
	res, err := c.Client.Do(req)
	if err != nil {
		return nil, nil, errors.WrapPrefix(err,
			fmt.Sprintf("failed HTTP request with method %s and url %s", method, urlStr), 0)
	}

	// Request successful, make sure body is closed at the end
	defer func() {
		_ = res.Body.Close()
	}()

	// Return a string
	// A max bytes reader is normally used for request bodies with a writer
	// However, this is still nice to use because unlike a limitreader, it returns an error if the body is too large
	// We use this function without a writer so we pass nil
	// We impose a limit because servers could be malicious and send huge amounts of data
	r := http.MaxBytesReader(nil, res.Body, c.ReadLimit)
	body, err := io.ReadAll(r)
	if err != nil {
		return res.Header, nil, errors.WrapPrefix(err,
			fmt.Sprintf("failed HTTP request with method: %s, url: %s and max bytes size: %v", method, urlStr, c.ReadLimit), 0)
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return res.Header, body, errors.Wrap(&StatusError{URL: urlStr, Body: string(body), Status: res.StatusCode}, 0)
	}

	// Return the body in bytes and signal the status error if there was one
	return res.Header, body, nil
}

// StatusError indicates that we have received a HTTP status error.
type StatusError struct {
	URL    string
	Body   string
	Status int
}

// Error returns the StatusError as an error string.
func (e *StatusError) Error() string {
	return fmt.Sprintf(
		"failed obtaining HTTP resource: %s as it gave an unsuccessful status code: %d. Body: %s",
		e.URL,
		e.Status,
		e.Body,
	)
}

// RegisterAgent registers the user agent for client and version
func RegisterAgent(client string, version string) {
	UserAgent = fmt.Sprintf("%s/%s %s", client, version, "eduvpn-common/2.0.0")
}
