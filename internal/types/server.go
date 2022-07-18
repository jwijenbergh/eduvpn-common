package types

import (
	"encoding/json"
)

// Shared server types

// Structs that define the json format for
// url: "https://disco.eduvpn.org/v2/organization_list.json"
type DiscoveryOrganizations struct {
	Version   uint64                  `json:"v"`
	List      []DiscoveryOrganization `json:"organization_list"`
	Timestamp int64                   `json:"-"`
	RawString string                  `json:"-"`
}

type DiscoveryOrganization struct {
	DisplayName        map[string]string `json:"display_name"`
	OrgId              string            `json:"org_id"`
	SecureInternetHome string            `json:"secure_internet_home"`
	KeywordList        struct {
		En string `json:"en"`
	} `json:"keyword_list"`
}

// Structs that define the json format for
// url: "https://disco.eduvpn.org/v2/server_list.json"
type DiscoveryServers struct {
	Version   uint64            `json:"v"`
	List      []DiscoveryServer `json:"server_list"`
	Timestamp int64             `json:"-"`
	RawString string            `json:"-"`
}

type DNMapOrString map[string]string

// The display name can either be a map or a string in the server list
// Unmarshal it by first trying a string and then the map
func (DN *DNMapOrString) UnmarshalJSON(data []byte) error {
	var displayNameString string

	err := json.Unmarshal(data, &displayNameString)

	if err == nil {
		*DN = map[string]string{"en": displayNameString}
		return nil
	}

	var resultingMap map[string]string

	err = json.Unmarshal(data, &resultingMap)

	if err == nil {
		*DN = resultingMap
		return nil
	}
	return err
}

type DiscoveryServer struct {
	AuthenticationURLTemplate string        `json:"authentication_url_template"`
	BaseURL                   string        `json:"base_url"`
	CountryCode               string        `json:"country_code"`
	DisplayName               DNMapOrString `json:"display_name,omitempty"`
	PublicKeyList             []string      `json:"public_key_list"`
	Type                      string        `json:"server_type"`
	SupportContact            []string      `json:"support_contact"`
}
