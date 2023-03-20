// Package client implements the public interface for creating eduVPN/Let's Connect! clients
package client

import (
	"fmt"
	"strings"
	"sync"

	"github.com/eduvpn/eduvpn-common/internal/config"
	"github.com/eduvpn/eduvpn-common/internal/discovery"
	"github.com/eduvpn/eduvpn-common/internal/failover"
	"github.com/eduvpn/eduvpn-common/internal/fsm"
	"github.com/eduvpn/eduvpn-common/internal/http"
	"github.com/eduvpn/eduvpn-common/internal/log"
	"github.com/eduvpn/eduvpn-common/internal/oauth"
	"github.com/eduvpn/eduvpn-common/internal/server"
	"github.com/eduvpn/eduvpn-common/types"
	"github.com/eduvpn/eduvpn-common/types/protocol"
	"github.com/go-errors/errors"
)

type (
	// ServerBase is an alias to the internal ServerBase
	// This contains the details for each server.
	ServerBase = server.Base
)

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

// Client is the main struct for the VPN client.
type Client struct {
	// The name of the client
	Name string `json:"-"`

	// The chosen server
	Servers server.Servers `json:"servers"`

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

	// The Failover monitor for the current VPN connection
	Failover *failover.DroppedConMon

	locationWg sync.WaitGroup
	profileWg  sync.WaitGroup
}

