package discovery

import (
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/eduvpn/eduvpn-common/internal/test"
	discotypes "github.com/eduvpn/eduvpn-common/types/discovery"
)

// TestServers tests whether or not we can obtain discovery servers
// It setups up a file server using the 'test_files' directory
func TestServers(t *testing.T) {
	handler := http.FileServer(http.Dir("test_files"))
	s := test.NewServer(handler)
	DiscoURL = s.URL
	c, err := s.Client()
	if err != nil {
		t.Fatalf("Failed to get HTTP test client: %v", err)
	}
	d := &Discovery{httpClient: c}
	// get servers
	s1, err := d.Servers()
	if err != nil {
		t.Fatalf("Failed getting servers: %v", err)
	}

	// Shutdown the server
	s.Close()
	// Test if we get the same cached copy
	s2, err := d.Servers()
	// We should not get an error as the timestamp is not expired
	if err != nil {
		t.Fatalf("Got a servers error after shutting down server: %v", err)
	}
	if s1 != s2 {
		t.Fatalf("Servers copies not equal after shutting down file server")
	}

	// Force expired, 1 hour in the past
	d.ServerList.Timestamp = time.Now().Add(-1 * time.Hour)

	s3, err := d.Servers()
	// Now we expect an error with the cached copy
	if err == nil {
		t.Fatalf("Got a servers nil error after shutting down file server and expired")
	}
	if s1 != s3 {
		t.Fatalf("Servers copies not equal after shutting down file server and expired")
	}
}

// TestOrganizations tests whether or not we can obtain discovery organizations
// It setups up a file server using the 'test_files' directory
func TestOrganizations(t *testing.T) {
	handler := http.FileServer(http.Dir("test_files"))
	s := test.NewServer(handler)
	DiscoURL = s.URL
	c, err := s.Client()
	if err != nil {
		t.Fatalf("Failed to get HTTP test client: %v", err)
	}
	d := &Discovery{httpClient: c}
	// get servers
	s1, err := d.Organizations()
	if err != nil {
		t.Fatalf("Failed getting organizations: %v", err)
	}

	// Shutdown the server
	s.Close()
	// Test if we get the same cached copy
	// We should not get an error as the timestamp is not zero
	s2, err := d.Organizations()
	if err != nil {
		t.Fatalf("Got an organizations error after shutting down file server: %v", err)
	}
	if s1 != s2 {
		t.Fatalf("Organizations copies not equal after shutting down file server")
	}
}

// TestSecureLocationList tests the function for getting a list of secure internet servers
func TestSecureLocationList(t *testing.T) {
	d := Discovery{
		ServerList: discotypes.Servers{
			Version: 1,
			List: []discotypes.Server{
				// institute access server, this should not be found
				{CountryCode: "", Type: "institute_access"},
				// secure internet servers, these should be found
				{CountryCode: "b", Type: "secure_internet"},
				{CountryCode: "c", Type: "secure_internet"},
				// Unexpected type, this should not be found
				{CountryCode: "d", Type: "test"},
			},
		},
	}

	cc := d.SecureLocationList()
	want := []string{"b", "c"}

	if !reflect.DeepEqual(cc, want) {
		t.Fatalf("Secure location list is not equal. Got: %v, Want: %v", cc, want)
	}
}

// TestServerByURL tests the function for getting a server by the Base URL and type
func TestServerByURL(t *testing.T) {
	d := Discovery{
		ServerList: discotypes.Servers{
			Version: 1,
			List: []discotypes.Server{
				// institute access server
				{BaseURL: "a", Type: "institute_access"},
				// secure internet servers
				{BaseURL: "b", Type: "secure_internet"},
				// Unexpected type, this should not be found
				{BaseURL: "d", Type: "test"},
			},
		},
	}
	// Institute Access: Can be found
	_, err := d.ServerByURL("a", "institute_access")
	if err != nil {
		t.Fatalf("Got error: %v, when getting a server by url with parameters 'a' and 'institute_access'", err)
	}

	// Institute Access: Cannot be found
	_, err = d.ServerByURL("b", "institute_access")
	if err == nil {
		t.Fatal("Got no error, when getting a non-existing server by url with parameters 'b' and 'institute_access'")
	}

	// Secure Internet: Can be found
	_, err = d.ServerByURL("b", "secure_internet")
	if err != nil {
		t.Fatalf("Got error: %v, when getting a server by url with parameters 'b' and 'secure_internet'", err)
	}

	// Secure Internet: Cannot be found because of invalid type
	_, err = d.ServerByURL("d", "secure_internet")
	if err == nil {
		t.Fatal("Got no error, when getting a non-existing server by url with parameters 'd' and 'secure_internet'")
	}
}

