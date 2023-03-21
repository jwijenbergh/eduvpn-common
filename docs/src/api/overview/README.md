# API overview

This chapter defines the API that is used to build an eduVPN/Let's Connect! client. We explain what functions there are, what their use is and what a typical flow is for creating an eduVPN client with this library. The extensive language specific documentation will be given in separate sections.

## Table of contents
1. [Types](#types)
   - [JSON](#json)
   - [Errors](#errors)
   - [States](#states)
2. [Functions](#functions)
   - [Registering](#registering)
   - [Add a server](#add-a-server)
   - [Remove a server](#remove-a-server)
   - [List of servers](#list-of-servers)
   - [Current server](#current-server)
   - [Get VPN config](#get-vpn-config)
   - [Expiry Times](#expiry-times)
   - [Set Profile ID](#set-profile-id)
   - [Set Secure Location](#set-profile-id)
   - [Discovery Servers](#discovery-servers)
   - [Discovery Organizations](#discovery-organizations)
   - [Cancel OAuth](#cancel-oauth)
   - [Set Support WireGuard](#set-support-wireguard)
   - [Cleanup](#cleanup)
   - [Renew Session](#renew-session)
   - [Secure Location List](#secure-location-list)
   - [Start Failover](#start-failover)
   - [Cancel Failover](#cancel-failover)
   - [Deregistering](#deregistering)
   - [Free String](#free-string)
   
## Types
This section describes a few types that are either used as arguments or return values

### JSON

The message passing between language X and Go is done using JSON. This means that every type that we mention here is converted to JSON. For a list of public types that are returned and their JSON representation see: <https://github.com/eduvpn/eduvpn-common/blob/v2/types/>. So for example, if we say that we return `types.server.Expiry` (meaning the `Expiry` struct defined in the [types/server](https://github.com/eduvpn/eduvpn-common/blob/v2/types/server/server.go), we will return the following json representation:

```json
{
	"start_time": 5,
	"end_time": 6,
	"button_time": 7,
	"countdown_time": 8,
	"notification_times": [1, 2],
}
```

But in the Go API, this means that we actually return the struct `types.server.Expiry`, so e.g.

```Go
// Get the return type
rt := somefunction()
fmt.Println(rt.start_time)
```

If we for example have an enumeration, e.g. `types.protocol.Protocol`, this is converted as an integer. E.g. `Unknown` translates to `0`, `OpenVPN` to `1` and `WireGuard` to `2`.

You can also see this when reading the source code. In Go this was denoted with the `iota` keyword, meaning start at 0 and increment on following const declarations.

> **_NOTE:_**  strings returned by CGO (`*C.char`) MUST be freed by the [FreeString](#free-string) function.

### Errors
Errors are encoded as error messages (`*C.char`) in the CGO API. For regular Go, this is just `error`. Errors are *hard-fail* unless otherwise defined. Hard-fail means that the associated data that is returned will be nil/default value if an error is returned.

> **_NOTE:_**  In case of CGO this error (a `*C.char`) MUST be freed by the [FreeString](#free-string) function.

### States

The `states` is an enumeration of the possible states that the state machine has defined. Starting at 0:

- `Deregistered`: the client is not yet registered 
- `No Server`: the client has registered and we're about to choose a server
- `Ask Location`: eduvpn-common is asking the client for a secure internet location
  - A slice/list `[]string` of locations (country codes). For the C API: a JSON list e.g. 
  ```json
  ["nl", "de"]
  ```
- `Chosen Location`: a secure internet location has been chosen
- `Loading Server`: the server is loading, e.g. doing a request
- `Chosen Server`: the server has been chosen
- `OAuth Started`: the OAuth procedure has started
  - Data with this transition: the URL to open in the browser as a string
- `Authorized`: authorization is finished, OAuth process is done
- `Request Config`: eduvpn-common is requesting a config from the server
- `Ask Profile`: eduvpn-common is asking the client for a profile
  - Data with this transition: `types.server.Profiles`.
- `Chosen Profile`: A profile has been chosen by the client
- `Got Config`: A VPN Configuration has been obtained for the current server and the client should be ready to connect

The states with data are required transitions, handle them by returning True/non-zero (e.g. 1) in your callback function. We will discuss this callback function later.
   
## Functions
For each function, we define it by giving a small description and then the arguments and return types that follows. We will also describe which type of state transitions must be handled by the client in order to call this function.

The functions are defined, more or less, in the order that you might call them.

### Registering
The first function that a client calls is the `register`
function. This function is meant as a registration/constructor of the
library and can only be called once during the lifetime of the library
(until `deregister` is called).

The arguments are:
- The name of the client as a ClientID (`string`), e.g. `org.eduvpn.app.linux`
- The version field that is used in the HTTP User agent (`string`), e.g. `1.0.0`
- The directory where config files are stored, absolute or relative (`string`), e.g. `/home/eduvpn/.config/eduvpn`
- A boolean that indicates whether or not debugging is enabled, debugging means log more verbose
- The callback function which is used for state transitions. Takes three arguments, old state (integer), new state (integer), data (string, JSON)

Return type:
- An error

<details>
  <summary>Python</summary>

```python
from eduvpn_common.main import EduVPN

# These integers are an enumeration under the hood
# See https://github.com/eduvpn/eduvpn-common/blob/v2/client/fsm.go#L17
def callback(old_state: int, new_state: int, data: str):
    pass

# Some arguments are in the class constructor
eduvpn = EduVPN("org.eduvpn.app.linux", "1.0.0", "/home/eduvpn/.config/eduvpn")
eduvpn.register(handler=callback, debug=True)
```
</details>
<details>
<summary>Go</summary>

```go
import "github.com/eduvpn/eduvpn-common/client"

// Note: these integer types may also be defined as client.FSMStateID
// See: https://github.com/eduvpn/eduvpn-common/blob/v2/client/fsm.go#L17
// The data here is an interface as we do not convert anything to JSON for the Go API
// You would type check depending on the state transition, e.g. https://github.com/eduvpn/eduvpn-common/blob/85aec7dbe5ba18b1b1e2ea3cd35b0d5797c404c3/cmd/cli/main.go#L101
func stateCallback(oldState int, newState int, data interface{}) {
	// do something
}

c := client.Client{}
c.Register("org.eduvpn.app.linux", "1.0.0", "/home/eduvpn/.config/eduvpn", stateCallback, true)
```
</details>

### Add a server

Eduvpn-common keeps track of the servers that the user/client has defined. To add a server, the `add server` function must be called. 

Arguments: 
- The type of server (`types.server.Type`)
- The identifier of the server (`string`), in case of secure internet the Org ID, otherwise the base URL

State transitions that must be handled:
- `OAuth_Started`: If the server needs authorization. Open the URL in the browser
- `Ask_Profile`: For choosing the correct profile. Acknowledge the request with [SetProfileID](#set-profile-id)
- `Ask_Location`: For asking the secure internet location. Acknowledge the request with [SetSecureLocationID](#set-secure-location-id)


Return type:
- An error message (`string`). Empty string if no error

### Remove a server
You can also remove a server again, using the `remove server` function.

Arguments: 
- The type of server (`types.server.Type`)
- The identifier of the server (`string`), in case of secure internet the Org ID, otherwise the base URL

Return type:
- An error

### List of servers
To get all the currently configured servers and some of their associated data, the `server list` function is used.

Arguments:
- None

Return type:
- The list of servers (`types.server.List`)
- An error


### Current server
After adding or getting a configuration for a server, the Go library sets that server as the `current` server internally. This is so that EduVPN clients do not even have to keep track of which server is currently configured.

Arguments:
- None

Return type:
- The current server (`types.server.Current`)
- An error


### Get VPN config
To get a VPN configuration (`WireGuard` or `OpenVPN`) for a server, the `get config` function is used. Note that the server must first have been added before calling this function.

Arguments: 
- The type of server (`types.server.Type`)
- The identifier of the server (`string`), in case of secure internet the Org ID, otherwise the base URL
- A boolean which indicates whether or not prefer TCP should be set
- Tokens used for authorization `types.server.Tokens`. If no tokens, pass a default struct or "{}" with the C JSON API

State transitions that must be handled:
- `OAuth_Started`: If the server needs to trigger re-authorization. Open the URL in the browser
- `Ask_Profile`: For choosing the correct profile. Acknowledge the request with [SetProfileID](#set-profile-id)
- `Ask_Location`: For asking the secure internet location. Acknowledge the request with [SetSecureLocation](#set-secure-location)

Return type:
- The VPN configuration with associated data (`types.server.Configuration`). Note that this also contains Tokens that can be saved by the client. Note that the VPN configuration itself has "script-security 0" added to the end if it's an OpenVPN config. This is to disable OpenVPN scripts from being run by default. A client may override this if it has a good reason to.
- An error

### Expiry Times
To get the different times regarding expiry, the function `expiry times` is used.

Arguments:
- None

Return type:
- The expiry times (`types.server.Expiry`)
- An error

### Set Profile ID
Set the profile ID for the current server. To be used as a reply to `Ask_Location` or just to change the current profile before getting a configuration

Arguments:
- The profile ID (`string`)

Return type:
- An error message (`string`). Empty string if no error

### Set Secure Location
Set the secure internet location for the current server. To be used as a reply to `Ask_Location` or just to change the current location before getting a configuration

Arguments:
- The location as a country code (`string`)

Return type:
- An error


### Discovery servers
Get the discovery servers from <https://disco.eduvpn.org/v2/server_list.json>. This returns a cached list if the server should not be contacted according to the eduvpn spec at <https://github.com/eduvpn/documentation/blob/v3/SERVER_DISCOVERY.md>. So you do not have to worry about when to call this function. However, clients may cache further to prevent parsing this data every time.

Arguments:
- None

Return type:
- The servers (`types.discovery.Servers`)
- An error message (`string`). Empty string if no error. Note that if an error is returned, when building this library in [release mode](/gettingstarted/building/release.md) this function is guaranteed to return a result for the servers, unless there is an issue with parsing the internal data representation. So the error can be used for logging instead of being a hard-fail

### Discovery organizations
Get the discovery organizations from <https://disco.eduvpn.org/v2/organizations_list.json>. This returns a cached list if the server should not be contacted according to the eduvpn spec at <https://github.com/eduvpn/documentation/blob/v3/SERVER_DISCOVERY.md>. So you do not have to worry about when to call this functions. Clients may cache further to prevent parsing this data every time.

Arguments:
- None

Return type:
- The organizations (`types.discovery.Organizations`)
- An error. Note that if an error is returned, when building this library in [release mode](/gettingstarted/building/release.md) this function is guaranteed to return a result for the organizations, unless there is an issue with parsing the internal data representation. So the error can be used for logging instead of being a hard-fail

### Cancel OAuth
Cancel the current OAuth process. 

Arguments:
- None

Return type:
- An error

### Set Support WireGuard
> **_NOTE:_**  This function might be removed in the future. This is currently here for the Linux client and also for the failover procedure.

WireGuard is by default enabled. To indicate that the client does not support WireGuard, you can use the `SetSupportWireGuard` function. 

Arguments:
- A boolean that indicates whether or not WireGuard should be enabled or disabled

Return type:
- An error

### Cleanup
Cleans up the VPN connection by sending a /disconnect

Arguments:
- None

### Renew Session
Renew session is used for renewing the VPN. This does not give you a configuration, but merely deletes the OAuth tokens from the current server.

Arguments:
- None

State transitions that must be handled:
- `OAuth_Started`: If the server needs authorization. Open the URL in the browser

Return type:
- An error

### Secure Location List
> **_NOTE:_**  This function might be removed in the future as clients can parse this out of discovery themselves

This gets the list of secure internet locations that are available in discovery.

Arguments:
- None

Return type:
- A slice/list of country codes (`[]string`)
- An error


### Start Failover
Eduvpn-common also has a `failover` implementation that can be started with `start failover`. This is used to check whether or not the VPN can reach the internet. Useful when connecting to WireGuard or OpenVPN over UDP. This function sends ICMP echo pings for a maximum of 10 seconds up until it is dropped. If a ping can be send and a pong returns within a timeout of 2 seconds, it returns after this pong is received.

If this functions tells you that the VPN is dropped, it might be wise to get a configuration again using Prefer TCP (see [Get VPN Config](#get-vpn-config)) and disabling WireGuard (see [Set Support Wireguard](#set-support-wireguard)). Note that this `start failover` function also checks if the current profile supports OpenVPN and will return an error if it doesn't.

Arguments:
- Gateway (`string`), the IP endpoint to ping to check if the VPN can reach the internet. As the name suggests, this should be the gateway
- MTU (`int`), the packet size to send for each ping. As the name suggests, this should be the MTU of the connection
- `readRxBytes`, a function that returns the current Rx bytes counter (`int64` in Go, `long long int` in CGO api) for the connection. Used to check if any bytes have been received in an interval of maximum 10 seconds

Return type:
- Dropped: a boolean that indicates whether or not the connection is dropped according to eduvpn-common. This means that the VPN is unable to reach the gateway
- An error

### Cancel Failover
To cancel the current failover process, e.g. due to disconnecting, you should call `cancel failover`. This makes the original failover function return dropped `false` and an error indicating cancellation.

Arguments:
- None

Return type:
- An error

### Deregistering
When the client is done, e.g. on application close, it can call the `deregister function` to save the internal state to disk and afterwards empty out this state. This can also be used to re-register, but this is probably not something you have to do.

Arguments:
- None

Return type:
- An error

### Free String
> **_NOTE:_**  This does not apply for the pure Go API

With the Go <-> X language API (using CGO), there is a function to free a string (`*C.char`). This is called `free string`

Arguments:
- The pointer to the string

Return type:
- None
