package custom

import (
	"context"

	"github.com/eduvpn/eduvpn-common/internal/server/api"
	"github.com/eduvpn/eduvpn-common/internal/server/base"
	"github.com/eduvpn/eduvpn-common/internal/server/institute"
	"github.com/eduvpn/eduvpn-common/types/server"
)

type (
	Server  = institute.Server
	Servers = institute.Servers
)

func New(ctx context.Context, url string) (*Server, error) {
	b := base.Base{
		URL:         url,
		DisplayName: map[string]string{"en": url},
		Type:        server.TypeCustom,
	}
	if err := api.Endpoints(ctx, &b); err != nil {
		return nil, err
	}
	API := b.Endpoints.API.V3

	s := &Server{Basic: b}
	s.Auth.Init(url, API.Authorization, API.Token)
	return s, nil
}
