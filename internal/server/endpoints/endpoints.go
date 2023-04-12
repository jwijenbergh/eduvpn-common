package endpoints

import (
	"net/url"

	"github.com/go-errors/errors"
)

type List struct {
	API           string `json:"api_endpoint"`
	Authorization string `json:"authorization_endpoint"`
	Token         string `json:"token_endpoint"`
}

type Versions struct {
	V2 List `json:"http://eduvpn.org/api#2"`
	V3 List `json:"http://eduvpn.org/api#3"`
}

// Endpoints defines the json format for /.well-known/vpn-user-portal".
type Endpoints struct {
	API Versions `json:"api"`
	V   string   `json:"v"`
}

func (e Endpoints) Validate() error {
	v3 := e.API.V3
	pAPI, err := url.Parse(v3.API)
	if err != nil {
		return errors.WrapPrefix(err, "failed to parse API endpoint", 0)
	}
	pAuth, err := url.Parse(v3.Authorization)
	if err != nil {
		return errors.WrapPrefix(err, "failed to parse API authorization endpoint", 0)
	}
	pToken, err := url.Parse(v3.Token)
	if err != nil {
		return errors.WrapPrefix(err, "failed to parse API token endpoint", 0)
	}
	if pAPI.Scheme != pAuth.Scheme {
		return errors.Errorf("API scheme: '%v', is not equal to authorization scheme: '%v'", pAPI.Scheme, pAuth.Scheme)
	}
	if pAPI.Scheme != pToken.Scheme {
		return errors.Errorf("API scheme: '%v', is not equal to token scheme: '%v'", pAPI.Scheme, pToken.Scheme)
	}
	if pAPI.Host != pAuth.Host {
		return errors.Errorf("API host: '%v', is not equal to authorization host: '%v'", pAPI.Host, pAuth.Host)
	}
	if pAPI.Host != pToken.Host {
		return errors.Errorf("API host: '%v', is not equal to token host: '%v'", pAPI.Host, pToken.Host)
	}
	return nil
}