// Register initializes the clientwith the following parameters:
//   - name: the name of the client
//   - directory: the directory where the config files are stored. Absolute or relative
//   - stateCallback: the callback function for the FSM that takes two states (old and new) and the data as an interface
//   - debug: whether or not we want to enable debugging
//
// It returns an error if initialization failed, for example when discovery cannot be obtained and when there are no servers.
func (c *Client) Register(
	name string,
	version string,
	directory string,
	stateCallback func(FSMStateID, FSMStateID, interface{}) bool,
	debug bool,
) (err error) {
	defer func() {
		if err != nil {
			c.logError(err)
		}
	}()

	if !c.InFSMState(StateDeregistered) {
		return errors.Errorf("fsm attempt to register while in '%v'", c.FSM.Current)
	}

	if !isAllowedClientID(name) {
		return errors.Errorf("client ID is not allowed: '%v', see https://git.sr.ht/~fkooman/vpn-user-portal/tree/v3/item/src/OAuth/VpnClientDb.php for a list of allowed IDs", name)
	}

	if len([]rune(version)) > 20 {
		return errors.Errorf("version is not allowed: '%s', must be max 20 characters", version)
	}

	http.RegisterAgent(userAgentName(name), version)

	c.Name = name

	// Initialize the logger
	lvl := log.LevelWarning
	if debug {
		lvl = log.LevelDebug
	}

	if err = log.Logger.Init(lvl, directory); err != nil {
		return err
	}

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

	// Go to the No Server state with the saved servers after we're done
	defer c.FSM.GoTransitionWithData(StateNoServer, c.Servers)

	// Let's Connect! doesn't care about discovery
	if c.isLetsConnect() {
		return nil
	}

	// Check if we are able to fetch discovery, and log if something went wrong
	if _, err := c.DiscoServers(); err != nil {
		log.Logger.Warningf("Failed to get discovery servers: %v", err)
	}

	if _, err := c.DiscoOrganizations(); err != nil {
		log.Logger.Warningf("Failed to get discovery organizations: %v", err)
	}

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

// askProfile asks the user for a profile by moving the FSM to the ASK_PROFILE state.
func (c *Client) askProfile(srv server.Server) error {
	ps, err := server.ValidProfiles(srv, c.SupportsWireguard)
	if err != nil {
		return err
	}

	c.profileWg.Add(1)
	if err = c.FSM.GoTransitionRequired(StateAskProfile, convertProfiles(*ps)); err != nil {
		return err
	}
	c.profileWg.Wait()

	return nil
}

// DiscoOrganizations gets the organizations list from the discovery server
// If the list cannot be retrieved an error is returned.
// If this is the case then a previous version of the list is returned if there is any.
// This takes into account the frequency of updates, see: https://github.com/eduvpn/documentation/blob/v3/SERVER_DISCOVERY.md#organization-list.
func (c *Client) DiscoOrganizations() (orgs *types.DiscoveryOrganizations, err error) {
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

	return c.Discovery.Organizations()
}

// DiscoServers gets the servers list from the discovery server
// If the list cannot be retrieved an error is returned.
// If this is the case then a previous version of the list is returned if there is any.
// This takes into account the frequency of updates, see: https://github.com/eduvpn/documentation/blob/v3/SERVER_DISCOVERY.md#server-list.
func (c *Client) DiscoServers() (dss *types.DiscoveryServers, err error) {
	defer func() {
		if err != nil {
			c.logError(err)
		}
	}()

	// Not supported with Let's Connect!
	if c.isLetsConnect() {
		return nil, errors.Errorf("discovery with Let's Connect is not supported")
	}

	return c.Discovery.Servers()
}

// ExpiryTimes returns the different Unix timestamps regarding expiry
// - The time starting at which the renew button should be shown, after 30 minutes and less than 24 hours
// - The time starting at which the countdown button should be shown, less than 24 hours
// - The list of times where notifications should be shown
// These times are reset when the VPN gets disconnected
func (c *Client) ExpiryTimes() (*types.Expiry, error) {
	// Get current expiry time
	srv, err := c.Servers.GetCurrentServer()
	if err != nil {
		c.logError(err)
		return nil, err
	}
	b, err := srv.Base()
	if err != nil {
		c.logError(err)
		return nil, err
	}

	bT := b.RenewButtonTime()
	cT := b.CountdownTime()
	nT := b.NotificationTimes()
	return &types.Expiry{
		StartTime: b.StartTime.Unix(),
		EndTime: b.EndTime.Unix(),
		ButtonTime: bT,
		CountdownTime: cT,
		NotificationTimes: nT,
	}, nil
}

func convertProfiles(profiles server.ProfileInfo) types.Profiles {
	m := make(map[string]types.Profile)
	for _, p := range profiles.Info.ProfileList {
		var protocols []protocol.Protocol
		// loop through all protocol strings
		for _, ps := range p.VPNProtoList {
			protocols = append(protocols, protocol.New(ps))
		}
		m[p.ID] = types.Profile{
			DisplayName: map[string]string{
				"en": p.DisplayName,
			},
			Protocols: protocols,
		}
	}
	return types.Profiles{Map: m, Current: profiles.Current}
}

func convertGeneric(server server.InstituteAccessServer) (*types.GenericServer, error) {
	b, err := server.Base()
	if err != nil {
		return nil, err
	}
	return &types.GenericServer{
		DisplayName: b.DisplayName,
		Identifier:  b.URL,
		Profiles:    convertProfiles(b.Profiles),
	}, nil
}

// TODO: CLEAN THIS UP
func (c *Client) ServerList() (*types.ServerList, error) {
	custom := c.Servers.CustomServers
	var customServers []types.GenericServer
	for _, v := range custom.Map {
		if v == nil {
			return nil, errors.New("found nil value in custom server map")
		}
		conv, err := convertGeneric(*v)
		if err != nil {
			return nil, errors.Errorf("failed to convert custom server for public type: %v", err)
		}
		customServers = append(customServers, *conv)
	}
	institute := c.Servers.InstituteServers
	var instituteServers []types.InstituteServer
	for _, v := range institute.Map {
		if v == nil {
			return nil, errors.New("found nil value in institute server map")
		}
		conv, err := convertGeneric(*v)
		if err != nil {
			return nil, errors.Errorf("failed to convert institute server for public type: %v", err)
		}
		instituteServers = append(instituteServers, types.InstituteServer{
			GenericServer: *conv,
			// TODO: delisted
			Delisted: false,
		})
	}

	var secureInternet *types.SecureInternetServer
	if c.Servers.HasSecureInternet() {
		b, err := c.Servers.SecureInternetHomeServer.Base()
		if err == nil {
			generic := types.GenericServer{
				DisplayName: b.DisplayName,
				Identifier:  b.URL,
				Profiles:    convertProfiles(b.Profiles),
			}
			cc := c.Servers.SecureInternetHomeServer.CurrentLocation
			secureInternet = &types.SecureInternetServer{
				GenericServer: generic,
				CountryCode:   cc,
				// TODO: delisted
				Delisted: false,
			}
		}

	}
	return &types.ServerList{
		Institutes:     instituteServers,
		SecureInternet: secureInternet,
		Custom:         customServers,
	}, nil
}

// TODO: CLEAN THIS UP
func (c *Client) CurrentServer() (*types.CurrentServer, error) {
	srvs := c.Servers

	switch srvs.IsType {
	case server.InstituteAccessServerType:
		curr, err := srvs.GetInstituteAccess(srvs.InstituteServers.CurrentURL)
		if err != nil {
			return nil, err
		}
		conv, err := convertGeneric(*curr)
		if err != nil {
			return nil, err
		}
		return &types.CurrentServer{
			Institute: &types.InstituteServer{
				GenericServer: *conv,
				// TODO: delisted
				Delisted: false,
			},
			Type: types.SERVER_INSTITUTE_ACCESS,
		}, nil
	case server.CustomServerType:
		curr, err := srvs.GetCustomServer(srvs.CustomServers.CurrentURL)
		if err != nil {
			return nil, err
		}
		conv, err := convertGeneric(*curr)
		if err != nil {
			return nil, err
		}
		return &types.CurrentServer{
			Custom: conv,
			Type:   types.SERVER_CUSTOM,
		}, nil
	case server.SecureInternetServerType:
		b, err := c.Servers.SecureInternetHomeServer.Base()
		if err != nil {
			return nil, err
		}
		generic := types.GenericServer{
			DisplayName: b.DisplayName,
			Identifier:  c.Servers.SecureInternetHomeServer.HomeOrganizationID,
			Profiles:    convertProfiles(b.Profiles),
		}
		cc := c.Servers.SecureInternetHomeServer.CurrentLocation
		return &types.CurrentServer{
			SecureInternet: &types.SecureInternetServer{
				GenericServer: generic,
				CountryCode:   cc,
				// TODO: delisted
				Delisted: false,
			},
			Type: types.SERVER_SECURE_INTERNET,
		}, nil
	default:
		return nil, errors.New("current server not found")
	}
}
