package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"time"

	httpw "github.com/eduvpn/eduvpn-common/internal/http"
	"github.com/eduvpn/eduvpn-common/internal/log"
	"github.com/eduvpn/eduvpn-common/internal/oauth"
	"github.com/eduvpn/eduvpn-common/internal/server/base"
	"github.com/eduvpn/eduvpn-common/internal/server/endpoints"
	"github.com/eduvpn/eduvpn-common/internal/server/profile"
	"github.com/go-errors/errors"
)

func Endpoints(ctx context.Context, b *base.Base) error {
	uStr, err := httpw.JoinURLPath(b.URL, "/.well-known/vpn-user-portal")
	if err != nil {
		return err
	}
	if b.HTTPClient == nil {
		b.HTTPClient = httpw.NewClient()
	}
	_, body, err := b.HTTPClient.Get(ctx, uStr)
	if err != nil {
		return errors.WrapPrefix(err, "failed getting server endpoints", 0)
	}

	ep := endpoints.Endpoints{}
	if err = json.Unmarshal(body, &ep); err != nil {
		return errors.WrapPrefix(err, "failed getting server endpoints", 0)
	}
	err = ep.Validate()
	if err != nil {
		return err
	}

	b.Endpoints = ep
	return nil
}

func authorized(
	ctx context.Context,
	b *base.Base,
	oauth *oauth.OAuth,
	method string,
	endpoint string,
	opts *httpw.OptionalParams,
) (http.Header, []byte, error) {
	// Ensure optional is not nil as we will fill it with headers
	if opts == nil {
		opts = &httpw.OptionalParams{}
	}
	errorMessage := "failed API authorized"

	// Join the paths
	u, err := url.Parse(b.Endpoints.API.V3.API)
	if err != nil {
		return nil, nil, errors.WrapPrefix(err, errorMessage, 0)
	}
	u.Path = path.Join(u.Path, endpoint)

	// Make sure the tokens are valid, this will return an error if re-login is needed
	t, err := oauth.AccessToken(ctx)
	if err != nil {
		return nil, nil, errors.WrapPrefix(err, errorMessage, 0)
	}

	key := "Authorization"
	val := fmt.Sprintf("Bearer %s", t)
	if opts.Headers != nil {
		opts.Headers.Add(key, val)
	} else {
		opts.Headers = http.Header{key: {val}}
	}

	// Create a client if it doesn't exist
	if b.HTTPClient == nil {
		b.HTTPClient = httpw.NewClient()
	}
	return b.HTTPClient.Do(ctx, method, u.String(), opts)
}

func authorizedRetry(
	ctx context.Context,
	b *base.Base,
	auth *oauth.OAuth,
	method string,
	endpoint string,
	opts *httpw.OptionalParams,
) (http.Header, []byte, error) {
	h, body, err := authorized(ctx, b, auth, method, endpoint, opts)
	if err == nil {
		return h, body, nil
	}

	statErr := &httpw.StatusError{}
	// Only retry authorized if we get an HTTP 401
	if errors.As(err, &statErr) && statErr.Status == 401 {
		log.Logger.Debugf("Got a 401 error after HTTP method: %s, endpoint: %s. Marking token as expired...", method, endpoint)
		// Mark the token as expired and retry, so we trigger the refresh flow
		auth.SetTokenExpired()
		h, body, err = authorized(ctx, b, auth, method, endpoint, opts)
	}
	return h, body, err
}

func Info(ctx context.Context, b *base.Base, auth *oauth.OAuth) error {
	_, body, err := authorizedRetry(ctx, b, auth, http.MethodGet, "/info", nil)
	if err != nil {
		return err
	}
	profiles := profile.Info{}
	if err = json.Unmarshal(body, &profiles); err != nil {
		return errors.WrapPrefix(err, "failed API /info", 0)
	}

	// Store the profiles and make sure that the current profile is not overwritten
	prev := b.Profiles.Current
	b.Profiles = profiles
	b.Profiles.Current = prev
	return nil
}

// see https://github.com/eduvpn/documentation/blob/v3/API.md#request-1
func boolToYesNo(preferTCP bool) string {
	if preferTCP {
		return "yes"
	}
	return "no"
}

func ConnectWireguard(
	ctx context.Context,
	b *base.Base,
	auth *oauth.OAuth,
	profileID string,
	pubkey string,
	preferTCP bool,
	openVPNSupport bool,
) (string, string, time.Time, error) {
	hdrs := http.Header{
		"content-type": {"application/x-www-form-urlencoded"},
		"accept":       {"application/x-wireguard-profile"},
	}

	// This profile also supports OpenVPN
	// Indicate that we also accept OpenVPN profiles
	if openVPNSupport {
		hdrs.Add("accept", "application/x-openvpn-profile")
	}

	vals := url.Values{
		"profile_id": {profileID},
		"public_key": {pubkey},
		"prefer_tcp": {boolToYesNo(preferTCP)},
	}
	h, body, err := authorizedRetry(ctx, b, auth, http.MethodPost, "/connect",
		&httpw.OptionalParams{Headers: hdrs, Body: vals})
	if err != nil {
		return "", "", time.Time{}, err
	}

	exp := h.Get("expires")

	expTime, err := http.ParseTime(exp)
	if err != nil {
		return "", "", time.Time{}, errors.WrapPrefix(err, "failed obtaining a WireGuard configuration", 0)
	}

	contentH := h.Get("content-type")
	content := "openvpn"
	if contentH == "application/x-wireguard-profile" {
		content = "wireguard"
	}

	return string(body), content, expTime, nil
}

func ConnectOpenVPN(ctx context.Context, b *base.Base, auth *oauth.OAuth, profileID string, preferTCP bool) (string, time.Time, error) {
	hdrs := http.Header{
		"content-type": {"application/x-www-form-urlencoded"},
		"accept":       {"application/x-openvpn-profile"},
	}

	vals := url.Values{
		"profile_id": {profileID},
		"prefer_tcp": {boolToYesNo(preferTCP)},
	}

	h, body, err := authorizedRetry(ctx, b, auth, http.MethodPost, "/connect",
		&httpw.OptionalParams{Headers: hdrs, Body: vals})
	if err != nil {
		return "", time.Time{}, err
	}

	expH := h.Get("expires")
	expT, err := http.ParseTime(expH)
	if err != nil {
		return "", time.Time{}, errors.WrapPrefix(err, "failed obtaining an OpenVPN configuration", 0)
	}

	return string(body), expT, nil
}

// Disconnect disconnects the VPN using the API.
func Disconnect(ctx context.Context, b *base.Base, auth *oauth.OAuth) error {
	// The timeout is a bit lower here such that this does not take a too long time for disconnecting
	// Clients may wish to retry this
	_, _, err := authorized(ctx, b, auth, http.MethodPost, "/disconnect", &httpw.OptionalParams{Timeout: 5 * time.Second})
	return err
}
