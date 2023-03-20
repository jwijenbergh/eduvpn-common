package server

import (
	"github.com/eduvpn/eduvpn-common/internal/oauth"
	"github.com/go-errors/errors"
)

type InstituteAccessServer struct {
	// An instute access server has its own OAuth
	Auth oauth.OAuth `json:"oauth"`

	// Embed the server base
	Basic Base `json:"base"`
}

type InstituteAccessServers struct {
	Map        map[string]*InstituteAccessServer `json:"map"`
	CurrentURL string                            `json:"current_url"`
}

func (ss *Servers) SetInstituteAccess(srv Server) error {
	b, err := srv.Base()
	if err != nil {
		return err
	}

	if b.Type != "institute_access" {
		return errors.Errorf("not an institute access server, URL: %s, type: %s", b.URL, b.Type)
	}

	if _, ok := ss.InstituteServers.Map[b.URL]; ok {
		ss.InstituteServers.CurrentURL = b.URL
		ss.IsType = InstituteAccessServerType
	} else {
		return errors.Errorf("institute access server with URL: %s, is not yet configured", b.URL)
	}
	return nil
}

func (ss *Servers) GetInstituteAccess(url string) (*InstituteAccessServer, error) {
	if srv, ok := ss.InstituteServers.Map[url]; ok {
		return srv, nil
	}
	return nil, errors.Errorf("no institute access server with URL: %s", url)
}

func (ss *Servers) RemoveInstituteAccess(url string) {
	ss.InstituteServers.Remove(url)
}

func (iass *InstituteAccessServers) Remove(url string) {
	// Reset the current url
	if iass.CurrentURL == url {
		iass.CurrentURL = ""
	}

	// Delete the url from the map
	delete(iass.Map, url)
}

func (ias *InstituteAccessServer) TemplateAuth() func(string) string {
	return func(authURL string) string {
		return authURL
	}
}

func (ias *InstituteAccessServer) Base() (*Base, error) {
	return &ias.Basic, nil
}

func (ias *InstituteAccessServer) OAuth() *oauth.OAuth {
	return &ias.Auth
}

func (ias *InstituteAccessServer) init(
	url string,
	name map[string]string,
	srvType string,
	supportContact []string,
) error {
	ias.Basic.URL = url
	ias.Basic.DisplayName = name
	ias.Basic.SupportContact = supportContact
	ias.Basic.Type = srvType
	err := ias.Basic.InitializeEndpoints()
	if err != nil {
		return err
	}
	API := ias.Basic.Endpoints.API.V3
	ias.Auth.Init(url, API.Authorization, API.Token)
	return nil
}
