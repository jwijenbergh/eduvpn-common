// package main implements the main exported API to be used by other languages
//
// Some notes:
//
// - Errors are returned as JSON c strings. The JSON type is defined in types/error/error.go Error. Free them using FreeString. Same is the case for other string types, you should also free them. The errors are always localized
//
// - Types are converted from the Go representation to C using JSON strings
//
// - Cookies are used for cancellation, just fancy contexts. Create a cookie using `CookieNew`, pass it to the function that needs one as the first argument. To cancel the function, call `CookieCancel`, passing in the same cookie as argument
//
// - Cookies must also be freed, by using the CookieDelete function if the cookie is no longer needed
//
// - The state machine is used to track the state of a client. It is mainly used for asking for certain data from the client, e.g. asking for profiles and locations. But a client may also wish to build upon this state machine to build the whole UI around it. The SetState and InState functions are useful for this
package main

/*
#include <stdint.h>
#include <stdlib.h>

typedef long long int (*ReadRxBytes)();

typedef int (*StateCB)(int oldstate, int newstate, void* data);

typedef void (*TokenGetter)(const char* server_id, int server_type, char* out, size_t len);
typedef void (*TokenSetter)(const char* server_id, int server_type, const char* tokens);
typedef void (*ProxyFD)(int fd);

static long long int get_read_rx_bytes(ReadRxBytes read)
{
    return read();
}
static int call_callback(StateCB callback, int oldstate, int newstate, void* data)
{
    return callback(oldstate, newstate, data);
}
static void call_token_getter(TokenGetter getter, const char* server_id, int server_type, char* out, size_t len)
{
    getter(server_id, server_type, out, len);
}
static void call_token_setter(TokenSetter setter, const char* server_id, int server_type, const char* tokens)
{
    setter(server_id, server_type, tokens);
}
static void call_proxy_fd(ProxyFD proxyfd, int fd)
{
    proxyfd(fd);
}
*/
import "C"

import (
	"bytes"
	"context"
	"encoding/json"
	"runtime/cgo"
	"unsafe"

	"github.com/eduvpn/eduvpn-common/client"
	"github.com/eduvpn/eduvpn-common/i18nerr"
	"github.com/eduvpn/eduvpn-common/internal/log"
	"github.com/eduvpn/eduvpn-common/types/cookie"
	errtypes "github.com/eduvpn/eduvpn-common/types/error"
	srvtypes "github.com/eduvpn/eduvpn-common/types/server"
)

// VPNState is the current state of the library
var VPNState *client.Client

func getCError(err error) *C.char {
	if err == nil {
		return nil
	}
	retErr := errtypes.Error{
		Message: errtypes.Translated{
			"en": err.Error(),
		},
		Misc: false,
	}
	v, ok := err.(*i18nerr.Error)
	if ok {
		retErr.Message = v.Translations()
		retErr.Misc = v.Misc
	}
	retData, err := getReturnData(retErr)
	if err != nil {
		return C.CString("failed to get error return")
	}
	return C.CString(retData)
}

