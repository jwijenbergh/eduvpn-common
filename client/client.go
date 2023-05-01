// Package client implements the public interface for creating eduVPN/Let's Connect! clients
package client

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/eduvpn/eduvpn-common/internal/config"
	"github.com/eduvpn/eduvpn-common/internal/discovery"
	"github.com/eduvpn/eduvpn-common/internal/failover"
	"github.com/eduvpn/eduvpn-common/internal/fsm"
	"github.com/eduvpn/eduvpn-common/internal/http"
	"github.com/eduvpn/eduvpn-common/internal/log"
	"github.com/eduvpn/eduvpn-common/internal/oauth"
	"github.com/eduvpn/eduvpn-common/internal/server"
	"github.com/eduvpn/eduvpn-common/types/cookie"
	discotypes "github.com/eduvpn/eduvpn-common/types/discovery"
	srvtypes "github.com/eduvpn/eduvpn-common/types/server"
	"github.com/go-errors/errors"
)

// isAllowedClientID checks if the 'clientID' is in the list of allowed client IDs
func isAllowedClientID(clientID string) bool {
	allowList := []string{
		// eduVPN
		"org.eduvpn.app.windows",
		"org.eduvpn.app.android",
		"org.eduvpn.app.ios",
		"org.eduvpn.app.macos",
		"org.eduvpn.app.linux",
		// Let's Connect!
		"org.letsconnect-vpn.app.windows",
		"org.letsconnect-vpn.app.android",
		"org.letsconnect-vpn.app.ios",
		"org.letsconnect-vpn.app.macos",
		"org.letsconnect-vpn.app.linux",
	}
	for _, x := range allowList {
		if x == clientID {
			return true
		}
	}
	return false
}

func userAgentName(clientID string) string {
	switch clientID {
	case "org.eduvpn.app.windows":
		return "eduVPN for Windows"
	case "org.eduvpn.app.android":
		return "eduVPN for Android"
	case "org.eduvpn.app.ios":
		return "eduVPN for iOS"
	case "org.eduvpn.app.macos":
		return "eduVPN for macOS"
	case "org.eduvpn.app.linux":
		return "eduVPN for Linux"
	case "org.letsconnect-vpn.app.windows":
		return "Let's Connect! for Windows"
	case "org.letsconnect-vpn.app.android":
		return "Let's Connect! for Android"
	case "org.letsconnect-vpn.app.ios":
		return "Let's Connect! for iOS"
	case "org.letsconnect-vpn.app.macos":
		return "Let's Connect! for macOS"
	case "org.letsconnect-vpn.app.linux":
		return "Let's Connect! for Linux"
	default:
		return "unknown"
	}
}

func (c *Client) logError(err error) {
	// Logs the error with the same level/verbosity as the error
	if c.Debug {
		log.Logger.Inherit(err, fmt.Sprintf("\nwith stacktrace: %s\n", err.(*errors.Error).ErrorStack()))
	} else {
		log.Logger.Inherit(err, "")
	}
}

func (c *Client) isLetsConnect() bool {
	// see https://git.sr.ht/~fkooman/vpn-user-portal/tree/v3/item/src/OAuth/VpnClientDb.php
	return strings.HasPrefix(c.Name, "org.letsconnect-vpn.app")
}

// Client is the main struct for the VPN client.
type Client struct {
	// The name of the client
	Name string `json:"-"`

	// The chosen server
	Servers server.List `json:"servers"`

	// The list of servers and organizations from disco
	Discovery discovery.Discovery `json:"discovery"`

	// The fsm
	FSM fsm.FSM `json:"-"`

	// The config
	Config config.Config `json:"-"`

	// Whether or not this client supports WireGuard
	SupportsWireguard bool `json:"-"`

	// Whether to enable debugging
	Debug bool `json:"-"`

	// TokenSetter sets the tokens in the client
	TokenSetter func(srv srvtypes.Current, tok srvtypes.Tokens) `json:"-"`

	// TokenGetter gets the tokens from the client
	TokenGetter func(srv srvtypes.Current) *srvtypes.Tokens `json:"-"`
}

