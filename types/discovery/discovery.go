package discovery

import (
	"encoding/json"
	"time"
)

// TODO: Discovery here is the same as the upstream discovery format, should we separate this as well?
// Defined in URL: "https://disco.eduvpn.org/v2/organization_list.json"
type Organizations struct {
	Version   uint64         `json:"v"`
	List      []Organization `json:"organization_list,omitempty"`
	Timestamp time.Time      `json:"go_timestamp"`
}

type Organization struct {
	DisplayName        MapOrString `json:"display_name,omitempty"`
	OrgID              string      `json:"org_id"`
	SecureInternetHome string      `json:"secure_internet_home,omitempty"`
	KeywordList        MapOrString `json:"keyword_list,omitempty"`
}

// Structs that define the json format for
// url: "https://disco.eduvpn.org/v2/server_list.json"
type Servers struct {
	Version   uint64    `json:"v"`
	List      []Server  `json:"server_list,omitempty"`
	Timestamp time.Time `json:"go_timestamp"`
}

type MapOrString map[string]string

// The display name can either be a map or a string in the server list
// Unmarshal it by first trying a string and then the map.
func (displayName *MapOrString) UnmarshalJSON(data []byte) error {
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

type Server struct {
	AuthenticationURLTemplate string      `json:"authentication_url_template"`
	BaseURL                   string      `json:"base_url"`
	CountryCode               string      `json:"country_code"`
	DisplayName               MapOrString `json:"display_name,omitempty"`
	KeywordList               MapOrString `json:"keyword_list,omitempty"`
	PublicKeyList             []string    `json:"public_key_list"`
	Type                      string      `json:"server_type"`
	SupportContact            []string    `json:"support_contact"`
}
