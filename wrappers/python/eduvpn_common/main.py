from enum import IntEnum
from typing import Any, Callable, Iterator, Optional

from eduvpn_common.loader import initialize_functions, load_lib
from eduvpn_common.types import (ReadRxBytes, VPNStateChange, decode_res,
                                  encode_args, get_bool, get_data_error)


class WrappedError(Exception):
    pass


def forwardError(error: bytes | str):
    # TODO: HACK, remove this
    if isinstance(error, str):
        raise WrappedError(error)
    raise WrappedError(error.decode("utf-8"))


class ServerType(IntEnum):
    UNKNOWN = 0
    INSTITUTE_ACCESS = 1
    SECURE_INTERNET = 2
    CUSTOM = 3


class EduVPN(object):
    """The main class used to communicate with the Go library.
    It registers the client with the library and then calls the needed appropriate functions

    :param name: str: The name of the client. For commonly used names, see https://git.sr.ht/~fkooman/vpn-user-portal/tree/v3/item/src/OAuth/ClientDb.php. E.g. org.eduvpn.app.linux, if this name has "letsconnect" in it, then it is a Let's Connect! variant
    :param version: str: The version number of the client as a string
    :param config_directory: str: The directory (absolute/relative) where to store the files

    """

    def __init__(self, name: str, version: str, config_directory: str):
        self.name = name
        self.version = version
        self.config_directory = config_directory

        # Load the library
        self.lib = load_lib()
        initialize_functions(self.lib)

    def go_function(
        self, func: Any, *args: Iterator, decode_func: Optional[Callable] = None
    ) -> Any:
        """Call an internal go function and properly forward the arguments.
        Also handles decoding the result

        :param func: Any: The Go function to call from the shared library
        :param \*args: Iterator: The arguments to call the function with
        :param decode_func: Optional[Callable]:  (Default value = None): The function to decode the result into a Python type

        :meta private:
        """
        # The functions all have at least one arg type which is the name of the client
        args_gen = encode_args(list(args), func.argtypes)
        res = func(*(args_gen))
        if decode_func is None:
            return decode_res(func.restype)(self.lib, res)
        else:
            return decode_func(self.lib, res)

    def cancel_oauth(self) -> None:
        """Cancel the OAuth process"""
        cancel_oauth_err = self.go_function(self.lib.CancelOAuth)

        if cancel_oauth_err:
            forwardError(cancel_oauth_err)

    def deregister(self) -> None:
        """Deregister the Go shared library.
        This removes the object from internal bookkeeping and saves the configuration
        """
        self.go_function(self.lib.Deregister)
        global callback_object
        callback_object = None

    def register(self, handler: Optional[Callable] = None, debug: bool = False) -> None:
        """Register the Go shared library.
        This makes sure the FSM is initialized and that we can call Go functions

        :param handler: Optional[Callable]:  (Default value = None): The handler that runs state transitions
        :param debug: bool:  (Default value = False): Whether or not we want to enable debug logging

        """
        global callback_object
        if callback_object is not None:
            raise Exception("Already registered")
        callback_object = handler
        register_err = self.go_function(
            self.lib.Register,
            self.name,
            self.version,
            self.config_directory,
            state_callback,
            debug,
        )

        if register_err:
            forwardError(register_err)

    def add_server(self, _type: ServerType, _id: str) -> None:
        """Add a server

        :param _type: ServerType: The type of server e.g. SERVER.INSTITUTE_ACCESS
        :param _id: str: The identifier of the server, e.g. "https://vpn.example.com/"

        :raises WrappedError: An error by the Go library
        """
        add_err = self.go_function(self.lib.AddServer, int(_type), _id)

        if add_err:
            forwardError(add_err)

    def get_expiry_times(self) -> Optional[str]:
        expiry, expiry_err = self.go_function(self.lib.ExpiryTimes)
        if expiry_err:
            forwardError(expiry_err)
        return expiry

    def get_current_server(self) -> Optional[str]:
        server, server_err = self.go_function(self.lib.CurrentServer)
        if server_err:
            forwardError(server_err)
        return server

    def get_secure_locations(self) -> Optional[str]:
        locs, locs_err = self.go_function(self.lib.SecureLocationList)
        if locs_err:
            forwardError(locs_err)
        return locs

    def get_disco_organizations(self) -> Optional[str]:
        orgs, _ = self.go_function(self.lib.DiscoOrganizations)
        # TODO: Log error
        return orgs

    def get_disco_servers(self) -> Optional[str]:
        servers, _ = self.go_function(self.lib.DiscoServers)
        # TODO: Log error
        return servers

    def get_servers(self) -> Optional[str]:
        servers, servers_err = self.go_function(self.lib.ServerList)
        if servers_err:
            forwardError(servers_err)
        return servers

    def remove_server(self, _type: ServerType, _id: str) -> None:
        """Remove a server

        :param _type: ServerType: The type of server e.g. SERVER.INSTITUTE_ACCESS
        :param _id: str: The identifier of the server, e.g. "https://vpn.example.com/"

        :raises WrappedError: An error by the Go library
        """
        remove_err = self.go_function(self.lib.RemoveServer, int(_type), _id)

        if remove_err:
            forwardError(remove_err)

    def get_config(
        self, _type: ServerType, identifier: str, prefer_tcp: bool = False, tokens: str = "{}"
    ) -> Optional[str]:
        """Get an OpenVPN/WireGuard configuration from the server

        :param _type: ServerType: The type of server e.g. SERVER.INSTITUTE_ACCESS
        :param identifier: str: The identifier of the server, e.g. URL or ORG ID
        :param prefer_tcp: bool:  (Default value = False): Whether or not to prefer TCP
        :param tokens: str  (Defualt value = ""): The OAuth tokens if available

        :meta private:

        :raises WrappedError: An error by the Go library

        :return: The configuration and configuration type ('openvpn' or 'wireguard') as a JSON string
        :rtype: str
        """
        # Because it could be the case that a profile callback is started, store a threading event
        # In the constructor, we have defined a wait event for Ask_Profile, this waits for this event to be set
        # The event is set in self.set_profile
        config, config_err = self.go_function(
            self.lib.GetConfig,
            _type,
            identifier,
            prefer_tcp,
            tokens,
        )

        if config_err:
            forwardError(config_err)

        return config

    def cleanup(self, tokens: str = "") -> None:
        """Cleanup the vpn connection

        :param tokens: str  (Default value = ""): The OAuth tokens if available

        :raises WrappedError: An error by the Go library
        """
        cleanup_err = self.go_function(self.lib.Cleanup, tokens)

        if cleanup_err:
            forwardError(cleanup_err)

    def token_calback(self, srv: Server, tok: Token):
        if self.token_callback is None:
            return
        self.token_callback(srv, tok)

    def set_profile(self, profile_id: str) -> None:
        """Set the profile of the current server

        :param profile_id: str: The profile id of the chosen profile for the server

        :raises WrappedError: An error by the Go library
        """
        # Set the profile id
        profile_err = self.go_function(self.lib.SetProfileID, profile_id)

        # If there is a profile event, set it so that the wait callback finishes
        # And so that the Go code can move to the next state
        if profile_err:
            forwardError(profile_err)

    def set_secure_location(self, country_code: str) -> None:
        """Set the secure location

        :param country_code: str: The country code of the new location

        :raises WrappedError: An error by the Go library
        """
        # Set the location by country code
        location_err = self.go_function(self.lib.SetSecureLocation, country_code)

        # If there is a location event, set it so that the wait callback finishes
        # And so that the Go code can move to the next state
        if location_err:
            forwardError(location_err)

    def renew_session(self) -> None:
        """Renew the session. This invalidates the tokens and runs the necessary callbacks to log back in

        :raises WrappedError: An error by the Go library
        """
        renew_err = self.go_function(self.lib.RenewSession)

        if renew_err:
            forwardError(renew_err)

    def set_support_wireguard(self, support: bool) -> None:
        """Indicates whether or not the OS supports WireGuard connections.

        :param support: bool: whether or not wireguard is supported

        :raises WrappedError: An error by the Go library
        """
        support_err = self.go_function(self.lib.SetSupportWireguard, support)

        if support_err:
            forwardError(support_err)

    def start_failover(
        self, gateway: str, wg_mtu: int, readrxbytes: ReadRxBytes
    ) -> bool:
        dropped, dropped_err = self.go_function(
            self.lib.StartFailover,
            gateway,
            wg_mtu,
            readrxbytes,
            decode_func=lambda lib, x: get_data_error(lib, x, get_bool),
        )
        if dropped_err:
            forwardError(dropped_err)
        return dropped

    def cancel_failover(self):
        cancel_err = self.go_function(self.lib.CancelFailover)
        if cancel_err:
            forwardError(cancel_err)


callback_object: Optional[Callable] = None


@UpdateToken
def token_callback(name: bytes, srv, tok):
    name_decoded = name.decode()
    if name_decoded not in eduvpn_objects:
        return 0
    obj = eduvpn_objects[name_decoded]
    srv_conv = get_transition_server(obj.lib, srv)
    tok_conv = get_tokens(obj.lib, tok)
    obj.token_callback(
        srv_conv, tok_conv
    )


@VPNStateChange
def state_callback(old_state: int, new_state: int, data: str) -> int:
    """The internal callback that is passed to the Go library

    :param old_state: int: The old state
    :param new_state: int: The new state
    :param data: str: The data that still needs to be converted by parsing the JSON

    :meta private:
    """
    if callback_object is None:
        return 0
    handled = callback_object(old_state, new_state, data.decode("utf-8"))
    if handled:
        return 1
    return 0