func (c *Client) updateTokens(srv server.Server) error {
	if c.TokenGetter == nil {
		return errors.New("no tokken getter defined")
	}
	pSrv, err := c.pubCurrentServer(srv)
	if err != nil {
		return err
	}
	// shouldn't happen
	if pSrv == nil {
		return errors.New("public server is nil when getting tokens")
	}
	tokens := c.TokenGetter(*pSrv)
	if tokens == nil {
		return errors.New("client returned nil for tokens")
	}

	server.UpdateTokens(srv, oauth.Token{
		Access: tokens.Access,
		Refresh: tokens.Refresh,
		ExpiredTimestamp: time.Unix(tokens.Expires, 0),
	})

	return nil
}

func (c *Client) forwardTokens(srv server.Server) error {
	if c.TokenSetter == nil {
		return errors.New("no token setter defined")
	}
	pSrv, err := c.pubCurrentServer(srv)
	if err != nil {
		return err
	}
	if pSrv == nil {
		return errors.New("public server is nil when updating tokens")
	}
	o := srv.OAuth()
	if o == nil {
		return errors.New("oauth was nil when forwarding tokens")
	}
	t := o.Token()
	c.TokenSetter(*pSrv, t.Public())
	return nil
}

// New creates a new client with the following parameters:
//   - name: the name of the client
//   - directory: the directory where the config files are stored. Absolute or relative
//   - stateCallback: the callback function for the FSM that takes two states (old and new) and the data as an interface
//   - debug: whether or not we want to enable debugging
//
// It returns an error if initialization failed, for example when discovery cannot be obtained and when there are no servers.
func New(name string, version string, directory string, stateCallback func(FSMStateID, FSMStateID, interface{}) bool, debug bool) (c *Client, err error) {
	// We create the client by filling fields one by one
	c = &Client{}

	if !isAllowedClientID(name) {
		return nil, errors.Errorf("client ID is not allowed: '%v', see https://git.sr.ht/~fkooman/vpn-user-portal/tree/v3/item/src/OAuth/VpnClientDb.php for a list of allowed IDs", name)
	}

	if len([]rune(version)) > 20 {
		return nil, errors.Errorf("version is not allowed: '%s', must be max 20 characters", version)
	}

	// Initialize the logger
	lvl := log.LevelWarning
	if debug {
		lvl = log.LevelDebug
	}

	if err = log.Logger.Init(lvl, directory); err != nil {
		return nil, err
	}

	// set client name
	c.Name = name

	// register HTTP agent
	http.RegisterAgent(userAgentName(name), version)

	// Initialize the FSM
	c.FSM = newFSM(stateCallback, directory, debug)

	// By default we support wireguard
	c.SupportsWireguard = true

	// Debug only if given
	c.Debug = debug

	// Initialize the Config
	c.Config.Init(directory, "state")

	// Try to load the previous configuration
	if c.Config.Load(&c) != nil {
		// This error can be safely ignored, as when the config does not load, the struct will not be filled
		log.Logger.Infof("Previous configuration not found")
	}

	return c, nil
}

// Registering means updating the FSM to get to the initial state correctly
func (c *Client) Register() error {
	if !c.FSM.InState(StateDeregistered) {
		return errors.Errorf("fsm attempt to register while in '%v'", c.FSM.Current)
	}
	c.FSM.GoTransition(StateNoServer)
	return nil
}

// Deregister 'deregisters' the client, meaning saving the log file and the config and emptying out the client struct.
func (c *Client) Deregister() {
	// Close the log file
	_ = log.Logger.Close()

	// Save the config
	if err := c.Config.Save(&c); err != nil {
		log.Logger.Infof("c.Config.Save failed: %s\nstacktrace:\n%s", err.Error(), err.(*errors.Error).ErrorStack())
	}

	// Empty out the state
	*c = Client{}
}

// DiscoOrganizations gets the organizations list from the discovery server
// If the list cannot be retrieved an error is returned.
// If this is the case then a previous version of the list is returned if there is any.
// This takes into account the frequency of updates, see: https://github.com/eduvpn/documentation/blob/v3/SERVER_DISCOVERY.md#organization-list.
func (c *Client) DiscoOrganizations(ck *cookie.Cookie) (orgs *discotypes.Organizations, err error) {
	defer func() {
		if err != nil {
			c.logError(err)
		}
	}()

	// Not supported with Let's Connect!
	if c.isLetsConnect() {
		return nil, errors.Errorf("discovery with Let's Connect is not supported")
	}

	// Mark organizations as expired if we have not set an organization yet
	if !c.Servers.HasSecureInternet() {
		c.Discovery.MarkOrganizationsExpired()
	}

	// TODO: pass a context
	return c.Discovery.Organizations(ck.Context())
}

