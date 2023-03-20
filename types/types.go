// package types lists the various public types that are returned to clients
package types

import (
	"encoding/json"
	"time"
)

// TODO: Discovery here is the same as the upstream discovery format, should we separate this as well?
// Shared server types
// Structs that define the json format for
// url: "https://disco.eduvpn.org/v2/organization_list.json"
type DiscoveryOrganizations struct {
	Version   uint64                  `json:"v"`
	List      []DiscoveryOrganization `json:"organization_list,omitempty"`
	Timestamp time.Time               `json:"go_timestamp"`
}

type DiscoveryOrganization struct {
	DisplayName        DiscoMapOrString `json:"display_name,omitempty"`
	OrgID              string           `json:"org_id"`
	SecureInternetHome string           `json:"secure_internet_home,omitempty"`
	KeywordList        DiscoMapOrString `json:"keyword_list,omitempty"`
}

// Structs that define the json format for
// url: "https://disco.eduvpn.org/v2/server_list.json"
type DiscoveryServers struct {
	Version   uint64            `json:"v"`
	List      []DiscoveryServer `json:"server_list,omitempty"`
	Timestamp time.Time         `json:"go_timestamp"`
}

type DiscoMapOrString map[string]string

// The display name can either be a map or a string in the server list
// Unmarshal it by first trying a string and then the map.
func (displayName *DiscoMapOrString) UnmarshalJSON(data []byte) error {
	var displayNameString string

	err := json.Unmarshal(data, &displayNameString)

	if err == nil {
		*displayName = map[string]string{"en": displayNameString}
		return nil
	}

	var resultingMap map[string]string

	err = json.Unmarshal(data, &resultingMap)

	if err == nil {
		*displayName = resultingMap
		return nil
	}
	return err
}

type DiscoveryServer struct {
	AuthenticationURLTemplate string           `json:"authentication_url_template"`
	BaseURL                   string           `json:"base_url"`
	CountryCode               string           `json:"country_code"`
	DisplayName               DiscoMapOrString `json:"display_name,omitempty"`
	KeywordList               DiscoMapOrString `json:"keyword_list,omitempty"`
	PublicKeyList             []string         `json:"public_key_list"`
	Type                      string           `json:"server_type"`
	SupportContact            []string         `json:"support_contact"`
}

type Expiry struct {
	StartTime int64 `json:"start_time"`
	EndTime int64 `json:"end_time"`
	ButtonTime int64 `json:"button_time"`
	CountdownTime int64 `json:"countdown_time"`
	NotificationTimes []int64 `json:"notification_times"`
}

type Protocol int8

const (
	// PROTOCOL_UNKNOWN indicates that the protocol is not known
	PROTOCOL_UNKNOWN Protocol = iota
	// PROTOCOL_OPENVPN indicates that the protocol is OpenVPN
	PROTOCOL_OPENVPN
	// PROTOCOL_WIREGUARD indicates that the protocol is WireGuard
	PROTOCOL_WIREGUARD
)

type Profile struct {
	Identifier  string            `json:"identifier"`
	DisplayName map[string]string `json:"display_name,omitempty"`
	Protocols   []Protocol        `json:"supported_protocols"`
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
	VPNConfig      string   `json:"config"`
	Protocol       Protocol `json:"protocol"`
	DefaultGateway bool     `json:"default_gateway"`
	Tokens         Tokens   `json:"tokens"`
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
