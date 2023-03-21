// package discovery defines the public types that have to deal with discovery
package discovery

import (
	"encoding/json"
	"time"
)

// Organizations is the type that defines the upstream discovery format for the list of organizations
// TODO: Discovery here is the same as the upstream discovery format, should we separate this as well?
// Defined in URL: "https://disco.eduvpn.org/v2/organization_list.json"
type Organizations struct {
	// Version is the version field. The Go library internally already checks for rollbacks, you can use this for logging
	Version   uint64         `json:"v"`
	// List is the list/slice of organizations. Omitted if none are there
	List      []Organization `json:"organization_list,omitempty"`
	// Timestamp is a timestamp that is internally used by the Go library to keep track of when the organizations was last updated
	// You can also use this for logging
	Timestamp time.Time      `json:"go_timestamp"`
}

// Organization is the type that defines the upstream discovery format for a single organization
type Organization struct {
	// DisplayName is the map of strings from language tags to display names
	// Omitted if none is defined
	DisplayName        MapOrString `json:"display_name,omitempty"`
	// OrgID is the organization ID for the server
	OrgID              string      `json:"org_id"`
	// SecureInternetHome is the secure internet home server that belongs to this organization
	// Omitted if none is defined
	SecureInternetHome string      `json:"secure_internet_home,omitempty"`
	// KeywordList is the list of keywords
	// Omitted if none is defined
	KeywordList        MapOrString `json:"keyword_list,omitempty"`
}

// Servers is the type that defines the upstream discovery format for the list of servers
// url: "https://disco.eduvpn.org/v2/server_list.json"
type Servers struct {
	// Version is the version field in discovery. The Go library already checks for rollbacks, use this for logging
	Version   uint64    `json:"v"`
	// List is the actual list of servers, omitted from the JSON if empty
	List      []Server  `json:"server_list,omitempty"`
	// Timestamp is a timestamp that is internally used by the Go library to keep track of when the organizations was last updated
	// You can also use this for logging
	Timestamp time.Time `json:"go_timestamp"`
}

// Server is a signle discovery server
type Server struct {
	// AuthenticationURLTemplate is the template to be used for authentication to skip WAYF
	AuthenticationURLTemplate string      `json:"authentication_url_template"`
	// BaseURL is the base URL of the server which is used as an identifier for the server by the Go library
	BaseURL                   string      `json:"base_url"`
	// CountryCode is the country code for the server in case of secure internet, e.g. NL
	CountryCode               string      `json:"country_code"`
	// DisplayName is the display name of the server, omitted if empty
	DisplayName               MapOrString `json:"display_name,omitempty"`
	// DisplayName are the keywords of the server, omitted if empty
	KeywordList               MapOrString `json:"keyword_list,omitempty"`
	// PublicKeyList are the public keys of the server. Currently not used in this lib but returned by the upstream discovery server
	PublicKeyList             []string    `json:"public_key_list"`
	// Type is the type of the server, "secure_internet" or "institute_access"
	Type                      string      `json:"server_type"`
	// SupportContact is the list/slice of support contacts
	SupportContact            []string    `json:"support_contact"`
}

// MapOrString is a custom type as the upstream discovery format is a map or a value.
// This library always marshals the data as a map and then makes sure unmarshalling also gives a map
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