// DiscoServers gets the servers list from the discovery server
// If the list cannot be retrieved an error is returned.
// If this is the case then a previous version of the list is returned if there is any.
// This takes into account the frequency of updates, see: https://github.com/eduvpn/documentation/blob/v3/SERVER_DISCOVERY.md#server-list.
func (c *Client) DiscoServers(ck *cookie.Cookie) (dss *discotypes.Servers, err error) {
	defer func() {
		if err != nil {
			c.logError(err)
		}
	}()

	// Not supported with Let's Connect!
	if c.isLetsConnect() {
		return nil, errors.Errorf("discovery with Let's Connect is not supported")
	}

	// TODO: pass a context
	return c.Discovery.Servers(ck.Context())
}

// ExpiryTimes returns the different Unix timestamps regarding expiry
// - The time starting at which the renew button should be shown, after 30 minutes and less than 24 hours
// - The time starting at which the countdown button should be shown, less than 24 hours
// - The list of times where notifications should be shown
// These times are reset when the VPN gets disconnected
func (c *Client) ExpiryTimes() (*srvtypes.Expiry, error) {
	// Get current expiry time
	srv, err := c.Servers.Current()
	if err != nil {
		c.logError(err)
		return nil, err
	}
	b, err := srv.Base()
	if err != nil {
		c.logError(err)
		return nil, err
	}

	if b.StartTime.IsZero() {
		return nil, errors.New("start time is zero, did you get a configuration?")
	}

	bT := b.RenewButtonTime()
	cT := b.CountdownTime()
	nT := b.NotificationTimes()
	return &srvtypes.Expiry{
		StartTime:         b.StartTime.Unix(),
		EndTime:           b.EndTime.Unix(),
		ButtonTime:        bT,
		CountdownTime:     cT,
		NotificationTimes: nT,
	}, nil
}

func (c *Client) locationCallback(ck *cookie.Cookie) error {
	locs := c.Discovery.SecureLocationList()
	errChan := make(chan error)
	go func() {
		err := c.FSM.GoTransitionRequired(StateAskLocation, &srvtypes.RequiredAskTransition{
			C:    ck,
			Data: locs,
		})
		if err != nil {
			errChan <- err
		}
	}()
	loc, err := ck.Receive(errChan)
	if err != nil {
		return err
	}
	err = c.SetSecureLocation(ck, loc)
	if err != nil {
		return err
	}
	t := c.FSM.GoTransition(StateChosenLocation)
	if !t {
		log.Logger.Warningf("transition chosen location not completed")
	}
	return nil
}

func (c *Client) loginCallback(ck *cookie.Cookie, srv server.Server) error {
	url, err := server.OAuthURL(srv, c.Name)
	if err != nil {
		return err
	}
	err = c.FSM.GoTransitionRequired(StateOAuthStarted, url)
	if err != nil {
		return err
	}
	err = server.OAuthExchange(ck.Context(), srv)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) callbacks(ck *cookie.Cookie, srv server.Server, forceauth bool) error {
	// location
	if srv.NeedsLocation() {
		err := c.locationCallback(ck)
		if err != nil {
			return err
		}
	}

	t := c.FSM.GoTransition(StateChosenServer)
	if !t {
		log.Logger.Warningf("transition not completed for chosen server")
	}
	// oauth
	// TODO: This should be ck.Context()
	// But needsrelogin needs a rewrite to support this properly

	// first make sure we get the most up to date tokens from the client
	err := c.updateTokens(srv)
	if err != nil {
		log.Logger.Debugf("failed to get tokens from client: %v", err)
	}
	if server.NeedsRelogin(context.Background(), srv) || forceauth {
		// mark organizations as expired if the server is a secure internet server
		b, berr := srv.Base()
		if berr == nil && b.Type == srvtypes.TypeSecureInternet {
			c.Discovery.MarkOrganizationsExpired()
		}
		err := c.loginCallback(ck, srv)
		if err != nil {
			return err
		}
	}
	t = c.FSM.GoTransition(StateAuthorized)
	if !t {
		log.Logger.Warningf("transition authorized not completed")
	}

	return nil
}

