package server

import (
	"github.com/eduvpn/eduvpn-common/types"
	"github.com/go-errors/errors"
)

// TODO: Have a dedicated type for custom servers
type Servers struct {
	// A custom server is just an institute access server under the hood
	CustomServers            InstituteAccessServers   `json:"custom_servers"`
	InstituteServers         InstituteAccessServers   `json:"institute_servers"`
	SecureInternetHomeServer SecureInternetHomeServer `json:"secure_internet_home"`
	IsType                   Type                     `json:"is_secure_internet"`
}

// HasSecureInternet returns whether or not we have a secure internet server added
func (ss *Servers) HasSecureInternet() bool {
	return len(ss.SecureInternetHomeServer.BaseMap) > 0
}

func (ss *Servers) AddSecureInternet(
	secureOrg *types.DiscoveryOrganization,
	secureServer *types.DiscoveryServer,
) (Server, error) {
	// If we have specified an organization ID
	// We also need to get an authorization template
	err := ss.SecureInternetHomeServer.init(secureOrg, secureServer)
	if err != nil {
		return nil, err
	}

	ss.IsType = SecureInternetServerType
	return &ss.SecureInternetHomeServer, nil
}

func (ss *Servers) GetCurrentServer() (Server, error) {
	// TODO(jwijenbergh): Almost certainly the return type should be pointer (*Server)
	if ss.IsType == SecureInternetServerType {
		if !ss.HasSecureLocation() {
			return nil, errors.Errorf("ss.IsType = %v; ss.HasSecureLocation() = false", ss.IsType)
		}
		return &ss.SecureInternetHomeServer, nil
	}

	srvs := &ss.InstituteServers

	if ss.IsType == CustomServerType {
		srvs = &ss.CustomServers
	}
	if srvs.Map == nil {
		return nil, errors.Errorf("srvs.Map is nil")
	}

	srv, ok := srvs.Map[srvs.CurrentURL]
	if !ok || srv == nil {
		return nil, errors.Errorf("server not found")
	}
	return srv, nil
}

func (ss *Servers) addInstituteAndCustom(
	discoServer *types.DiscoveryServer,
	isCustom bool,
) (Server, error) {
	URL := discoServer.BaseURL
	srvs := &ss.InstituteServers
	srvType := InstituteAccessServerType

	if isCustom {
		srvs = &ss.CustomServers
		srvType = CustomServerType
	}

	if srvs.Map == nil {
		srvs.Map = make(map[string]*InstituteAccessServer)
	}

	srv, ok := srvs.Map[URL]

	// initialize the server if it doesn't exist yet
	if !ok {
		srv = &InstituteAccessServer{}
	}

	if err := srv.init(URL, discoServer.DisplayName, discoServer.Type, discoServer.SupportContact); err != nil {
		return nil, err
	}
	srvs.Map[URL] = srv
	ss.IsType = srvType
	return srv, nil
}

func (ss *Servers) AddInstituteAccessServer(
	instituteServer *types.DiscoveryServer,
) (Server, error) {
	return ss.addInstituteAndCustom(instituteServer, false)
}

func (ss *Servers) AddCustomServer(
	customServer *types.DiscoveryServer,
) (Server, error) {
	return ss.addInstituteAndCustom(customServer, true)
}

func (ss *Servers) GetSecureLocation() string {
	return ss.SecureInternetHomeServer.CurrentLocation
}

func (ss *Servers) SetSecureLocation(
	chosenLocationServer *types.DiscoveryServer,
) error {
	// Make sure to add the current location

	if _, err := ss.SecureInternetHomeServer.addLocation(chosenLocationServer); err != nil {
		return err
	}

	ss.SecureInternetHomeServer.CurrentLocation = chosenLocationServer.CountryCode
	return nil
}
