# Let's build a client using Python
To begin, let's follow the flow and see if we can figure out how it works.

## Registering

> The client starts up. It calls the Register function that communicates with the library that it has initialized

In Python, this works like the following:
- First import the library
```python
import eduvpn_common.main as edu

class Transitions:
    def __init__(self, common):
        self.common = common

# These arguments can be found in the docstring
# But also in the exports.go file
# For Python it's a bit different, we have split the arguments into the constructor and register
# Here we pass the client ID for OAuth, the version of the client and the directory where config files should be found
common=edu.EduVPN("org.eduvpn.app.linux", "0.0.1", "/tmp/test")

common.register(debug=True)

# we will come back to this later
transitions = Transitions(common)
common.register_class_callbacks(transitions)
```

Now after registering, we know that we have no servers configured (unless you're following this tutorial again with an existing `/tmp/test`). So we continue with step 4

## Discovery

>  If the client has no servers, or it wants to add a new server, the client calls `DiscoOrganizations` and `DiscoServers` to get the discovery files from the library.

```python
# Let's get them and print them
print(common.get_disco_organizations())
print(common.get_disco_servers())
```

We get a big JSON blob, so which format is this? From the Go documentation:

> DiscoOrganizations gets the organizations from discovery, returned as types/discovery/discovery.go Organizations marshalled as JSON

> DiscoServers gets the servers from discovery, returned as types/discovery/discovery.go Servers marshalled as JSON

If you follow these files, you see two structs, Servers and Organizations. These structs have json tags associated with them. You can use this structure to figure out how to parse the returned data. In case of discovery, it's very similar to the [JSON files from the discovery server](https://disco.eduvpn.org/v2).

## Adding a server

The next bullet point that we implement is the following:

> From this discovery list, it calls AddServer to add the server to the internal server list of eduvpn-common. This also calls necessary state transitions, e.g. for authorizing the server. The next call to ServerList then has this server included

The discovery servers contains a server called the demo server. Let's try to add it. To add it we need to pass the type of server we're adding. From discovery we can deduce that this is an institute access server as the JSON looks like the following:

```json
{
    "authentication_url_template": "",
    "base_url": "https://demo.eduvpn.nl/",
    "display_name": {
        "en": "Demo"
    },
    "server_type": "institute_access", # this is why we know it is Institute Access
    "support_contact": [
        "mailto:eduvpn@surf.nl"
    ]
},
```

From the Go documentation, we know that the identifier must be the Base URL:

> id is the identifier of the string
> - In case of secure internet: The organization ID
> - In case of custom server: The base URL
> - In case of institute access: The base URL


```python
# Compare this to the Go version, the non-interactive field is optional here as it is default False
common.add_server(edu.ServerType.INSTITUTE_ACCESS, "https://demo.eduvpn.nl/")
```

But we get an error!
```bash
eduvpn_common.main.WrappedError: fsm failed transition from 'Chosen_Server' to 'OAuth_Started', is this required transition handled?
```

This is the state machine we briefly mentioned before. Some functions require that you handle certain transitions. From the Go documentation, we can find this in the documentation as well that you must handle this transition. Let's handle it in Python to open the webbrowser for the OAuth process.

We do this with the python wrapper by defining a class of state transitions. This class was already added and registered with `register_class_callbacks`. However, there was no transition added. Let's add it
```python
import webbrowser
from eduvpn_common.event import class_state_transition
from eduvpn_common.state import State, StateType

class Transitions:
    def __init__(self, common):
        self.common = common

    @class_state_transition(State.OAUTH_STARTED, StateType.ENTER)
    def enter_oauth(self, old_state: State, url: str):
        webbrowser.open(url)
```

Now if you re-rerun the whole code with this transition added, your webbrowser should open.

Note that this state transition is essentially the same as the following code:

```python
-def handler(old: int, new: int, data: str):
-    # it's 6 because https://github.com/eduvpn/eduvpn-common/blob/b660911b5db000b43970f3754b5767bb50741360/client/fsm.go#L33
-    if new == 6:
-        webbrowser.open(data)
-        return True
-    return False
```

This is the code that is passed to the Go library. It handles certain states and returns `False` (zero) if a state is not handled, `True` (non-zero) if it is. If you define your own wrapper you should build an abstraction layer that resolves to a handler similar as above. This handler should be passed as a C function to the Go library when registering.

After you have authorized the application through the portal using the webbrowser, the server should have been added:

```python
print(common.get_servers())
```

Returns:

```json
{
  "institute_access_servers": [
    {
      "display_name": {
        "en": "Demo"
      },
      "identifier": "https://demo.eduvpn.nl/",
      "profiles": {
        "current": ""
      },
      "delisted": false
    }
  ]
}
```

The format of this JSON is specified in the Go documentation:

`(in exports/exports.go)`
> It returns the server list as a JSON string defined in types/server/server.go List

## Obtaining a VPN configuration from the server

The next part of the flow is:

> When the user selects a server to connect to in the UI, it calls the GetConfig to get a VPN configuration for this server. This function transitions the state machine multiple times. The client uses these state transitions for logging or even updating the UI. The client then connects

Let's try it, the required arguments are the same for adding a config in the Python wrapper:

```python
print(common.get_config(edu.ServerType.INSTITUTE_ACCESS, "https://demo.eduvpn.nl"))
```

However, this gives an exception:

```bash
eduvpn_common.main.WrappedError: fsm failed transition from 'Request_Config' to 'Ask_Profile', is this required transition handled?
```

A similar error to the OAuth error we had before. This `Ask_Profile` transition is there for the client/user to choose a profile as this server has multiple profiles defined.

To handle this transition and thus choose a profile to continue, we must do multiple steps:
- Add the condition to the transitions class
- Parse the data that we get back
- Reply with a choice for the profile 

If we add the condition and print the data:

```python
@class_state_transition(State.ASK_PROFILE, StateType.ENTER)
def enter_ask_profile(self, old_state: State, data: str):
    print("profiles:", data)
```

we get back the following JSON (from the Go docs: `The data for this transition is defined in types/server/server.go RequiredAskTransition with embedded data Profiles in types/server/server.go`):

```python
{
  "cookie": 4,
  "data": {
    "map": {
      "internet": {
        "display_name": {
          "en": "Internet"
        },
        "supported_protocols": [
          1,
          2
        ]
      },
      "internet-split": {
        "display_name": {
          "en": "No rfc1918 routes"
        },
        "supported_protocols": [
          1,
          2
        ]
      }
    },
    "current": ""
  }
}
```

This thus gives you the list of profiles with a so-called "cookie". This *cookie* is used to confirm the choice to the Go library. To do so we must do the following to handle this:

```python
import json

# Do this inside the Transitions class
@class_state_transition(State.ASK_PROFILE, StateType.ENTER)
def enter_ask_profile(self, old_state: State, data: str):
    # parse the json
    json_dict = json.loads(data)
    
    self.common.cookie_reply(json_dict["cookie"], "internet")
```

If we then re-run the code, we get back the following JSON (from the Go docs: `The return data is the configuration, marshalled as JSON and defined in types/server/server.go Configuration`)

```python
{
  "config": "the WireGuard config",
  "protocol": 2, # 2 specifies WireGuard
  "default_gateway": true
}
```

## Cleanup
The flow also mentioned:

> When the client is done, it calls `Deregister` such that the most up to date internal state is saved to disk. Note that eduvpn-common also saves the internal state .e.g. after obtaining a VPN configuration

Let's be a nice client and do this:

```python
common.deregister()
```

If we then call any function, we get an error, so it is important that you do this on exit:

```python
print(common.get_servers())
>>> eduvpn_common.main.WrappedError: No state available, did you register the client?
```

But when we register again and then get the list of servers, the servers are retrieved from disk:

```python
common=edu.EduVPN("org.eduvpn.app.linux", "0.0.1", "/tmp/test")
common.register(debug=True)
print(common.get_servers())
```

gives

```json
{
  "institute_access_servers": [
    {
      "display_name": {
        "en": "Demo"
      },
      "identifier": "https://demo.eduvpn.nl/",
      "profiles": {
        "map": {
          "internet": {
            "display_name": {
              "en": "Internet"
            },
            "supported_protocols": [
              1,
              2
            ]
          },
          "internet-split": {
            "display_name": {
              "en": "No rfc1918 routes"
            },
            "supported_protocols": [
              1,
              2
            ]
          }
        },
        "current": "internet"
      },
      "delisted": false
    }
  ]
}
```

Note the difference with the previous JSON, the profiles are now initialized because we have gotten a configuration before.

If the `/tmp/test` directory is removed (the argument that was passed to register), we get no servers again:

```python
import shutil
shutil.rmtree("/tmp/test")
common=edu.EduVPN("org.eduvpn.app.linux", "0.0.1", "/tmp/test")
common.register(debug=True)
print(common.get_servers())
```

gives `"{}"`, an empty JSON object string
