// package types lists the various public types that are returned to clients
package types

import (
	"github.com/eduvpn/eduvpn-common/types/protocol"
)

type Expiry struct {
	StartTime         int64   `json:"start_time"`
	EndTime           int64   `json:"end_time"`
	ButtonTime        int64   `json:"button_time"`
	CountdownTime     int64   `json:"countdown_time"`
	NotificationTimes []int64 `json:"notification_times"`
}

type Profile struct {
	Identifier  string              `json:"identifier"`
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

type GenericServer struct {
	DisplayName map[string]string `json:"display_name,omitempty"`
	Identifier  string            `json:"identifier"`
	Profiles    Profiles          `json:"profiles"`
}

type InstituteServer struct {
	GenericServer
	Delisted bool `json:"delisted"`
}

type SecureInternetServer struct {
	GenericServer
	CountryCode string `json:"country_code"`
	Delisted    bool   `json:"delisted"`
}

type ServerList struct {
	Institutes     []InstituteServer     `json:"institute_access_servers,omitempty"`
	SecureInternet *SecureInternetServer `json:"secure_internet_server,omitempty"`
	Custom         []GenericServer       `json:"custom_servers,omitempty"`
}

type Configuration struct {
	VPNConfig      string            `json:"config"`
	Protocol       protocol.Protocol `json:"protocol"`
	DefaultGateway bool              `json:"default_gateway"`
	Tokens         Tokens            `json:"tokens"`
}

type ServerType int8

const (
	SERVER_UNKNOWN ServerType = iota

	SERVER_INSTITUTE_ACCESS

	SERVER_SECURE_INTERNET

	SERVER_CUSTOM
)

type CurrentServer struct {
	Institute      *InstituteServer      `json:"institute_access_server,omitempty"`
	SecureInternet *SecureInternetServer `json:"secure_internet_server,omitempty"`
	Custom         *GenericServer        `json:"custom_server,omitempty"`
	Type           ServerType            `json:"server_type"`
}
