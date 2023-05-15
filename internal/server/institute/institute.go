package institute

import (
	"context"

	"github.com/eduvpn/eduvpn-common/internal/oauth"
	"github.com/eduvpn/eduvpn-common/internal/server/api"
	"github.com/eduvpn/eduvpn-common/internal/server/base"
	"github.com/eduvpn/eduvpn-common/types/server"
	"github.com/go-errors/errors"
)

type Server struct {
	// An instute access server has its own OAuth
	Auth oauth.OAuth `json:"oauth"`

	// Embed the server base
	Basic base.Base `json:"base"`
}

type Servers struct {
	Map        map[string]*Server `json:"map"`
	CurrentURL string             `json:"current_url"`
}

func New(
	ctx context.Context,
	clientID string,
	url string,
	name map[string]string,
	supportContact []string,
) (*Server, error) {
	b := base.Base{
		URL:            url,
		DisplayName:    name,
		SupportContact: supportContact,
		Type:           server.TypeInstituteAccess,
	}
	if err := api.Endpoints(ctx, &b); err != nil {
		return nil, err
	}
	API := b.Endpoints.API.V3

	s := &Server{Basic: b}
	s.Auth.Init(clientID, url, API.Authorization, API.Token)
	return s, nil
}

func (s *Servers) Current() (*Server, error) {
	if s.Map == nil {
		return nil, errors.Errorf("No map is found when getting the current server")
	}

	srv, ok := s.Map[s.CurrentURL]
	if !ok || srv == nil {
		return nil, errors.Errorf("server not found")
	}
	return srv, nil
}

func (s *Servers) Remove(url string) error {
	// check if it is in the map to begin with
	if _, ok := s.Map[url]; ok {
		delete(s.Map, url)
	} else {
		return errors.Errorf("cannot remove URL: %v, not found in list", url)
	}

	// Reset the current url
	if s.CurrentURL == url {
		s.CurrentURL = ""
	}
	return nil
}

func (s *Servers) Add(srv *Server) {
	if s.Map == nil {
		s.Map = make(map[string]*Server)
	}
	s.Map[srv.Basic.URL] = srv
}

func (s *Server) TemplateAuth() func(string) string {
	return func(authURL string) string {
		return authURL
	}
}

func (s *Server) Base() (*base.Base, error) {
	return &s.Basic, nil
}

func (s *Server) OAuth() *oauth.OAuth {
	return &s.Auth
}

func (s *Server) NeedsLocation() bool {
	return false
}

func (s *Server) Public() (interface{}, error) {
	return &server.Server{
		DisplayName: s.Basic.DisplayName,
		Identifier:  s.Basic.URL,
		Profiles:    s.Basic.Profiles.Public(),
	}, nil
}
