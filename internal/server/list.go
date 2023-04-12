package server

import (
	"context"

	"github.com/eduvpn/eduvpn-common/internal/server/custom"
	"github.com/eduvpn/eduvpn-common/internal/server/institute"
	"github.com/eduvpn/eduvpn-common/internal/server/secure"
	discotypes "github.com/eduvpn/eduvpn-common/types/discovery"
	srvtypes "github.com/eduvpn/eduvpn-common/types/server"
	"github.com/go-errors/errors"
)

type List struct {
	CustomServers            custom.Servers    `json:"custom_servers"`
	InstituteServers         institute.Servers `json:"institute_servers"`
	SecureInternetHomeServer secure.Server     `json:"secure_internet_home"`
	IsType                   srvtypes.Type     `json:"is_secure_internet"`
}

// HasSecureInternet returns whether or not we have a secure internet server added
func (l *List) HasSecureInternet() bool {
	return len(l.SecureInternetHomeServer.BaseMap) > 0
}

func (l *List) HasSecureLocation() bool {
	return l.SecureInternetHomeServer.CurrentLocation != ""
}

func (l *List) Current() (Server, error) {
	if l.IsType == srvtypes.TypeUnknown {
		return nil, errors.New("no current server")
	}
	if l.IsType == srvtypes.TypeSecureInternet {
		if !l.HasSecureLocation() {
			return nil, errors.Errorf("Current server is secure internet but there is no secure internet location: %v", l.IsType)
		}
		return &l.SecureInternetHomeServer, nil
	}

	if l.IsType == srvtypes.TypeCustom {
		return l.CustomServers.Current()
	}
	return l.InstituteServers.Current()
}

func (l *List) AddCustom(ctx context.Context, url string) (Server, error) {
	srv, err := custom.New(ctx, url)
	if err != nil {
		return nil, err
	}
	l.CustomServers.Add(srv)
	return srv, nil
}

func (l *List) AddInstituteAccess(ctx context.Context, discoServer *discotypes.Server) (Server, error) {
	srv, err := institute.New(ctx, discoServer.BaseURL, discoServer.DisplayName, discoServer.SupportContact)
	if err != nil {
		return nil, err
	}
	l.InstituteServers.Add(srv)
	return srv, nil
}

func (l *List) AddSecureInternet(
	ctx context.Context,
	secureOrg *discotypes.Organization,
	secureServer *discotypes.Server,
) (*secure.Server, error) {
	// If we have specified an organization ID
	// We also need to get an authorization template
	err := l.SecureInternetHomeServer.Init(ctx, secureOrg, secureServer)
	if err != nil {
		return nil, err
	}

	l.IsType = srvtypes.TypeSecureInternet
	return &l.SecureInternetHomeServer, nil
}

func (l *List) SecureInternet(identifier string) (*secure.Server, error) {
	if l.SecureInternetHomeServer.HomeOrganizationID != identifier {
		return nil, errors.Errorf("no secure internet home server with identifier: %s", identifier)
	}
	return &l.SecureInternetHomeServer, nil
}

func (l *List) SetSecureInternet(server Server) error {
	b, err := server.Base()
	if err != nil {
		return err
	}

	if b.Type != srvtypes.TypeSecureInternet {
		return errors.New("not a secure internet server")
	}

	// The location should already be configured
	// TODO: check for location?
	l.IsType = srvtypes.TypeSecureInternet
	return nil
}

func (l *List) RemoveSecureInternet(identifier string) error {
	oid := l.SecureInternetHomeServer.HomeOrganizationID
	if identifier != oid {
		return errors.Errorf("cannot remove secure internet server: identifier: %s, is not equal to the Org ID: %s", identifier, oid)
	}
	// Empty out the struct
	l.SecureInternetHomeServer = secure.Server{}

	// If the current server is secure internet, reset to unknown
	if l.IsType == srvtypes.TypeSecureInternet {
		l.IsType = srvtypes.TypeUnknown
	}
	return nil
}

func (l *List) SetInstituteAccess(srv Server) error {
	b, err := srv.Base()
	if err != nil {
		return err
	}

	if b.Type != srvtypes.TypeInstituteAccess {
		return errors.Errorf("not an institute access server, URL: %s, type: %v", b.URL, b.Type)
	}

	if _, ok := l.InstituteServers.Map[b.URL]; ok {
		l.InstituteServers.CurrentURL = b.URL
		l.IsType = srvtypes.TypeInstituteAccess
	} else {
		return errors.Errorf("institute access server with URL: %s, is not yet configured", b.URL)
	}
	return nil
}

func (l *List) InstituteAccess(url string) (*institute.Server, error) {
	if srv, ok := l.InstituteServers.Map[url]; ok {
		return srv, nil
	}
	return nil, errors.Errorf("no institute access server with URL: %s", url)
}

func (l *List) RemoveInstituteAccess(url string) error {
	// TODO: Reset current to unknown?
	return l.InstituteServers.Remove(url)
}

func (l *List) SetCustom(server Server) error {
	b, err := server.Base()
	if err != nil {
		return err
	}

	if b.Type != srvtypes.TypeCustom {
		return errors.New("not a custom server")
	}

	if _, ok := l.CustomServers.Map[b.URL]; ok {
		l.CustomServers.CurrentURL = b.URL
		l.IsType = srvtypes.TypeCustom
	} else {
		return errors.Errorf("this server is not yet added as a custom server: %s", b.URL)
	}
	return nil
}

func (l *List) CustomServer(url string) (*institute.Server, error) {
	if srv, ok := l.CustomServers.Map[url]; ok {
		return srv, nil
	}
	return nil, errors.Errorf("failed to get institute access server - no custom server with URL '%s'", url)
}

func (l *List) RemoveCustom(url string) error {
	// TODO: Reset current to unknown?
	return l.CustomServers.Remove(url)
}
