package server

import (
	"github.com/eduvpn/eduvpn-common/internal/oauth"
	"github.com/eduvpn/eduvpn-common/internal/util"
	discotypes "github.com/eduvpn/eduvpn-common/types/discovery"
	"github.com/go-errors/errors"
)

// SecureInternetHomeServer secure internet server which has its own OAuth tokens
// It specifies the current location url it is connected to.
type SecureInternetHomeServer struct {
	Auth        oauth.OAuth       `json:"oauth"`
	DisplayName map[string]string `json:"display_name"`

	// The home server has a list of info for each configured server location
	BaseMap map[string]*Base `json:"base_map"`

	// We have the authorization URL template, the home organization ID and the current location
	AuthorizationTemplate string `json:"authorization_template"`
	HomeOrganizationID    string `json:"home_organization_id"`
	CurrentLocation       string `json:"current_location"`
}

func (ss *Servers) GetSecureInternetHomeServer() (*SecureInternetHomeServer, error) {
	if !ss.HasSecureLocation() {
		return nil, errors.Errorf("no secure internet home server")
	}
	return &ss.SecureInternetHomeServer, nil
}

func (ss *Servers) SetSecureInternet(server Server) error {
	b, err := server.Base()
	if err != nil {
		return err
	}

	if b.Type != "secure_internet" {
		return errors.Errorf("not a secure internet server")
	}

	// The location should already be configured
	// TODO: check for location?
	ss.IsType = SecureInternetServerType
	return nil
}

func (ss *Servers) RemoveSecureInternet() {
	// Empty out the struct
	ss.SecureInternetHomeServer = SecureInternetHomeServer{}

	// If the current server is secure internet, default to custom server
	if ss.IsType == SecureInternetServerType {
		ss.IsType = CustomServerType
	}
}

func (s *SecureInternetHomeServer) TemplateAuth() func(string) string {
	return func(authURL string) string {
		return util.ReplaceWAYF(s.AuthorizationTemplate, authURL, s.HomeOrganizationID)
	}
}

func (s *SecureInternetHomeServer) Base() (*Base, error) {
	if s.BaseMap == nil {
		return nil, errors.Errorf("secure internet map not found")
	}

	b, ok := s.BaseMap[s.CurrentLocation]
	if !ok {
		return nil, errors.Errorf("secure internet base with location '%s' not found", s.CurrentLocation)
	}
	return b, nil
}

func (s *SecureInternetHomeServer) OAuth() *oauth.OAuth {
	return &s.Auth
}

func (ss *Servers) HasSecureLocation() bool {
	return ss.SecureInternetHomeServer.CurrentLocation != ""
}

func (s *SecureInternetHomeServer) addLocation(locSrv *discotypes.Server) (*Base, error) {
	// Initialize the base map if it is non-nil
	if s.BaseMap == nil {
		s.BaseMap = make(map[string]*Base)
	}

	// Add the location to the base map
	b, ok := s.BaseMap[locSrv.CountryCode]
	if !ok || b == nil {
		// Create the base to be added to the map
		b = &Base{}
		b.URL = locSrv.BaseURL
		b.DisplayName = s.DisplayName
		b.SupportContact = locSrv.SupportContact
		b.Type = "secure_internet"
		if err := b.InitializeEndpoints(); err != nil {
			return nil, err
		}
	}

	// Ensure it is in the map
	s.BaseMap[locSrv.CountryCode] = b
	return b, nil
}

// Initializes the home server and adds its own location.
func (s *SecureInternetHomeServer) init(
	homeOrg *discotypes.Organization, homeLoc *discotypes.Server,
) error {
	if s.HomeOrganizationID != homeOrg.OrgID {
		// New home organisation, clear everything
		*s = SecureInternetHomeServer{}
	}

	// Make sure to set the organization ID
	s.HomeOrganizationID = homeOrg.OrgID
	s.DisplayName = homeOrg.DisplayName

	// Make sure to set the authorization URL template
	s.AuthorizationTemplate = homeLoc.AuthenticationURLTemplate

	b, err := s.addLocation(homeLoc)
	if err != nil {
		return err
	}

	// Set the current location to the home location if there is none
	if s.CurrentLocation == "" {
		s.CurrentLocation = homeLoc.CountryCode
	}

	// Make sure oauth contains our endpoints
	s.Auth.Init(b.URL, b.Endpoints.API.V3.Authorization, b.Endpoints.API.V3.Token)
	return nil
}