func (c *Client) profileCallback(ck *cookie.Cookie, srv server.Server) error {
	vp, err := server.HasValidProfile(ck.Context(), srv, c.SupportsWireguard)
	if err != nil {
		return err
	}
	if !vp {
		b, err := srv.Base()
		if err != nil {
			return err
		}
		ps := b.Profiles.Public()
		errChan := make(chan error)
		go func() {
			err := c.FSM.GoTransitionRequired(StateAskProfile, &srvtypes.RequiredAskTransition{
				C:    ck,
				Data: ps,
			})
			if err != nil {
				errChan <- err
			}
		}()
		pID, err := ck.Receive(errChan)
		if err != nil {
			return err
		}
		err = server.Profile(srv, pID)
		if err != nil {
			return err
		}
	}
	t := c.FSM.GoTransition(StateChosenProfile)
	if !t {
		log.Logger.Warningf("transition chosen profile not completed")
	}
	return nil
}

// AddServer adds a server with identifier and type
func (c *Client) AddServer(ck *cookie.Cookie, identifier string, _type srvtypes.Type, ni bool) (err error) {
	// If we have failed to add the server, we remove it again
	// We add the server because we can then obtain it in other callback functions
	defer func() {
		if err != nil {
			_ = c.RemoveServer(identifier, _type) //nolint:errcheck
		}
		if !ni {
			c.FSM.GoTransition(StateNoServer)
		}
	}()

	if !ni {
		// Try to go to no server
		c.FSM.GoTransition(StateNoServer)

		// If the transition was not successful, log
		if !c.FSM.InState(StateNoServer) {
			return errors.Errorf("wrong state to add a server: %s", GetStateName(c.FSM.Current))
		}
		t := c.FSM.GoTransition(StateLoadingServer)
		if !t {
			log.Logger.Warningf("transition not completed for loading server")
		}
	}

	identifier, err = http.EnsureValidURL(identifier, _type != srvtypes.TypeSecureInternet)
	if err != nil {
		return err
	}

	var srv server.Server

	switch _type {
	case srvtypes.TypeInstituteAccess:
		dSrv, err := c.Discovery.ServerByURL(identifier, "institute_access")
		if err != nil {
			return err
		}
		srv, err = c.Servers.AddInstituteAccess(ck.Context(), dSrv)
		if err != nil {
			return err
		}
	case srvtypes.TypeSecureInternet:
		dOrg, dSrv, err := c.Discovery.SecureHomeArgs(identifier)
		if err != nil {
			// We mark the organizations as expired because we got an error
			// Note that in the docs it states that it only should happen when the Org ID doesn't exist
			// However, this is nice as well because it also catches the error where the SecureInternetHome server is not found
			c.Discovery.MarkOrganizationsExpired()
			return err
		}
		srv, err = c.Servers.AddSecureInternet(ck.Context(), dOrg, dSrv)
		if err != nil {
			return err
		}
	case srvtypes.TypeCustom:
		srv, err = c.Servers.AddCustom(ck.Context(), identifier)
		if err != nil {
			return err
		}
	default:
		return errors.Errorf("not a valid server type: %v", _type)
	}

	// if we are non interactive, we run no callbacks
	if ni {
		return nil
	}

	// callbacks
	err = c.callbacks(ck, srv, false)
	if err != nil {
		return err
	}
	terr := c.forwardTokens(srv)
	if terr != nil {
		log.Logger.Debugf("failed to forward tokens after adding: %v", terr)
	}
	return nil
}

func (c *Client) config(ck *cookie.Cookie, srv server.Server, pTCP bool, forceAuth bool) (cfg *srvtypes.Configuration, err error) {
	// do the callbacks to ensure valid profile, location and authorization
	err = c.callbacks(ck, srv, forceAuth)
	if err != nil {
		return nil, err
	}

	t := c.FSM.GoTransition(StateRequestConfig)
	if !t {
		log.Logger.Warningf("transition not completed for requesting config")
	}

	err = c.profileCallback(ck, srv)
	if err != nil {
		return nil, err
	}

	cfgS, err := server.Config(ck.Context(), srv, c.SupportsWireguard, pTCP)
	if err != nil {
		return nil, err
	}
	p, err := server.CurrentProfile(srv)
	if err != nil {
		return nil, err
	}
	pcfg := cfgS.Public(p.DefaultGateway)
	if err != nil {
		return nil, err
	}
	return &pcfg, nil
}

