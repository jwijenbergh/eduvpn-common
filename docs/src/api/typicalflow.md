# Typical flow for a client
> **_NOTE:_** This uses the function names that are defined in the exports file in Go. For your own wrapper/the Python wrapper they are different. But the general flow is the same
1. The client starts up. It calls the `Register` function that communicates with the library that it has initialized
2. It gets the list of servers using `ServerList`
3. When the user selects a server to connect to in the UI, it calls the `GetConfig` to get a VPN configuration for this server. This function transitions the state machine multiple times. The client uses these state transitions for logging or even updating the UI. The client then connects
	- New feature in eduvpn-common: Check if the VPN can reach the gateway after the client is connected by calling `StartFailover`
4. If the client has no servers, or it wants to add a new server, the client calls `DiscoOrganizations` and `DiscoServers` to get the discovery files from the library. This even returns cached copies if the organizations or servers should not have been updated [according to the documentation](https://docs.eduvpn.org/server/v3/server-discovery.html)
	- From this discovery list, it calls `AddServer` to add the server to the internal server list of eduvpn-common. This also calls necessary state transitions, e.g. for authorizing the server. The next call to `ServerList` then has this server included
	- It can then get a configuration for this server like we have explained in *step 3*
5. When a configuration has been obtained, the internal state has changed and the client can get the current server that was configured using `CurrentServer`. `CurrentServer` can also be called after startup if a server was previously set as the current server.
6. When the VPN disconnects, the client calls `Cleanup` so that the server resources are cleaned up by calling the `/disconnect` endpoint
7. A server can be removed with the `RemoveServer` function
8. When the client is done, it calls `Deregister` such that the most up to date internal state is saved to disk. Note that eduvpn-common also saves the internal state .e.g. after obtaining a VPN configuration
