package oauth

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/eduvpn/eduvpn-common/internal/log"
	"github.com/eduvpn/eduvpn-common/types/server"
	"github.com/go-errors/errors"
)

// TokenResponse defines the OAuth response from the server that includes the tokens.
type TokenResponse struct {
	// Access is the access token returned by the server
	Access string `json:"access_token"`

	// Refresh token is the refresh token returned by the server
	Refresh string `json:"refresh_token"`

	// Type indicates which type of tokens we have
	Type string `json:"token_type"`

	// Expires is the expires time returned by the server
	Expires int64 `json:"expires_in"`
}

// The public type that can be passed to an update function
// It contains our access and refresh tokens with a timestamp
type Token struct {
	// Access is the Access token returned by the server
	Access string

	// Refresh token is the Refresh token returned by the server
	Refresh string

	// ExpiredTimestamp is the Expires field but converted to a Go timestamp
	ExpiredTimestamp time.Time
}

func (t *Token) Public() server.Tokens {
	return server.Tokens{
		Access:  t.Access,
		Refresh: t.Refresh,
		Expires: t.ExpiredTimestamp.Unix(),
	}
}

// tokenRefresher is a structure that contains our access and refresh tokens and a timestamp when they expire.
// Additionally, it contains the refresher to get new tokens
type tokenRefresher struct {
	Token
	// Refresher is the function that refreshes the token
	Refresher func(context.Context, string) (*TokenResponse, time.Time, error)
}

// tokenLock is a wrapper around token that protects it with a lock
type tokenLock struct {
	// Protects t
	mu sync.Mutex

	// The token fields protected by the lock
	// This token struct contains a refresher
	t *tokenRefresher
}

// Access gets the OAuth access token used for contacting the server API
// It returns the access token as a string, possibly obtained fresh using the refresher
// If the token cannot be obtained, an error is returned and the token is an empty string.
func (l *tokenLock) Access(ctx context.Context) (string, error) {
	log.Logger.Debugf("Getting access token")
	l.mu.Lock()
	defer l.mu.Unlock()

	// The tokens are not expired yet
	// So they should be valid, re-login not neede
	if !l.expired() {
		log.Logger.Debugf("Access token is not expired, returning")
		return l.t.Access, nil
	}

	// Check if refresh is even possible by doing a simple check if the refresh token is empty
	// This is not needed but reduces API calls to the server
	if l.t.Refresh == "" {
		log.Logger.Debugf("Refresh token is empty, returning error")
		return "", errors.Wrap(&TokensInvalidError{Cause: "no refresh token is present"}, 0)
	}

	// Otherwise refresh and then later return the access token if we are successful
	tr, s, err := l.t.Refresher(ctx, l.t.Refresh)
	if err != nil {
		log.Logger.Debugf("Got a refresh token error: %v", err)
		// We have failed to ensure the tokens due to refresh not working
		return "", errors.Wrap(
			&TokensInvalidError{Cause: fmt.Sprintf("tokens failed refresh with error: %v", err)}, 0)
	}
	if tr == nil {
		log.Logger.Debugf("No token response after refreshing")
		return "", errors.New("No token response after refreshing")
	}
	// store the previous refresh token
	pr := l.t.Refresh
	// get the response as a non-pointer
	r := *tr
	e := s.Add(time.Second * time.Duration(r.Expires))
	t := Token{Access: r.Access, Refresh: r.Refresh, ExpiredTimestamp: e}
	l.updateInternal(t)
	// set the previous refresh token if the new one is empty
	// This is for 2.x servers
	if l.t.Refresh == "" {
		log.Logger.Debugf("The previous refresh token is set as the response had no refresh token")
		l.t.Refresh = pr
	}
	return l.t.Access, nil
}

// UpdateResponse updates the structure using the server response and locks
func (l *tokenLock) UpdateResponse(r TokenResponse, s time.Time) {
	l.mu.Lock()
	e := s.Add(time.Second * time.Duration(r.Expires))
	t := Token{Access: r.Access, Refresh: r.Refresh, ExpiredTimestamp: e}
	l.updateInternal(t)
	l.mu.Unlock()
}

// updateInternal updates the token structure internally but does not lock
func (l *tokenLock) updateInternal(r Token) {
	l.t.Access = r.Access
	l.t.Refresh = r.Refresh
	l.t.ExpiredTimestamp = r.ExpiredTimestamp
}

// Update updates the token structure using the internal function but locks
func (l *tokenLock) Update(r Token) {
	l.mu.Lock()
	l.updateInternal(r)
	l.mu.Unlock()
}

// Get gets the tokens into a public struct
func (l *tokenLock) Get() Token {
	// TODO: Check nil?
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.t.Token
}

// SetExpired overrides the timestamp to the current time
// This marks the tokens as expired
func (l *tokenLock) SetExpired() {
	l.mu.Lock()
	l.t.ExpiredTimestamp = time.Now()
	l.mu.Unlock()
}

// expired checks if the access token is expired.
// This is only called internally and thus does not lock
func (l *tokenLock) expired() bool {
	now := time.Now()
	return !now.Before(l.t.ExpiredTimestamp)
}