func (c *Client) server(identifier string, _type srvtypes.Type) (srv server.Server, setter func(server.Server) error, err error) {
	switch _type {
	case srvtypes.TypeInstituteAccess:
		srv, err = c.Servers.InstituteAccess(identifier)
		setter = c.Servers.SetInstituteAccess
	case srvtypes.TypeSecureInternet:
		srv, err = c.Servers.SecureInternet(identifier)
		setter = c.Servers.SetSecureInternet
	case srvtypes.TypeCustom:
		srv, err = c.Servers.CustomServer(identifier)
		setter = c.Servers.SetCustom
	default:
		return nil, nil, errors.Errorf("not a valid server type: %v", _type)
	}
	return srv, setter, err
}

// GetConfig gets a VPN configuration
func (c *Client) GetConfig(ck *cookie.Cookie, identifier string, _type srvtypes.Type, pTCP bool) (cfg *srvtypes.Configuration, err error) {
	defer func() {
		if err == nil {
			c.FSM.GoTransition(StateGotConfig)
		} else {
			// go back if an error occurred
			c.FSM.GoTransition(StateNoServer)
		}
	}()
	identifier, err = http.EnsureValidURL(identifier, _type != srvtypes.TypeSecureInternet)
	if err != nil {
		return nil, err
	}
	t := c.FSM.GoTransition(StateLoadingServer)
	if !t {
		log.Logger.Warningf("transition not completed for loading server")
	}
	srv, set, err := c.server(identifier, _type)
	if err != nil {
		return nil, err
	}
	// refresh the server endpoints
	err = server.RefreshEndpoints(ck.Context(), srv)

	// If we get a canceled error, return that, otherwise just log the error
	cErr := context.Canceled
	if err != nil {
		if errors.As(err, &cErr) {
			return nil, err
		}

		log.Logger.Warningf("failed to refresh server endpoints: %v", err)
	}

	// get a config and retry with authorization if expired
	cfg, err = c.config(ck, srv, pTCP, false)
	tErr := &oauth.TokensInvalidError{}
	if err != nil && errors.As(err, &tErr) {
		cfg, err = c.config(ck, srv, pTCP, true)
	}

	// tokens might be updated, forward them
	defer func() {
		terr := c.forwardTokens(srv)
		if terr != nil {
			log.Logger.Debugf("failed to forward tokens after get config: %v", terr)
		}
	}()

	// still an error, return nil with the error
	if err != nil {
		return nil, err
	}

	// set the current server
	if err = set(srv); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Client) RemoveServer(identifier string, _type srvtypes.Type) (err error) {
	identifier, err = http.EnsureValidURL(identifier, _type != srvtypes.TypeSecureInternet)
	if err != nil {
		return err
	}
	switch _type {
	case srvtypes.TypeInstituteAccess:
		return c.Servers.RemoveInstituteAccess(identifier)
	case srvtypes.TypeSecureInternet:
		return c.Servers.RemoveSecureInternet(identifier)
	case srvtypes.TypeCustom:
		return c.Servers.RemoveCustom(identifier)
	default:
		return errors.Errorf("not a valid server type: %v", _type)
	}
}

func (c *Client) CurrentServer() (*srvtypes.Current, error) {
	if !c.FSM.InState(StateGotConfig) {
		return nil, errors.Errorf("State: %s, cannot have a current server. Did you get a VPN configuration?", GetStateName(c.FSM.Current))
	}
	srv, err := c.Servers.Current()
	if err != nil {
		return nil, err
	}
	return c.pubCurrentServer(srv)
}

func (c *Client) pubCurrentServer(srv server.Server) (*srvtypes.Current, error) {
	b, err := srv.Base()
	if err != nil {
		return nil, err
	}
	pub, err := srv.Public()
	if err != nil {
		return nil, err
	}
	switch t := pub.(type) {
	case *srvtypes.Server:
		if b.Type == srvtypes.TypeInstituteAccess {
			return &srvtypes.Current{
				Institute: &srvtypes.Institute{
					Server: *t,
					// TODO: delisted
					Delisted: false,
				},
				Type: srvtypes.TypeInstituteAccess,
			}, nil
		}
		return &srvtypes.Current{
			Custom: t,
			Type:   srvtypes.TypeCustom,
		}, nil
	case *srvtypes.SecureInternet:
		t.Locations = c.Discovery.SecureLocationList()
		return &srvtypes.Current{
			SecureInternet: t,
			Type:           srvtypes.TypeSecureInternet,
		}, nil
	default:
		panic("unknown type")
	}
}

