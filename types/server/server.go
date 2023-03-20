package server

import "github.com/eduvpn/eduvpn-common/types/protocol"

type Type int8

const (
	TypeUnknown Type = iota
	TypeInstituteAccess
	TypeSecureInternet
	TypeCustom
)

type Expiry struct {
	StartTime         int64   `json:"start_time"`
	EndTime           int64   `json:"end_time"`
	ButtonTime        int64   `json:"button_time"`
	CountdownTime     int64   `json:"countdown_time"`
	NotificationTimes []int64 `json:"notification_times"`
}

type Profile struct {
	DisplayName map[string]string   `json:"display_name,omitempty"`
	Protocols   []protocol.Protocol `json:"supported_protocols"`
}

type Profiles struct {
	Map     map[string]Profile `json:"map,omitempty"`
	Current string             `json:"current"`
}

type Tokens struct {
	Access  string `json:"access_token"`
	Refresh string `json:"refresh_token"`
	Expires int64  `json:"expires_in"`
}

type Server struct {
	DisplayName map[string]string `json:"display_name,omitempty"`
	Identifier  string            `json:"identifier"`
	Profiles    Profiles          `json:"profiles"`
}

type Institute struct {
	Server
	Delisted bool `json:"delisted"`
}

type SecureInternet struct {
	Server
	CountryCode string `json:"country_code"`
	Delisted    bool   `json:"delisted"`
}

type List struct {
	Institutes     []Institute     `json:"institute_access_servers,omitempty"`
	SecureInternet *SecureInternet `json:"secure_internet_server,omitempty"`
	Custom         []Server       `json:"custom_servers,omitempty"`
}

type Configuration struct {
	VPNConfig      string            `json:"config"`
	Protocol       protocol.Protocol `json:"protocol"`
	DefaultGateway bool              `json:"default_gateway"`
	Tokens         Tokens            `json:"tokens"`
}

type Current struct {
	Institute      *Institute      `json:"institute_access_server,omitempty"`
	SecureInternet *SecureInternet `json:"secure_internet_server,omitempty"`
	Custom         *Server        `json:"custom_server,omitempty"`
	Type           Type            `json:"server_type"`
}
