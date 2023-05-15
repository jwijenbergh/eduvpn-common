package secure

import (
	"context"

	"github.com/eduvpn/eduvpn-common/internal/oauth"
	"github.com/eduvpn/eduvpn-common/internal/server/api"
	"github.com/eduvpn/eduvpn-common/internal/server/base"
	"github.com/eduvpn/eduvpn-common/internal/util"
	discotypes "github.com/eduvpn/eduvpn-common/types/discovery"
	"github.com/eduvpn/eduvpn-common/types/server"
	"github.com/go-errors/errors"
)

// Server secure internet server which has its own OAuth tokens
// It specifies the current location url it is connected to.
type Server struct {
	Auth        oauth.OAuth       `json:"oauth"`
	DisplayName map[string]string `json:"display_name"`

	// The home server has a list of info for each configured server location
	BaseMap map[string]*base.Base `json:"base_map"`

	// We have the authorization URL template, the home organization ID and the current location
	AuthorizationTemplate string `json:"authorization_template"`
	HomeOrganizationID    string `json:"home_organization_id"`
	CurrentLocation       string `json:"current_location"`
}

func (s *Server) TemplateAuth() func(string) string {
	return func(authURL string) string {
		return util.ReplaceWAYF(s.AuthorizationTemplate, authURL, s.HomeOrganizationID)
	}
}

func (s *Server) Base() (*base.Base, error) {
	if s.BaseMap == nil {
		return nil, errors.Errorf("secure internet map not found")
	}

	b, ok := s.BaseMap[s.CurrentLocation]
	if !ok {
		return nil, errors.Errorf("secure internet base with location '%s' not found", s.CurrentLocation)
	}
	return b, nil
}

func (s *Server) OAuth() *oauth.OAuth {
	return &s.Auth
}

func (s *Server) NeedsLocation() bool {
	if s.CurrentLocation == "" {
		return true
	}
	if len(s.BaseMap) == 0 {
		return true
	}
	return false
}

func (s *Server) addLocation(ctx context.Context, locSrv *discotypes.Server) (*base.Base, error) {
	// Initialize the base map if it is non-nil
	if s.BaseMap == nil {
		s.BaseMap = make(map[string]*base.Base)
	}

	// Add the location to the base map
	b, ok := s.BaseMap[locSrv.CountryCode]
	if !ok || b == nil {
		// Create the base to be added to the map
		b = &base.Base{}
		b.URL = locSrv.BaseURL
		b.DisplayName = s.DisplayName
		b.SupportContact = locSrv.SupportContact
		b.Type = server.TypeSecureInternet
		if err := api.Endpoints(ctx, b); err != nil {
			return nil, err
		}
	}

	// Ensure it is in the map
	s.BaseMap[locSrv.CountryCode] = b
	return b, nil
}

func (s *Server) Location(ctx context.Context, locSrv *discotypes.Server) error {
	if _, err := s.addLocation(ctx, locSrv); err != nil {
		return err
	}
	s.CurrentLocation = locSrv.CountryCode
	return nil
}

// Initializes the home server and adds its own location.
func (s *Server) Init(
	ctx context.Context,
	clientID string,
	homeOrg *discotypes.Organization, homeLoc *discotypes.Server,
) error {
	if s.HomeOrganizationID != homeOrg.OrgID {
		// New home organisation, clear everything
		*s = Server{}
	}

	// Make sure to set the organization ID
	s.HomeOrganizationID = homeOrg.OrgID
	s.DisplayName = homeOrg.DisplayName

	// Make sure to set the authorization URL template
	s.AuthorizationTemplate = homeLoc.AuthenticationURLTemplate

	b, err := s.addLocation(ctx, homeLoc)
	if err != nil {
		return err
	}

	// set the home location as the current
	err = s.Location(ctx, homeLoc)
	if err != nil {
		return err
	}

	// Make sure oauth contains our endpoints
	s.Auth.Init(clientID, b.URL, b.Endpoints.API.V3.Authorization, b.Endpoints.API.V3.Token)
	return nil
}

func (s *Server) Public() (interface{}, error) {
	b, err := s.Base()
	var p server.Profiles
	dn := s.DisplayName
	if err == nil {
		dn = b.DisplayName
		p = b.Profiles.Public()
	}
	return &server.SecureInternet{
		Server: server.Server{
			DisplayName: dn,
			Identifier:  s.HomeOrganizationID,
			Profiles:    p,
		},
		CountryCode: s.CurrentLocation,
	}, nil
}