func getReturnData(data interface{}) (string, error) {
	// If it is already a string return directly
	if x, ok := data.(string); ok {
		return x, nil
	}

	// Otherwise use JSON
	b, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func stateCallback(
	cb C.StateCB,
	oldState client.FSMStateID,
	newState client.FSMStateID,
	data interface{},
) bool {
	oldStateC := C.int(oldState)
	newStateC := C.int(newState)
	d, err := getReturnData(data)
	if err != nil {
		return false
	}
	dataC := C.CString(d)
	handled := C.call_callback(cb, oldStateC, newStateC, unsafe.Pointer(dataC))
	FreeString(dataC)
	return handled != C.int(0)
}

func getVPNState() (*client.Client, error) {
	if VPNState == nil {
		return nil, i18nerr.NewInternal("No state available, did you register the client?")
	}
	return VPNState, nil
}

// Register creates a new client and also registers the FSM to go to the initial state
//
// `Name` is the name of the client, must be a valid client ID
//
// `Version` is the version of the client. This version field is used for the user agent in all HTTP requests
//
// `cb` is the state callback. It takes three arguments: The old state, the new state and the data for the state as JSON
//
//   - Note that the states are defined in client/fsm.go, e.g. NO_SERVER (in Go: StateNoServer), ASK_PROFILE (in Go: StateAskProfile)
//
//   - This callback returns non-zero if the state transition is handled. This is used to check if the client handles the needed transitions
//
// debug, if non-zero, enables debugging mode for the library, this means:
//
//   - Log everything in debug mode, so you can get more detail of what is going on
//
//   - Write the state graph to a file in the configDirectory. This can be used to create a FSM png file with mermaid https://mermaid.js.org/
//
// After registering, the FSM is initialized and the state transition NO_SERVER should have been completed
// If some error occurs during registering, it is returned as a types/error/error.go Error
//
// Example Input:
// ```Register("org.eduvpn.app.linux", "0.0.1", "/tmp/eduvpn-common", myCallbackFunc, 1)```
//
// Example Output:
//
//	{
//	  "message": {
//	    "en": "failed to register, a VPN state is already present"
//	  },
//	  "misc": false
//	}
//
//export Register
func Register(
	name *C.char,
	version *C.char,
	configDirectory *C.char,
	cb C.StateCB,
	debug C.int,
) *C.char {
	_, stateErr := getVPNState()
	if stateErr == nil {
		return getCError(i18nerr.NewInternal("failed to register, a VPN state is already present"))
	}
	c, err := client.New(
		C.GoString(name),
		C.GoString(version),
		C.GoString(configDirectory),
		func(old client.FSMStateID, new client.FSMStateID, data interface{}) bool {
			return stateCallback(cb, old, new, data)
		},
		debug != 0,
	)
	// Only update the state if we get no error
	if err == nil {
		// Update the global client such that other functions can retrieve it
		// TODO: Use a sync.Once or return a CGO handler instead of a global state?
		VPNState = c
		// finally register the newly created client
		err = c.Register()
		if err != nil {
			// Note: Registering can only fail for non-newly created clients
			// We have obtained a fresh copy here
			// This error is only there for the Go API where you can call register multiple times on an already client
			panic(err)
		}
	}

	return getCError(err)
}

// ExpiryTimes gets the expiry times for the current server
//
// Expiry times are just fields that represent unix timestamps at which to do certain events regarding expiry,
// e.g. when to show the renew button, when to show expiry notifications
//
// The expiry times structure is defined in types/server/server.go `Expiry`
// If some error occurs, it is returned as types/error/error.go Error
//
// Example Input:
// ```ExpiryTimes()```
//
// Example Output (1...4 are unix timestamps):
//
//	{
//	     "start_time": 1,
//	     "end_time": 2,
//	     "button_time": 3,
//	     "countdown_time": 4,
//	     "notification_times": [
//	         1,
//	         2,
//	     ],
//	}, null
//
//export ExpiryTimes
func ExpiryTimes() (*C.char, *C.char) {
	state, stateErr := getVPNState()
	if stateErr != nil {
		return nil, getCError(stateErr)
	}
	exp, err := state.ExpiryTimes()
	if err != nil {
		return nil, getCError(err)
	}
	ret, err := getReturnData(exp)
	if err != nil {
		return nil, getCError(err)
	}
	return C.CString(ret), nil
}

// Deregister cleans up the state for the client.
//
// This function SHOULD be called when the application exits such that the configuration file is saved correctly.
// Note that saving of the configuration file also happens in other cases, such as after getting a VPN configuration.
// Thus it is often not problematic if this function cannot be called due to a client crash
//
// If no client is available or deregistering fails, it returns an error.
//
// Example Input:
// ```Deregister()```
//
// Example Output:
//
//	{
//	  "message": {
//	    "en": "failed to deregister"
//	  },
//	  "misc": false
//	}
//
//export Deregister
func Deregister() *C.char {
	state, stateErr := getVPNState()
	if stateErr != nil {
		return getCError(stateErr)
	}
	state.Deregister()
	VPNState = nil
	return nil
}

// AddServer adds a server to the eduvpn-common server list
// `c` is the cookie that is used for cancellation. Create a cookie first with CookieNew. This same cookie is also used for replying to state transitions
//
// `_type` is the type of server that needs to be added. This type is defined in types/server/server.go Type
//
// `id` is the identifier of the string:
//
//   - In case of secure internet: The organization ID
//
//   - In case of custom server: The base URL
//
//   - In case of institute access: The base URL
//
// `ni` stands for non-interactive. If non-zero, any state transitions will not be run.
//
// This `ni` flag is useful for preprovisioned servers. For normal usage, you want to set this to zero (meaning: False)
//
// If the server cannot be added it returns the error as types/error/error.go Error.
// Note that the server is removed when an error has occured
//
// The following state callbacks are mandatory to handle:
//
//   - OAUTH_STARTED: This indicates that the OAuth procedure has been started, it returns the URL as the data.
//     The client should open the webbrowser with this URL and continue the authorization process. Note: For mobile platforms this returns a Cookie and data (json: `{"cookie": x, "data": url}`).
//     This `url` should also be opened in the browser like desktop platforms. But these platforms also need to reply to the library to give back the full authorization code URI with `CookieReply(x, uri)`.
//     E.g. `CookieReply(x, "/callback?code=...&state=...&iss=...")` this is the path of the request that the apps get back when the user clicks approve. For this, apps need to register an app url or sorts. For the valid values for app URLs, see the redirect URIs for mobile platforms here https://git.sr.ht/~fkooman/vpn-user-portal/tree/v3/item/src/OAuth/VpnClientDb.php
//
// Example Input (3=custom server):
// ```AddServer(mycookie, 3, "https://demo.eduvpn.nl", 0)```
//
// Example Output:
//
//	{
//	  "message": {
//	    "en": "failed to add server"
//	  },
//	  "misc": false
//	}
//
//export AddServer
func AddServer(c C.uintptr_t, _type C.int, id *C.char, ni C.int) *C.char {
	// TODO: type
	state, stateErr := getVPNState()
	if stateErr != nil {
		return getCError(stateErr)
	}
	v, err := getCookie(c)
	if err != nil {
		return getCError(err)
	}
	err = state.AddServer(v, C.GoString(id), srvtypes.Type(_type), ni != 0)
	return getCError(err)
}

// RemoveServer removes a server from the eduvpn-common server list
//
// `_type` is the type of server that needs to be added. This type is defined in types/server/server.go Type
//
// `id` is the identifier of the string:
//
//   - In case of secure internet: The organization ID
//
//   - In case of custom server: The base URL
//
//   - In case of institute access: The base URL
//
// If the server cannot be removed it returns the error types/error/error.go Error.
//
// Example Input (3=custom server):
// ```RemoveServer(3, "bogus")```
//
// Example Output:
//
//	{
//	  "message": {
//	    "en": "failed to remove server"
//	  },
//	  "misc": false
//	}
//
//export RemoveServer
func RemoveServer(_type C.int, id *C.char) *C.char {
	state, stateErr := getVPNState()
	if stateErr != nil {
		return getCError(stateErr)
	}
	err := state.RemoveServer(C.GoString(id), srvtypes.Type(_type))
	return getCError(err)
}

// CurrentServer gets the current server from eduvpn-common
//
// In eduvpn-common, a server is marked as 'current' if you have gotten a VPN configuration for it
//
// It returns the server as JSON, defined in types/server/server.go Current
//
// If there is no current server or some other, e.g. there is no current state, an error is returned with a nil string
//
// Example Input:
// ```CurrentServer()```
//
// Example Output:
//
//	{
//	  "institute_access_server": {
//	    "display_name": {
//	      "en": "Demo"
//	    },
//	    "identifier": "https://demo.eduvpn.nl/",
//	    "profiles": {
//	      "map": {
//	        "internet": {
//	          "display_name": {
//	            "en": "Internet"
//	          },
//	          "supported_protocols": [
//	            1,
//	            2
//	          ]
//	        },
//	        "internet-split": {
//	          "display_name": {
//	            "en": "No rfc1918 routes"
//	          },
//	          "supported_protocols": [
//	            1,
//	            2
//	          ]
//	        }
//	      },
//	      "current": "internet"
//	    },
//	    "support_contacts": [
//	      "mailto:eduvpn@surf.nl"
//	    ],
//	    "delisted": false
//	  },
//	  "server_type": 1
//	}, null
//
//export CurrentServer
func CurrentServer() (*C.char, *C.char) {
	state, stateErr := getVPNState()
	if stateErr != nil {
		return nil, getCError(stateErr)
	}
	srv, err := state.CurrentServer()
	if err != nil {
		return nil, getCError(err)
	}
	ret, err := getReturnData(srv)
	if err != nil {
		return nil, getCError(err)
	}
	return C.CString(ret), nil
}

// ServerList gets the list of servers that are currently added
//
// This is NOT the discovery list, but the servers that have previously been added with `AddServer`
//
// It returns the server list as a JSON string defined in types/server/server.go List.
// If the server list cannot be retrieved it returns a nil string and an error
//
// Example Input:
// ```ServerList()```
//
// Example Output (current profile here is empty as none has been chosen yet):
//
//	{
//	  "institute_access_servers": [
//	    {
//	      "display_name": {
//	        "en": "Demo"
//	      },
//	      "identifier": "https://demo.eduvpn.nl/",
//	      "profiles": {
//	        "current": ""
//	      },
//	      "support_contacts": [
//	        "mailto:eduvpn@surf.nl"
//	      ],
//	      "delisted": false
//	    }
//	  ]
//	}, null
//
//export ServerList
func ServerList() (*C.char, *C.char) {
	state, stateErr := getVPNState()
	if stateErr != nil {
		return nil, getCError(stateErr)
	}
	list, err := state.ServerList()
	if err != nil {
		return nil, getCError(err)
	}
	ret, err := getReturnData(list)
	if err != nil {
		return nil, getCError(err)
	}
	return C.CString(ret), nil
}

// GetConfig gets a configuration for the server. It returns additional information in case WireGuard over Proxyguard is used (see the last example)
//
// `c` is the cookie that is used for cancellation. Create a cookie first with CookieNew, this same cookie is also used for replying to state transitions
//
// `_type` is the type of server that needs to be added. This type is defined in types/server/server.go Type
//
// `id` is the identifier of the string
//
//   - In case of secure internet: The organization ID
//   - In case of custom server: The base URL
//   - In case of institute access: The base URL
//
// `pTCP` is if we prefer TCP or not to get the configuration, non-zero means yes
//
// `startup` is if the client is just starting up, set this to true (non-zero) if you autoconnect to a server on startup.
// If this startup value is true (non-zero) then any authorization or other callacks (profile/location) are not triggered
//
// After getting a configuration, the FSM moves to the GOT_CONFIG state
// The return data is the configuration, marshalled as JSON and defined in types/server/server.go Configuration
//
// If the config cannot be retrieved it returns an error as types/error/error.go Error.
//
// The current state callbacks MUST be handled:
//
// ### ASK_PROFILE
//
// This asks the client for profile.
//
// This is called when the user/client has not set a profile for this server before, or the current profile is invalid
//
// When the user has selected a profile, reply with the choice using the `CookieReply` function and the profile ID
// e.g. CookieReply(cookie, "wireguard"). CookieReply can be done in the background as the Go library waits for a reply
//
// The data for this transition is defined in types/server/server.go RequiredAskTransition with embedded data Profiles in types/server/server.go.
// Note that RequiredTransition contains the cookie to be used for the CookieReply
//
// So a client would:
//
// - Parse the data to get the cookie and data
//
// - get the cookie
//
// - get the profiles from the data
//
// - show it in the UI and then reply with CookieReply using the choice
//
// ### ASK_LOCATION
//
// This asks the client for a location. Note that under normal circumstances,
// this callback is not actually called as the home organization for the secure internet server is set as the current
// if for some reason, an invalid location has been configured, the library will ask the client for a new one
//
// When the user has selected a location, reply with the choice using the `CookieReply` function and the location ID
// e.g. CookieReply(cookie, "nl")
//
// CookieReply can be done in the background as the Go library waits for a reply
// The data for this transition is defined in types/server/server.go RequiredAskTransition with embedded data a list of strings ([]string)
//
// Note that RequiredTransition contains the cookie to be used for the CookieReply,
//
// So a client would:
//
//   - Parse the data to get the cookie and data
//
//   - get the cookie
//
//   - get the list of locations from the data
//
//   - show it in the UI and then reply with CookieReply using the choice
//
// ### OAUTH_STARTED
//
//   - OAUTH_STARTED: This indicates that the OAuth procedure has been started, it returns the URL as the data.
//     The client should open the webbrowser with this URL and continue the authorization process. Note: For mobile platforms this returns a Cookie and data (json: `{"cookie": x, "data": url}`).
//     This `url` should also be opened in the browser like desktop platforms. But these platforms also need to reply to the library to give back the full authorization code URI with `CookieReply(x, uri)`.
//     E.g. `CookieReply(x, "/callback?code=...&state=...&iss=...")` this is the path of the request that the apps get back when the user clicks approve. For this, apps need to register an app url or sorts. For the valid values for app URLs, see the redirect URIs for mobile platforms here https://git.sr.ht/~fkooman/vpn-user-portal/tree/v3/item/src/OAuth/VpnClientDb.php
//
// The client should open the webbrowser with this URL and continue the authorization process.
// This is only called if authorization needs to be retriggered
//
// Example Input (3=custom server):
// ```GetConfig(myCookie, 3, "https://demo.eduvpn.nl/", 0, 0)```
//
// Example Output (2=WireGuard):
//
//	{
//	 "config": "[Interface]\nPrivateKey = ...\nAddress = ...\nDNS = ...\n\n[Peer]\nPublicKey = ...=\nAllowedIPs = 0.0.0.0/0,::/0\nEndpoint = ...",
//	 "protocol": 2,
//	 "default_gateway": true,
//	 "should_failover": true, <- whether or not the failover procedure should happen
//	}
//
// Example Output (3=WireGuard + Proxyguard):
//
//	{
//	"config":"[Interface]\nMTU = ...\nAddress = ...\nDNS = ...\nPrivateKey = ...\n[Peer]\nPublicKey = ...\nAllowedIPs = ...\nEndpoint = 127.0.0.1:x\n",
//	"protocol":3,
//	"default_gateway":true,
//	"should_failover":true,
//	"proxy":{"source_port":38683,"listen":"127.0.0.1:59812","peer":"https://..."}
//	}
//
//export GetConfig
func GetConfig(c C.uintptr_t, _type C.int, id *C.char, pTCP C.int, startup C.int) (*C.char, *C.char) {
	state, stateErr := getVPNState()
	if stateErr != nil {
		return nil, getCError(stateErr)
	}
	ck, err := getCookie(c)
	if err != nil {
		return nil, getCError(err)
	}
	preferTCPBool := pTCP != 0
	startupBool := startup != 0
	cfg, err := state.GetConfig(ck, C.GoString(id), srvtypes.Type(_type), preferTCPBool, startupBool)
	if cfg != nil && err == nil {
		d, err := getReturnData(cfg)
		if err == nil {
			return C.CString(d), nil
		}
	}
	return nil, getCError(err)
}

// SetProfileID sets the profile ID of the current serrver
//
// This MUST only be called if the user/client wishes to manually set a profile instead of the common lib asking for one using a transition
//
// `Data` is the profile ID
//
// It returns an error if unsuccessful.
// Example Input: ```SetProfileID("splittunnel")```
//
// Example Output:
//
//	{
//	  "message": {
//	    "en": "profile does not exist"
//	  },
//	  "misc": false
//	}
//
//export SetProfileID
func SetProfileID(data *C.char) *C.char {
	state, stateErr := getVPNState()
	if stateErr != nil {
		return getCError(stateErr)
	}
	profileErr := state.SetProfileID(C.GoString(data))
	return getCError(profileErr)
}

// SetSecureLocation sets the location for the secure internet server if it exists
//
// This MUST only be called if the user/client wishes to manually set a location instead of the common lib asking for one using a transition
//
// `orgID` is the organisation ID for the secure internet server
// `cc` is the location ID/country code
//
// It returns an error if unsuccessful.
// Example Input: ```SetSecureLocation("http://idp.geant.org/", "nl")```
//
// Example Output:
//
//	{
//	  "message": {
//	    "en": "location does not exist"
//	  },
//	  "misc": false
//	}
//
//export SetSecureLocation
func SetSecureLocation(orgID *C.char, cc *C.char) *C.char {
	state, stateErr := getVPNState()
	if stateErr != nil {
		return getCError(stateErr)
	}
	locationErr := state.SetSecureLocation(C.GoString(orgID), C.GoString(cc))
	return getCError(locationErr)
}

// DiscoServers gets the servers from discovery, returned as types/discovery/discovery.go Servers marshalled as JSON
//
// `c` is the Cookie that needs to be passed. Create a new Cookie using `CookieNew`
//
// If it was unsuccessful, it returns an error. Note that when the lib was built in release mode the data is almost always non-nil, even when an error has occurred
// This means it has just returned the cached list
//
// Example Input: ```DiscoServers(myCookie)```
//
// Example Output:
//
//	{
//	 "v": 1695291170,
//	 "server_list": [
//	   {
//	     "base_url": "https://eduvpn.rash.al/",
//	     "country_code": "AL",
//	     "public_key_list": [
//	       "k7.pub.S4j5JJiTEz1fWMkI.hzU_xJasWzD6Da2WR7hgbobx9n3o4XSDeqFh03tgM-0"
//	     ],
//	     "server_type": "secure_internet",
//	     "support_contact": [
//	       "mailto:helpdesk@rash.al"
//	     ]
//	   },
//	   {
//	     "base_url": "https://eduvpn.deic.dk/",
//	     "country_code": "DK",
//	     "public_key_list": [
//	       "k7.pub.RNOJIYbemlfsE7EL.BxmV2l2UV7pCqz135ofBgyG9-xLg0R9rILQedZrfLtE"
//	     ], ..................
//	} , null
//
//export DiscoServers
func DiscoServers(c C.uintptr_t) (*C.char, *C.char) {
	state, stateErr := getVPNState()
	if stateErr != nil {
		return nil, getCError(stateErr)
	}
	ck, err := getCookie(c)
	if err != nil {
		return nil, getCError(err)
	}
	servers, err := state.DiscoServers(ck)
	if servers == nil && err != nil {
		return nil, getCError(err)
	}
	s, reterr := getReturnData(servers)
	if reterr != nil {
		return nil, getCError(reterr)
	}
	return C.CString(s), getCError(err)
}

// DiscoOrganizations gets the organizations from discovery, returned as types/discovery/discovery.go Organizations marshalled as JSON
//
// `c` is the Cookie that needs to be passed. Create a new Cookie using `CookieNew`
//
// If it was unsuccessful, it returns an error. Note that when the lib was built in release mode the data is almost always non-nil, even when an error has occurred
// This means it has just returned the cached list
//
// Example Input: ```DiscoOrganizations(myCookie)```
//
// Example Output:
//
//	{
//	 "v": 1695291170,
//	 "organization_list": [
//	   {
//	     "display_name": {
//	       "en": "Academic Network of Albania - RASH"
//	     },
//	     "org_id": "https://idp.rash.al/simplesaml/saml2/idp/metadata.php",
//	     "secure_internet_home": "https://eduvpn.rash.al/"
//	   },
//	   {
//	     "display_name": {
//	       "da": "Dansk Sprognævn",
//	       "en": "Danish Language Council"
//	     },
//	     "org_id": "http://idp.dsn.dk/adfs/services/trust",
//	     "secure_internet_home": "https://eduvpn.deic.dk/"
//	   },
//	   {
//	     "display_name": {
//	       "da": "Erhvervsakademi Aarhus",
//	       "en": "Business Academy Aarhus"
//	     },
//	     "org_id": "http://adfs.eaaa.dk/adfs/services/trust",
//	     "secure_inte .....................
//	}, null
//
//export DiscoOrganizations
func DiscoOrganizations(c C.uintptr_t) (*C.char, *C.char) {
	state, stateErr := getVPNState()
	if stateErr != nil {
		return nil, getCError(stateErr)
	}
	ck, err := getCookie(c)
	if err != nil {
		return nil, getCError(err)
	}
	orgs, err := state.DiscoOrganizations(ck)
	if orgs == nil && err != nil {
		return nil, getCError(err)
	}
	s, reterr := getReturnData(orgs)
	if reterr != nil {
		return nil, getCError(reterr)
	}
	return C.CString(s), getCError(err)
}

// Cleanup sends a /disconnect to cleanup the connection
//
// This MUST be called when disconnecting, see https://docs.eduvpn.org/server/v3/api.html#application-flow
// `c` is the Cookie that needs to be passed. Create a new Cookie using `CookieNew`
//
// If it was unsuccessful, it returns an error.
//
// Example Input: ```Cleanup(myCookie)```
//
// Example Output:
//
//	{
//	  "message": {
//	    "en": "cleanup was not successful"
//	  },
//	  "misc": false
//	}
//
//export Cleanup
func Cleanup(c C.uintptr_t) *C.char {
	state, stateErr := getVPNState()
	if stateErr != nil {
		return getCError(stateErr)
	}
	ck, err := getCookie(c)
	if err != nil {
		return getCError(err)
	}
	err = state.Cleanup(ck)
	return getCError(err)
}

// RenewSession renews the session of the VPN
//
// This essentially means that the OAuth tokens are deleted.
// And it also possibly re-runs every state callback you need when getting a config.
// So least you MUST handle the OAuth started transition
//
// It returns an error if unsuccessful.
// Example Input: ```RenewSession(myCookie)```
//
// Example Output:
//
//	{
//	  "message": {
//	    "en": "could not renew session"
//	  },
//	  "misc": false
//	}
//
//export RenewSession
func RenewSession(c C.uintptr_t) *C.char {
	state, stateErr := getVPNState()
	if stateErr != nil {
		return getCError(stateErr)
	}
	ck, err := getCookie(c)
	if err != nil {
		return getCError(err)
	}
	renewSessionErr := state.RenewSession(ck)
	return getCError(renewSessionErr)
}

// SetSupportWireguard enables or disables WireGuard for the client.
// *WARNING: This function will be removed*
//
// By default WireGuard support is enabled
// To disable it you can pass a 0 int to this
//
// `support` thus indicates whether or not to enable WireGuard
// An error is returned if this is not possible
//
//export SetSupportWireguard
func SetSupportWireguard(support C.int) *C.char {
	state, stateErr := getVPNState()
	if stateErr != nil {
		return getCError(stateErr)
	}
	// TODO: Do not do any nested struct member here
	state.Servers.WGSupport = support != 0
	return nil
}

// StartFailover starts the 'failover' procedure in eduvpn-common
//
// Failover has one primary goal: check if the VPN can reach the gateway.
// This can be used to check whether or not the client needs to 'failover' to prefer TCP (if currently using UDP).
// Which is useful to go from a broken WireGuard connection to OpenVPN over TCP
//
//   - `c` is the cookie that is passed for cancellation. To create a cookie, use the `CookieNew` function
//   - `gateway` is the gateway IP of the VPN
//   - `readRxBytes` is a function that returns the current rx bytes of the VPN interface, this should return a `long long int` in c
//
// It returns a boolean whether or not the common lib has determined that it cannot reach the gateway. Non-zero=dropped, zero=not dropped.
// It also returns an error, if it fails to indicate if it has dropped or not. In this case, dropped is also set to zero
//
// Example Input: ```StartFailover(myCookie, "10.10.10.1", 1400, myRxBytesReader)```
//
// Example Output: ```1, null```
//
//export StartFailover
func StartFailover(c C.uintptr_t, gateway *C.char, mtu C.int, readRxBytes C.ReadRxBytes) (C.int, *C.char) {
	state, stateErr := getVPNState()
	if stateErr != nil {
		return C.int(0), getCError(stateErr)
	}
	ck, err := getCookie(c)
	if err != nil {
		return C.int(0), getCError(err)
	}
	dropped, droppedErr := state.StartFailover(ck, C.GoString(gateway), int(mtu), func() (int64, error) {
		rxBytes := int64(C.get_read_rx_bytes(readRxBytes))
		if rxBytes < 0 {
			return 0, i18nerr.NewInternal("client gave an invalid rx bytes value")
		}
		return rxBytes, nil
	})
	if droppedErr != nil {
		return C.int(0), getCError(droppedErr)
	}
	droppedC := C.int(0)
	if dropped {
		droppedC = C.int(1)
	}
	return droppedC, nil
}

// StartProxyguard starts the 'proxyguard' procedure in eduvpn-common.
// This proxies WireGuard UDP connections over HTTP: https://codeberg.org/eduvpn/proxyguard.
// These input variables can be gotten from the configuration that is retrieved using the `proxy` JSON key
//
//   - `c` is the cookie
//   - `listen` is the ip:port of the local udp connection, this is what is set to the WireGuard endpoint
//   - `tcpsp` is the TCP source port
//   - `peer` is the ip:port of the remote server
//   - `proxyFD` is a callback with the file descriptor as only argument. It can be used to set certain
//     socket option, e.g. to exclude the proxy connection from going over the VPN
//
// If the proxy cannot be started it returns an error
//
//export StartProxyguard
func StartProxyguard(c C.uintptr_t, listen *C.char, tcpsp C.int, peer *C.char, proxyFD C.ProxyFD) *C.char {
	state, stateErr := getVPNState()
	if stateErr != nil {
		return getCError(stateErr)
	}
	ck, err := getCookie(c)
	if err != nil {
		return getCError(err)
	}

	proxyErr := state.StartProxyguard(ck, C.GoString(listen), int(tcpsp), C.GoString(peer), func(fd int) {
		if proxyFD == nil {
			return
		}
		C.call_proxy_fd(proxyFD, C.int(fd))
	})
	return getCError(proxyErr)
}

// SetState sets the state of the statemachine
//
// Note: this transitions the FSM into the new state without passing any data to it.
// Example Input: ```SetState(5)```
//
// Example Output: ```null```
//
//export SetState
func SetState(fsmState C.int) *C.char {
	state, stateErr := getVPNState()
	if stateErr != nil {
		return getCError(stateErr)
	}
	return getCError(state.SetState(client.FSMStateID(fsmState)))
}

// InState checks if the FSM is in `fsmState`
//
// Example Input: ```InState(5)```
//
// Example Output: ```1, null```
//
//export InState
func InState(fsmState C.int) (C.int, *C.char) {
	state, stateErr := getVPNState()
	if stateErr != nil {
		return 0, getCError(stateErr)
	}

	if yes := state.InState(client.FSMStateID(fsmState)); yes {
		return 1, nil
	}
	return 0, nil
}

// FreeString frees a string that was allocated by the eduvpn-common Go library
//
// This happens when we return strings, such as errors from the Go lib back to the client.
// The client MUST thus ensure that this memory is freed using this function.
// Simply pass the pointer to the string in here
//
// Example Input: ```FreeString(strPtr)```
//
//export FreeString
func FreeString(addr *C.char) {
	C.free(unsafe.Pointer(addr))
}

func getCookie(c C.uintptr_t) (*cookie.Cookie, error) {
	if c == 0 {
		return nil, i18nerr.NewInternal("cookie is nil")
	}
	h := cgo.Handle(c)
	v, ok := h.Value().(*cookie.Cookie)
	if !ok {
		return nil, i18nerr.NewInternal("value is not a cookie")
	}
	// the cookie itself has a reference to the handle
	// such that we can return the same exact handle in callbacks
	// TODO: On first glance this might not make any sense, find a better way
	v.H = h
	return v, nil
}

// SetTokenHandler sets the token getters and token setters for OAuth
//
// Because the data that is saved does not contain OAuth tokens for server, the common lib asks and sets the tokens using these callback functions.
// The client can thus pass callbacks to this function so that the tokens can be securely stored in a keyring
//
// Client must pass two arguments to these functions
//
//   - getter is the void function that gets tokens from the client. It takes three arguments:
//
//   - The `server` for which to get the tokens for, marshalled as JSON and defined in types/server/server.go `Current`
//
//   - The `output` buffer
//
//   - The `length` of the output buffer. This 'output buffer' must contain the tokens, marshalled as JSON that is defined in types/server/server.go `Tokens`
//
// setter is the void function that sets tokens. It takes two arguments:
//
//   - The `server` for which to get the tokens for, marshalled as JSON and defined in types/server/server.go `Current`
//
//   - The `tokens`, defined in types/server/server.go `Tokens` marshalled as JSON
//
// It returns an error when the tokens cannot be set.
// Example Input: ```SetTokenHandler(getterFunc, setterFunc)```
//
// Example Output: ```null```
//
//export SetTokenHandler
func SetTokenHandler(getter C.TokenGetter, setter C.TokenSetter) *C.char {
	state, stateErr := getVPNState()
	if stateErr != nil {
		return getCError(stateErr)
	}
	state.TokenSetter = func(sid string, stype srvtypes.Type, t srvtypes.Tokens) {
		tJSON, err := getReturnData(t)
		if err != nil {
			log.Logger.Warningf("failed to get tokens for setting tokens in exports: %v", err)
			return
		}
		c1 := C.CString(sid)
		c2 := C.CString(tJSON)
		C.call_token_setter(setter, c1, C.int(stype), c2)
		FreeString(c1)
		FreeString(c2)
	}

	state.TokenGetter = func(sid string, stype srvtypes.Type) *srvtypes.Tokens {
		// create an output buffer with size 2048
		// In my testing tokens seem to be ~1033 bytes marshalled as JSON
		d := make([]byte, 2048)

		c1 := C.CString(sid)
		C.call_token_getter(getter, c1, C.int(stype), (*C.char)(unsafe.Pointer(&d[0])), C.size_t(len(d)))
		FreeString(c1)

		// get null pointer index as unmarshalling wants it without
		null := bytes.IndexByte(d, 0)
		if null < 0 {
			log.Logger.Warningf("output buffer is not NULL terminated")
			return nil
		}

		// no data found
		if null == 0 {
			log.Logger.Debugf("empty string returned when getting tokens")
			return nil
		}

		var gotT srvtypes.Tokens
		err := json.Unmarshal(d[:null], &gotT)
		if err != nil {
			log.Logger.Warningf("failed to get JSON data for getting tokens in exports: %v", err)
			return nil
		}
		return &gotT
	}

	return nil
}

// CookieNew creates a new cookie and returns it
//
// This value should not be parsed or converted somehow by the client
// This value is simply to pass back to the Go library
// This value has two purposes:
//
//   - Cancel a long running function
//
//   - Send a reply to a state transition (ASK_PROFILE and ASK_LOCATION)
//
// # Functions that take a cookie have it as the first argument
//
// Example Input: ```CookieNew()```
//
// Example Output: ```5```
//
//export CookieNew
func CookieNew() C.uintptr_t {
	c := cookie.NewWithContext(context.Background())
	return C.uintptr_t(cgo.NewHandle(c))
}

// CookieReply replies to a state transition using the cookie
//
// The data that is sent to the Go library is the second argument of this function
//
//   - `c` is the Cookie
//
//   - `data` is the data to send, e.g. a profile ID
//
// Example Input: ```CookieReply(myCookie, "split-tunnel-profile")```
//
// Example Output: ```null```
//
//export CookieReply
func CookieReply(c C.uintptr_t, data *C.char) *C.char {
	v, err := getCookie(c)
	if err != nil {
		return getCError(err)
	}
	err = v.Send(C.GoString(data))
	return getCError(err)
}

// CookieDelete deletes the cookie by cancelling it and deleting the underlying cgo handle
//
// This function MUST be called when the cookie that is created using `CookieNew` is no longer needed.
// Example Input: ```CookieDelete(myCookie)```
//
// Example Output: null
//
//export CookieDelete
func CookieDelete(c C.uintptr_t) *C.char {
	v, err := getCookie(c)
	if err != nil {
		return getCError(err)
	}
	// cancel the cookie and then delete the handle
	err = v.Cancel()
	cgo.Handle(c).Delete()
	return getCError(err)
}

// CookieCancel cancels the cookie
//
// This means that functions which take this as first argument, return if they're still running
// The error cause is always context.Canceled for that cancelled function: https://pkg.go.dev/context#pkg-variables
//
// This CookieCancel function can also return an error if cancelling was unsuccessful
// Example Input: ```CookieCancel(myCookie)```
//
// Example Output: null
//
//export CookieCancel
func CookieCancel(c C.uintptr_t) *C.char {
	v, err := getCookie(c)
	if err != nil {
		return getCError(err)
	}
	err = v.Cancel()
	if err != nil {
		return getCError(err)
	}
	return nil
}

// Not used in library, but needed to compile.
func main() { panic("compile with -buildmode=c-shared") }