// TestServerByCountryCode tests the function for getting a server by the country code
func TestServerByCountryCode(t *testing.T) {
	s1 := discotypes.Server{CountryCode: "a", Type: "secure_internet"}
	d := Discovery{
		ServerList: discotypes.Servers{
			Version: 1,
			List: []discotypes.Server{
				// secure internet server
				s1,
				// Unexpected types, these should not be found
				{CountryCode: "b", Type: "institute_access"},
				{CountryCode: "c", Type: "test"},
			},
		},
	}
	// Institute Access: Can be found
	s, err := d.ServerByCountryCode("a")
	if err != nil {
		t.Fatalf("Got error: %v, when getting a server by country code 'a'", err)
	}
	if s.CountryCode != s1.CountryCode || s.Type != s1.Type {
		t.Fatalf("Server with country code 'a' not equal, Got: %v, Want: %v", s, s1)
	}

	// Others: Cannot be found
	_, err = d.ServerByCountryCode("b")
	if err == nil {
		t.Fatal("Got no error when getting a server by country code 'b'")
	}
	_, err = d.ServerByCountryCode("c")
	if err == nil {
		t.Fatal("Got no error when getting a server by country code 'c'")
	}
}

// TestOrgByID tests the function for getting an organization by ID
func TestOrgByID(t *testing.T) {
	o1 := discotypes.Organization{OrgID: "a"}
	d := Discovery{
		OrganizationList: discotypes.Organizations{
			Version: 1,
			List: []discotypes.Organization{
				o1,
			},
		},
	}
	o, err := d.orgByID("a")
	if err != nil {
		t.Fatal("Got an error when getting an organization with ID: 'a'")
	}
	if o.OrgID != o1.OrgID {
		t.Fatalf("Organizations not equal, Got: %v, Want: %v", o, o1)
	}
	_, err = d.orgByID("b")
	if err == nil {
		t.Fatal("Got no error when searching for non-existing organization 'b'")
	}
}

// TestSecureHomeArgs tests the function for getting an organization and matching secure internet server by organization ID
func TestSecureHomeArgs(t *testing.T) {
	o1 := discotypes.Organization{OrgID: "id", SecureInternetHome: "a"}
	s1 := discotypes.Server{BaseURL: "a", Type: "secure_internet"}
	d := Discovery{
		OrganizationList: discotypes.Organizations{
			Version: 1,
			List: []discotypes.Organization{
				{OrgID: "id2", SecureInternetHome: "c"},
				o1,
			},
		},
		ServerList: discotypes.Servers{
			Version: 1,
			List: []discotypes.Server{
				s1,
				{BaseURL: "b"},
			},
		},
	}

	// Args found
	o, s, err := d.SecureHomeArgs("id")
	if err != nil {
		t.Fatalf("Got error: %v, when getting secure home arguments with ID: 'id'", err)
	}
	if o.OrgID != o1.OrgID || o.SecureInternetHome != o1.SecureInternetHome {
		t.Fatalf("Organizations not equal for secure home arguments, Got: %v, Want: %v", o, o1)
	}
	if s.BaseURL != s1.BaseURL {
		t.Fatalf("Servers not equal for secure home arguments, Got: %v, Want: %v", s, s1)
	}
	// Args not found because no matching secure internet server
	_, _, err = d.SecureHomeArgs("id2")
	if err == nil {
		t.Fatal("Got no error, when getting non-matching secure home arguments with ID: 'id2'")
	}

	// Args not found because no organization
	_, _, err = d.SecureHomeArgs("id3")
	if err == nil {
		t.Fatal("Got no error, when getting non-existing secure home arguments with ID: 'id3'")
	}
}
