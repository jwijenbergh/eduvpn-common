// Package server implements functions that have to deal with server interaction
package server

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/eduvpn/eduvpn-common/internal/api"
	"github.com/eduvpn/eduvpn-common/internal/api/profiles"
	v2 "github.com/eduvpn/eduvpn-common/internal/config/v2"
	"github.com/eduvpn/eduvpn-common/types/protocol"
	srvtypes "github.com/eduvpn/eduvpn-common/types/server"
)

// Server is the struct for a single server
type Server struct {
	identifier string
	t          srvtypes.Type
	apiw       *api.API
	storage    *v2.V2
}

// ErrInvalidProfile is an error that is returned when an invalid profile has been chosen
var ErrInvalidProfile = errors.New("invalid profile")

// NewServer creates a new server
func (s *Servers) NewServer(identifier string, t srvtypes.Type, api *api.API) Server {
	return Server{
		identifier: identifier,
		t:          t,
		apiw:       api,
		storage:    s.config,
	}
}

// Profiles gets the profiles for the server
// It only does a /info network request if the profiles have not been cached
// force indicates whether or not the profiles should be fetched fresh
func (s *Server) Profiles(ctx context.Context) (*profiles.Info, error) {
	a, err := s.api()
	if err != nil {
		return nil, err
	}
	// Otherwise get fresh profiles and set the cache
	prfs, err := a.Info(ctx)
	if err != nil {
		return nil, err
	}
	err = s.SetProfileList(prfs.Public())
	if err != nil {
		return nil, err
	}
	return prfs, nil
}

func (s *Server) api() (*api.API, error) {
	if s.apiw == nil {
		return nil, errors.New("no API object found")
	}
	return s.apiw, nil
}

func (s *Server) findProfile(ctx context.Context, wgSupport bool) (*profiles.Profile, error) {
	// Get the profiles by ignoring the cache
	prfs, err := s.Profiles(ctx)
	if err != nil {
		return nil, err
	}

	// No profiles available
	if prfs.Len() == 0 {
		return nil, errors.New("the server has no available profiles for your account")
	}

	// No WireGuard support, we have to filter the profiles that only have WireGuard
	if !wgSupport {
		prfs = prfs.FilterWireGuard()
	}

	var chosenP profiles.Profile

	n := prfs.Len()
	switch n {
	// If we now get no profiles then that means a profile with only WireGuard was removed
	case 0:
		return nil, errors.New("the server has only WireGuard profiles but the client does not support WireGuard")
	case 1:
		// Only one profile, make sure it is set
		chosenP = prfs.MustIndex(0)
	default:
		// Profile doesn't exist
		prID, err := s.ProfileID()
		if err != nil {
			return nil, err
		}
		v := prfs.Get(prID)
		if v == nil {
			return nil, ErrInvalidProfile
		}
		chosenP = *v
	}
	return &chosenP, nil
}

func (s *Server) connect(ctx context.Context, wgSupport bool, pTCP bool) (*srvtypes.Configuration, error) {
	a, err := s.api()
	if err != nil {
		return nil, err
	}

	// find a suitable profile to connect
	chosenP, err := s.findProfile(ctx, wgSupport)
	if err != nil {
		return nil, err
	}
	err = s.SetProfileID(chosenP.ID)
	if err != nil {
		return nil, err
	}

	protos := []protocol.Protocol{protocol.OpenVPN}
	if wgSupport {
		protos = append(protos, protocol.WireGuard)
	}
	// If the client supports WireGuard and the profile supports both protocols we remove openvpn from client support if EDUVPN_PREFER_WG is set to "1"
	// This also only happens if prefer TCP is set to false
	// TODO: remove the prefer TCP check when we have implemented proxyguard
	if wgSupport && os.Getenv("EDUVPN_PREFER_WG") == "1" {
		if chosenP.HasWireGuard() && chosenP.HasOpenVPN() {
			protos = []protocol.Protocol{protocol.WireGuard}
		}
	}
	// SAFETY: chosenP is guaranteed to be non-nil
	apicfg, err := a.Connect(ctx, *chosenP, protos, pTCP)
	if err != nil {
		return nil, err
	}
	err = s.SetExpireTime(apicfg.Expires)
	if err != nil {
		return nil, err
	}
	var proxy *srvtypes.Proxy
	if apicfg.Proxy != nil {
		proxy = &srvtypes.Proxy{
			SourcePort: apicfg.Proxy.SourcePort,
			Listen:     apicfg.Proxy.Listen,
			Peer:       apicfg.Proxy.Peer,
		}
	}
	return &srvtypes.Configuration{
		VPNConfig:        apicfg.Configuration,
		Protocol:         apicfg.Protocol,
		DefaultGateway:   chosenP.DefaultGateway,
		DNSSearchDomains: chosenP.DNSSearchDomains,
		ShouldFailover:   chosenP.ShouldFailover() && !pTCP,
		Proxy:            proxy,
	}, nil
}

// Disconnect sends an API /disconnect to the server
func (s *Server) Disconnect(ctx context.Context) error {
	a, err := s.api()
	if err != nil {
		return err
	}
	return a.Disconnect(ctx)
}

func (s *Server) cfgServer() (*v2.Server, error) {
	if s.storage == nil {
		return nil, errors.New("cannot get server, no configuration passed")
	}
	return s.storage.GetServer(s.identifier, s.t)
}

// SetProfileID sets the profile id `id` for the server
func (s *Server) SetProfileID(id string) error {
	cs, err := s.cfgServer()
	if err != nil {
		return err
	}
	cs.Profiles.Current = id
	return nil
}

// SetProfileList sets the profile list `prfs` for the server
func (s *Server) SetProfileList(prfs srvtypes.Profiles) error {
	cs, err := s.cfgServer()
	if err != nil {
		return err
	}
	cs.Profiles.Map = prfs.Map
	return nil
}

// SetExpireTime sets the time `et` when the VPN expires
func (s *Server) SetExpireTime(et time.Time) error {
	cs, err := s.cfgServer()
	if err != nil {
		return err
	}
	cs.ExpireTime = et
	return nil
}

// ProfileID gets the profile ID for the server
func (s *Server) ProfileID() (string, error) {
	cs, err := s.cfgServer()
	if err != nil {
		return "", err
	}
	return cs.Profiles.Current, nil
}

// SetLocation sets the secure internet location for the server
func (s *Server) SetLocation(loc string) error {
	if s.t != srvtypes.TypeSecureInternet {
		return errors.New("changing secure internet location is only possible when the server is a secure location")
	}
	cs, err := s.cfgServer()
	if err != nil {
		return err
	}
	cs.CountryCode = loc
	return nil
}

// SetCurrent sets the current server in the state file to this one
func (s *Server) SetCurrent() error {
	if s.storage == nil {
		return errors.New("no storage available")
	}
	s.storage.LastChosen = &v2.ServerKey{
		ID: s.identifier,
		T:  s.t,
	}
	return nil
}
