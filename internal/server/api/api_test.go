package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/eduvpn/eduvpn-common/internal/server/base"
	"github.com/eduvpn/eduvpn-common/internal/server/endpoints"
	"github.com/eduvpn/eduvpn-common/internal/test"
	"github.com/go-errors/errors"
)

func getErrorMsg(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func compareEndpoints(ep1 endpoints.Endpoints, ep2 endpoints.Endpoints) bool {
	v3_1 := ep1.API.V3
	v3_2 := ep2.API.V3
	return v3_1.API == v3_2.API && v3_1.Authorization == v3_2.Authorization && v3_1.Token == v3_2.Token
}

func Test_APIGetEndpoints(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello!")
	})
	hs := &test.HandlerSet{}
	hs.SetHandler(handler)
	s := test.NewServer(hs)
	defer s.Close()

	c, err := s.Client()
	if err != nil {
		t.Fatalf("failed to get client for test server endpoints: %v", err)
	}

	testCases := []struct {
		epl endpoints.List
		err error
	}{
		{
			epl: endpoints.List{
				API:           "https://example.com/1",
				Authorization: "https://example.com/2",
				Token:         "https://example.com/3",
			},
			err: nil,
		},
		{
			epl: endpoints.List{
				API:           "https://example.com/1",
				Authorization: "http://example.com/2",
				Token:         "http://example.com/3",
			},
			err: errors.New("API scheme: 'https', is not equal to authorization scheme: 'http'"),
		},
		{
			epl: endpoints.List{
				API:           "https://example.com/1",
				Authorization: "https://example.com/2",
				Token:         "ftp://example.com/3",
			},
			err: errors.New("API scheme: 'https', is not equal to token scheme: 'ftp'"),
		},
		{
			epl: endpoints.List{
				API:           "https://malicious.com/1",
				Authorization: "https://example.com/2",
				Token:         "https://example.com/3",
			},
			err: errors.New("API host: 'malicious.com', is not equal to authorization host: 'example.com'"),
		},
		{
			epl: endpoints.List{
				API:           "https://example.com/1",
				Authorization: "https://example.com/2",
				Token:         "https://malicious.com/3",
			},
			err: errors.New("API host: 'example.com', is not equal to token host: 'malicious.com'"),
		},
		{
			epl: endpoints.List{
				API:           "https://example.com/1",
				Authorization: "https://malicious.com/2",
				Token:         "https://example.com/3",
			},
			err: errors.New("API host: 'example.com', is not equal to authorization host: 'malicious.com'"),
		},
		{
			epl: endpoints.List{
				API:           "https://example.com/1",
				Authorization: "https://example.com/2",
				Token:         "ftp://example.com/3",
			},
			err: errors.New("API scheme: 'https', is not equal to token scheme: 'ftp'"),
		},
		{
			epl: endpoints.List{
				API:           "https://example.com/1",
				Authorization: "ftp://example.com/2",
				Token:         "https://example.com/3",
			},
			err: errors.New("API scheme: 'https', is not equal to authorization scheme: 'ftp'"),
		},
	}

	for _, tc := range testCases {
		ep := &endpoints.Endpoints{
			API: endpoints.Versions{
				V3: tc.epl,
			},
		}
		// Update the handler
		hs.SetHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			jsonStr, err := json.Marshal(ep)
			if err != nil {
				t.Fatalf("failed to marshal JSON for test case: %v, err: %v", tc, err)
			}

			fmt.Fprintln(w, string(jsonStr))
		}))
		b := &base.Base{
			URL:        s.URL,
			HTTPClient: c,
		}
		err = Endpoints(context.Background(), b)
		if getErrorMsg(err) != getErrorMsg(tc.err) {
			t.Fatalf("Errors not equal, want err: %v, got: %v", tc.err, err)
		}
		// The error was not nil, continue because endpoints should not be compared
		if tc.err != nil {
			continue
		}
		if ep == nil {
			t.Fatalf("No test case endpoints")
		}
		// if no error then the endpoints should be equal
		if !compareEndpoints(*ep, b.Endpoints) {
			t.Fatalf("Endpoints are not equal, got: %v, want: %v", b.Endpoints, ep)
		}
	}
}
