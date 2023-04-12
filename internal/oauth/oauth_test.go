package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"
)

func Test_verifiergen(t *testing.T) {
	v, err := genVerifier()
	if err != nil {
		t.Fatalf("Gen verifier error: %v", err)
	}

	// Verifier must be at minimum 43 and at max 128 characters...
	// However... Our verifier is exactly 43!
	if len(v) != 43 {
		t.Fatalf(
			"Got verifier length: %d, want a verifier with at least 43 characters",
			len(v),
		)
	}

	_, err = url.QueryUnescape(v)
	if err != nil {
		t.Fatalf("Verifier: %s can not be unescaped", v)
	}
}

func Test_stategen(t *testing.T) {
	s1, err := genState()
	if err != nil {
		t.Fatalf("Error when generating state 1: %v", err)
	}

	s2, err := genState()
	if err != nil {
		t.Fatalf("Error when generating state 2: %v", err)
	}

	if s1 == s2 {
		t.Fatalf("State: %v, equal to: %v", s1, s2)
	}
}

func Test_challengergen(t *testing.T) {
	verifier := "test"
	// Calculated using: base64.urlsafe_b64encode(hashlib.sha256("test".encode("utf-8")).digest()).decode("utf-8").replace("=", "") in Python
	// This test might not be the best because we're now comparing two different implementations, but at least it gives us a way to see if we messed something up in a commit
	want := "n4bQgYhMfWWaL-qgxVrQFaO_TxsrC4Is0V1sFbDwCgg"
	got := genChallengeS256(verifier)

	if got != want {
		t.Fatalf("Challenger not equal, got: %v, want: %v", got, want)
	}
}

func Test_accessToken(t *testing.T) {
	o := OAuth{}
	_, err := o.AccessToken(context.Background())
	if err == nil {
		t.Fatalf("No error when getting access token on empty structure")
	}

	// Here we should get no error because the access token is set and is not expired
	want := "test"
	expired := time.Now().Add(1 * time.Hour)
	o = OAuth{token: &tokenLock{t: &tokenRefresher{Token: Token{Access: want, ExpiredTimestamp: expired}}}}
	got, err := o.AccessToken(context.Background())
	if err != nil {
		t.Fatalf("Got error when getting access token on non-empty structure: %v", err)
	}
	if got != want {
		t.Fatalf("Access token not equal, Got: %v, Want: %v", got, want)
	}

	// Set the tokens as expired
	o.SetTokenExpired()

	// We should not get an error because expired and no refresh token
	_, err = o.AccessToken(context.Background())
	if err == nil {
		t.Fatal("Got no error when getting access token on non-empty structure and expired")
	}

	want = "test2"
	// Now we internally update the refresh function and refresh token, we should get new tokens
	refresh := "refresh"
	o.token.t.Refresh = refresh
	o.token.t.Refresher = func(ctx context.Context, refreshToken string) (*TokenResponse, time.Time, error) {
		if refreshToken != refresh {
			t.Fatalf("Passed refresh token to refresher not equal to updated refresh token, got: %v, want: %v", refreshToken, refresh)
		}
		// Only the access and refresh fields are really important
		r := &TokenResponse{Access: want, Refresh: "test2"}
		return r, expired, nil
	}

	got, err = o.AccessToken(context.Background())
	if err != nil {
		t.Fatalf("Got error when getting access token on non-empty expired structure and with a 'valid' refresh token: %v", err)
	}
	if got != want {
		t.Fatalf("Access token not equal, Got: %v, Want: %v", got, want)
	}


	// Set the tokens as expired
	o.SetTokenExpired()
	want = "test3"

	// Now let's act like a 2.x server, we give no refresh token back. When we refresh the previous refresh token should be gotten
	o.token.t.Refresh = refresh
	prevRefresh := refresh
	o.token.t.Refresher = func(refreshToken string) (*TokenResponse, time.Time, error) {
		if refreshToken != refresh {
			t.Fatalf("Passed refresh token to refresher not equal to updated refresh token, got: %v, want: %v", refreshToken, refresh)
		}
		// Only the access token is returned now
		r := &TokenResponse{Access: want}
		return r, expired, nil
	}

	got, err = o.AccessToken()
	if err != nil {
		t.Fatalf("Got error when getting access token on non-empty expired structure and with an empty refresh response: %v", err)
	}
	if got != want {
		t.Fatalf("Access token not equal, Got: %v, Want: %v", got, want)
	}
	if o.token.t.Refresh == "" {
		t.Fatalf("Refresh token is empty after refreshing and getting back an empty refresh")
	}
	if o.token.t.Refresh != prevRefresh {
		t.Fatalf("Refresh token is not equal to previous refresh token after refreshing and getting back an empty refresh token, got: %v, want: %v", o.token.t.Refresh, prevRefresh)
	}
}

