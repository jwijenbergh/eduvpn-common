package server

import (
	"os"
	"time"

	"github.com/eduvpn/eduvpn-common/internal/oauth"
	"github.com/eduvpn/eduvpn-common/internal/wireguard"
	"github.com/go-errors/errors"
)

type Type int8

const (
	CustomServerType Type = iota
	InstituteAccessServerType
	SecureInternetServerType
)

type Server interface {
	OAuth() *oauth.OAuth

	// TemplateAuth returns the authorization URL template function
	TemplateAuth() func(string) string

	// Base returns the server base
	Base() (*Base, error)
}

type EndpointList struct {
	API           string `json:"api_endpoint"`
	Authorization string `json:"authorization_endpoint"`
	Token         string `json:"token_endpoint"`
}

type EndpointsVersions struct {
	V2 EndpointList `json:"http://eduvpn.org/api#2"`
	V3 EndpointList `json:"http://eduvpn.org/api#3"`
}

// Endpoints defines the json format for /.well-known/vpn-user-portal".
type Endpoints struct {
	API EndpointsVersions `json:"api"`
	V   string            `json:"v"`
}

func UpdateTokens(srv Server, t oauth.Token) {
	srv.OAuth().UpdateTokens(t)
}

func OAuthURL(srv Server, name string) (string, error) {
	return srv.OAuth().AuthURL(name, srv.TemplateAuth())
}

func OAuthExchange(srv Server) error {
	return srv.OAuth().Exchange()
}

func HeaderToken(srv Server) (string, error) {
	return srv.OAuth().AccessToken()
}

func MarkTokenExpired(srv Server) {
	srv.OAuth().SetTokenExpired()
}

func MarkTokensForRenew(srv Server) {
	srv.OAuth().SetTokenRenew()
}

func NeedsRelogin(srv Server) bool {
	_, err := HeaderToken(srv)
	return err != nil
}

func CancelOAuth(srv Server) {
	srv.OAuth().Cancel()
}

func CurrentProfile(srv Server) (*Profile, error) {
	b, err := srv.Base()
	if err != nil {
		return nil, err
	}
	pID := b.Profiles.Current
	for _, profile := range b.Profiles.Info.ProfileList {
		if profile.ID == pID {
			return &profile, nil
		}
	}

	return nil, errors.Errorf("profile not found: " + pID)
}

func ValidProfiles(srv Server, wireguardSupport bool) (*ProfileInfo, error) {
	// No error wrapping here otherwise we wrap it too much
	b, err := srv.Base()
	if err != nil {
		return nil, err
	}
	ps := b.ValidProfiles(wireguardSupport)
	if len(ps.Info.ProfileList) == 0 {
		return nil, errors.Errorf("no profiles found with supported protocols")
	}
	return &ps, nil
}

type ConfigData struct {
	// The configuration
	Config string

	// The type of configuration
	Type string

	// The tokens
	Tokens oauth.Token
}

func wireguardGetConfig(srv Server, preferTCP bool, openVPNSupport bool) (*ConfigData, error) {
	b, err := srv.Base()
	if err != nil {
		return nil, err
	}

	pID := b.Profiles.Current
	key, err := wireguard.GenerateKey()
	if err != nil {
		return nil, err
	}

	pub := key.PublicKey().String()
	cfg, proto, exp, err := APIConnectWireguard(srv, pID, pub, preferTCP, openVPNSupport)
	if err != nil {
		return nil, err
	}

	// Store start and end time
	b.StartTime = time.Now()
	b.EndTime = exp

	if proto == "wireguard" {
		// This needs the go code a way to identify a connection
		// Use the uuid of the connection e.g. on Linux
		// This needs the client code to call the go code

		cfg = wireguard.ConfigAddKey(cfg, key)
	}

	t := oauth.Token{}
	o := srv.OAuth()
	if o != nil {
		t = o.Token()
	}

	return &ConfigData{Config: cfg, Type: proto, Tokens: t}, nil
}

func openVPNGetConfig(srv Server, preferTCP bool) (*ConfigData, error) {
	b, err := srv.Base()
	if err != nil {
		return nil, err
	}
	pid := b.Profiles.Current
	cfg, exp, err := APIConnectOpenVPN(srv, pid, preferTCP)
	if err != nil {
		return nil, err
	}

	// Store start and end time
	b.StartTime = time.Now()
	b.EndTime = exp

	t := oauth.Token{}

	o := srv.OAuth()
	if o != nil {
		t = o.Token()
	}

	return &ConfigData{Config: cfg, Type: "openvpn", Tokens: t}, nil
}

func HasValidProfile(srv Server, wireguardSupport bool) (bool, error) {
	// Get new profiles using the info call
	// This does not override the current profile
	err := APIInfo(srv)
	if err != nil {
		return false, err
	}

	b, err := srv.Base()
	if err != nil {
		return false, err
	}

	// If there was a profile chosen and it doesn't exist anymore, reset it
	if b.Profiles.Current != "" {
		if _, err = CurrentProfile(srv); err != nil {
			b.Profiles.Current = ""
		}
	}

	if len(b.Profiles.Info.ProfileList) != 1 && b.Profiles.Current == "" {
		return false, nil
	}

	// Set the current profile if there is only one profile or profile is already selected
	// Set the first profile if none is selected
	if b.Profiles.Current == "" {
		b.Profiles.Current = b.Profiles.Info.ProfileList[0].ID
	}
	p, err := CurrentProfile(srv)
	// shouldn't happen
	if err != nil {
		return false, err
	}
	// Profile does not support OpenVPN but the client also doesn't support WireGuard
	if !p.SupportsOpenVPN() && !wireguardSupport {
		return false, nil
	}
	return true, nil
}

func RefreshEndpoints(srv Server) error {
	// Re-initialize the endpoints
	// TODO: Make this a warning instead?
	b, err := srv.Base()
	if err != nil {
		return err
	}

	return b.InitializeEndpoints()
}

func Config(server Server, wireguardSupport bool, preferTCP bool) (*ConfigData, error) {
	p, err := CurrentProfile(server)
	if err != nil {
		return nil, err
	}

	ovpn := p.SupportsOpenVPN()
	wg := p.SupportsWireguard() && wireguardSupport

	// If we don't prefer TCP and this profile and client supports wireguard,
	// we disable openvpn if the EDUVPN_PREFER_WG environment variable is set
	// This is useful to force WireGuard if the profile supports both OpenVPN and WireGuard but the server still prefers OpenVPN
	if !preferTCP && wg {
		if os.Getenv("EDUVPN_PREFER_WG") == "1" {
			ovpn = false
		}
	}

	var cfg *ConfigData

	switch {
	// The config supports wireguard and optionally openvpn
	case wg:
		// A wireguard connect call needs to generate a wireguard key and add it to the config
		// Also the server could send back an OpenVPN config if it supports OpenVPN
		cfg, err = wireguardGetConfig(server, preferTCP, ovpn)
	//  The config only supports OpenVPN
	case ovpn:
		cfg, err = openVPNGetConfig(server, preferTCP)
		// The config supports no available protocol because the profile only supports WireGuard but the client doesn't
	default:
		return nil, errors.New("no supported protocol found")
	}

	// Add script security 0 to disable OpenVPN scripts
	// The client may override this but we provide the default protection here
	if err == nil && cfg.Type == "openvpn" {
		cfg.Config += "\nscript-security 0"
	}
	return cfg, err
}

func Disconnect(server Server) error {
	return APIDisconnect(server)
}
