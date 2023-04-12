package profile

import (
	"github.com/eduvpn/eduvpn-common/types/protocol"
	"github.com/eduvpn/eduvpn-common/types/server"
)

type Profile struct {
	ID             string   `json:"profile_id"`
	DisplayName    string   `json:"display_name"`
	VPNProtoList   []string `json:"vpn_proto_list"`
	DefaultGateway bool     `json:"default_gateway"`
}

type ListInfo struct {
	ProfileList []Profile `json:"profile_list"`
}

type Info struct {
	Current string   `json:"current_profile"`
	Info    ListInfo `json:"info"`
}

func (info Info) CurrentProfileIndex() int {
	for i, profile := range info.Info.ProfileList {
		if profile.ID == info.Current {
			return i
		}
	}
	// Default is 'first' profile
	return 0
}

func (profile *Profile) supportsProtocol(protocol string) bool {
	for _, proto := range profile.VPNProtoList {
		if proto == protocol {
			return true
		}
	}
	return false
}

func (profile *Profile) SupportsWireguard() bool {
	return profile.supportsProtocol("wireguard")
}

func (profile *Profile) SupportsOpenVPN() bool {
	return profile.supportsProtocol("openvpn")
}

func (info Info) Supported(wireguardSupport bool) []Profile {
	var valid []Profile
	for _, p := range info.Info.ProfileList {
		// Not a valid profile because it does not support openvpn
		// Also the client does not support wireguard
		if !p.SupportsOpenVPN() && !wireguardSupport {
			continue
		}
		valid = append(valid, p)
	}
	return valid
}

func (info Info) Has(id string) bool {
	for _, p := range info.Info.ProfileList {
		if p.ID == id {
			return true
		}
	}
	return false
}

func (info Info) Public() server.Profiles {
	m := make(map[string]server.Profile)
	for _, p := range info.Info.ProfileList {
		var protocols []protocol.Protocol
		for _, ps := range p.VPNProtoList {
			protocols = append(protocols, protocol.New(ps))
		}
		m[p.ID] = server.Profile{
			DisplayName: map[string]string{
				"en": p.DisplayName,
			},
			Protocols: protocols,
		}
	}
	return server.Profiles{Map: m, Current: info.Current}
}