// TODO: This should not rely on interface{}
func (c *Client) pubServer(srv server.Server) (interface{}, error) {
	pub, err := srv.Public()
	if err != nil {
		return nil, err
	}
	switch t := pub.(type) {
	case *srvtypes.Server:
		b, err := srv.Base()
		if err != nil {
			return nil, err
		}
		if b.Type == srvtypes.TypeInstituteAccess {
			return &srvtypes.Institute{
				Server: *t,
				// TODO: delisted
				Delisted: false,
			}, nil
		}
		return t, nil
	case *srvtypes.SecureInternet:
		t.Locations = c.Discovery.SecureLocationList()
		return t, nil
	default:
		panic("unknown type")
	}
}

func (c *Client) ServerList() (*srvtypes.List, error) {
	if c.FSM.InState(StateDeregistered) {
		return nil, errors.New("client is not registered")
	}
	var customServers []srvtypes.Server
	for _, v := range c.Servers.CustomServers.Map {
		if v == nil {
			continue
		}
		p, err := c.pubServer(v)
		if err != nil {
			continue
		}
		c, ok := p.(*srvtypes.Server)
		if !ok {
			continue
		}
		customServers = append(customServers, *c)
	}
	var instituteServers []srvtypes.Institute
	for _, v := range c.Servers.InstituteServers.Map {
		if v == nil {
			continue
		}
		p, err := c.pubServer(v)
		if err != nil {
			continue
		}
		i, ok := p.(*srvtypes.Institute)
		if !ok {
			continue
		}
		instituteServers = append(instituteServers, *i)
	}
	var secureInternet *srvtypes.SecureInternet
	if c.Servers.HasSecureInternet() {
		srv := &c.Servers.SecureInternetHomeServer
		p, err := c.pubServer(srv)
		if err == nil {
			s, ok := p.(*srvtypes.SecureInternet)
			if ok {
				secureInternet = s
			}
		}
	}
	return &srvtypes.List{
		Institutes:     instituteServers,
		SecureInternet: secureInternet,
		Custom:         customServers,
	}, nil
}

func (c *Client) SetProfileID(pID string) (err error) {
	srv, err := c.Servers.Current()
	if err != nil {
		return err
	}
	return server.Profile(srv, pID)
}

func (c *Client) Cleanup(ck *cookie.Cookie) (err error) {
	// get the current server
	srv, err := c.Servers.Current()
	if err != nil {
		return err
	}
	err = c.updateTokens(srv)
	if err != nil {
		log.Logger.Debugf("failed to update tokens for disconnect: %v", err)
	}
	err = server.Disconnect(ck.Context(), srv)
	if err != nil {
		return err
	}
	err = c.forwardTokens(srv)
	if err != nil {
		log.Logger.Debugf("failed to forward tokens after disconnect: %v", err)
	}
	return nil
}

func (c *Client) SetSecureLocation(ck *cookie.Cookie, countryCode string) (err error) {
	if c.isLetsConnect() {
		return errors.Errorf("setting a secure internet location with Let's Connect! is not supported")
	}

	if !c.Servers.HasSecureInternet() {
		return errors.Errorf("no secure internet server available to set a location for")
	}

	dSrv, err := c.Discovery.ServerByCountryCode(countryCode)
	if err != nil {
		return err
	}

	return c.Servers.SecureInternetHomeServer.Location(ck.Context(), dSrv)
}

func (c *Client) RenewSession(ck *cookie.Cookie) (err error) {
	srv, err := c.Servers.Current()
	if err != nil {
		return err
	}
	// The server has not been chosen yet, this means that we want to manually renew
	// TODO: is this needed?
	if !c.FSM.InState(StateChosenServer) {
		c.FSM.GoTransition(StateLoadingServer)
		c.FSM.GoTransition(StateChosenServer)
	}
	// update tokens in the end
	defer func() {
		terr := c.forwardTokens(srv)
		if terr != nil {
			log.Logger.Debugf("failed to forward tokens after renew: %v", terr)
		}
	}()
	// TODO: Maybe this can be deleted because we force auth now
	server.MarkTokensForRenew(srv)
	// run the callbacks by forcing auth
	return c.callbacks(ck, srv, true)
}

func (c *Client) StartFailover(ck *cookie.Cookie, gateway string, mtu int, readRxBytes func() (int64, error)) (bool, error) {
	f := failover.New(readRxBytes)

	return f.Start(ck.Context(), gateway, mtu)
}