func Test_secretJSON(t *testing.T) {
	// Access and refresh tokens should not be present in marshalled JSON
	a := "ineedtobesecret_access"
	r := "ineedtobesecret_refresh"
	o := OAuth{token: &tokenLock{t: &tokenRefresher{Token: Token{Access: a, Refresh: r}}}}
	b, err := json.Marshal(o)
	if err != nil {
		t.Fatalf("Error when marshalling OAuth JSON: %v", err)
	}
	s := string(b)
	// Of course this is a very dumb check, it could be that we are writing in some other serialized format. However, we simply marshal the structure directly. Go just serializes this as a simple string
	if strings.Contains(s, a) {
		t.Fatalf("Serialized OAuth contains Access Token! Serialized: %v, Access Token: %v", s, a)
	}

	if strings.Contains(s, r) {
		t.Fatalf("Serialized OAuth contains Refresh Token! Serialized: %v, Refresh Token: %v", s, a)
	}
}

func Test_AuthURL(t *testing.T) {
	iss := "local"
	auth := "https://127.0.0.1/auth"
	token := "https://127.0.0.1/token"
	id := "client_id"
	o := OAuth{ISS: iss, BaseAuthorizationURL: auth, TokenURL: token}
	s, err := o.AuthURL(id, func(s string) string {
		// We do nothing here are this function is for skipping WAYF
		return s
	})
	if err != nil {
		t.Fatalf("Error in getting OAuth URL: %v", err)
	}

	// Check if the OAuth session has valid values
	if o.session.ClientID != id {
		t.Fatalf("OAuth ClientID not equal, want: %v, got: %v", o.session.ClientID, id)
	}
	if o.session.ISS != iss {
		t.Fatalf("OAuth ISS not equal, want: %v, got: %v", o.session.ISS, iss)
	}
	if o.session.State == "" {
		t.Fatal("No OAuth session state paremeter found")
	}
	if o.session.Verifier == "" {
		t.Fatal("No OAuth session state paremeter found")
	}
	if o.session.ErrChan == nil {
		t.Fatal("No OAuth session error channel found")
	}

	u, err := url.Parse(s)
	if err != nil {
		t.Fatalf("Returned Auth URL cannot be parsed with error: %v", err)
	}

	port, err := o.ListenerPort()
	if err != nil {
		t.Fatalf("Listener port cannot be found with error: %v", err)
	}

	c := []struct {
		query string
		want  string
	}{
		{query: "client_id", want: id},
		{query: "code_challenge_method", want: "S256"},
		{query: "response_type", want: "code"},
		{query: "scope", want: "config"},
		{query: "redirect_uri", want: fmt.Sprintf("http://127.0.0.1:%d/callback", port)},
	}

	q := u.Query()

	// We should have 7 parameters: client_id, challenge method, challenge, response type, scope, state and redirect uri
	if len(q) != 7 {
		t.Fatalf("Total query parameters is not 7, url: %v, total params: %v", u, len(q))
	}

	for _, v := range c {
		p := q.Get(v.query)
		if p != v.want {
			t.Fatalf("Parameter: %v, not equal, want: %v, got: %v", v.query, v.want, p)
		}
	}
}
