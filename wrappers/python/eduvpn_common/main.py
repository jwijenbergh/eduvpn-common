import ctypes
from enum import IntEnum
from typing import Any, Callable, Iterator, Optional

from eduvpn_common.loader import initialize_functions, load_lib
from eduvpn_common.types import ReadRxBytes, TokenGetter, TokenSetter, VPNStateChange, decode_res, encode_args


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


class Jar(object):
    """A cookie jar"""

    def __init__(self, canceller):
        self.cookies = []
        self.canceller = canceller

    def add(self, cookie):
        self.cookies.append(cookie)

    def delete(self, cookie):
        self.cookies.remove(cookie)

    def cancel(self):
        for cookie in self.cookies:
            self.canceller(cookie)


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
        self.jar = Jar(lambda x: self.go_function(self.lib.CookieCancel, x))
        self.callback = None
        self.token_setter = None
        self.token_getter = None

        # Load the library
        self.lib = load_lib()
        initialize_functions(self.lib)

    def go_cookie_function(self, func: Any, *args: Iterator) -> Any:
        cookie = self.lib.CookieNew()
        self.jar.add(cookie)
        res = self.go_function(func, cookie, *args)
        self.jar.delete(cookie)
        self.lib.CookieDelete(cookie)
        return res

    def go_function(self, func: Any, *args: Iterator) -> Any:
        """Call an internal go function and properly forward the arguments.
        Also handles decoding the result

        :param func: Any: The Go function to call from the shared library
        :param \*args: Iterator: The arguments to call the function with

        :meta private:
        """
        # The functions all have at least one arg type which is the name of the client
        args_gen = encode_args(list(args), func.argtypes)
        res = func(*(args_gen))
        return decode_res(func.restype)(self.lib, res)

    def deregister(self) -> None:
        """Deregister the Go shared library.
        This removes the object from internal bookkeeping and saves the configuration
        """
        self.go_function(self.lib.Deregister)
        global global_object
        global_object = None

    def register(self, handler: Optional[Callable] = None, debug: bool = False) -> None:
        """Register the Go shared library.
        This makes sure the FSM is initialized and that we can call Go functions

        :param handler: Optional[Callable]:  (Default value = None): The handler that runs state transitions
        :param debug: bool:  (Default value = False): Whether or not we want to enable debug logging

        """
        global global_object
        if global_object is not None:
            raise Exception("Already registered")
        self.callback = handler
        global_object = self
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

    def add_server(self, _type: ServerType, _id: str, ni: bool = False) -> None:
        """Add a server

        :param _type: ServerType: The type of server e.g. SERVER.INSTITUTE_ACCESS
        :param _id: str: The identifier of the server, e.g. "https://vpn.example.com/"
        :param ni: bool: Whether the server should be added non interactively, meaning no callbacks

        :raises WrappedError: An error by the Go library
        """
        add_err = self.go_cookie_function(self.lib.AddServer, int(_type), _id, ni)

        if add_err:
            forwardError(add_err)

    def get_expiry_times(self) -> str:
        expiry, expiry_err = self.go_function(self.lib.ExpiryTimes)
        if expiry_err:
            forwardError(expiry_err)
        return expiry

    def get_current_server(self) -> str:
        server, server_err = self.go_function(self.lib.CurrentServer)
        if server_err:
            forwardError(server_err)
        return server

    def get_disco_organizations(self) -> str:
        orgs, _ = self.go_cookie_function(self.lib.DiscoOrganizations)
        # TODO: Log error
        return orgs

    def get_disco_servers(self) -> str:
        servers, _ = self.go_cookie_function(self.lib.DiscoServers)
        # TODO: Log error
        return servers

    def get_servers(self) -> str:
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

    def set_state(self, state: int):
        state_err = self.go_function(self.lib.SetState, state)
        if state_err:
            forwardError(state_err)

    def get_config(
        self, _type: ServerType, identifier: str, prefer_tcp: bool = False
    ) -> str:
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
        config, config_err = self.go_cookie_function(
            self.lib.GetConfig,
            int(_type),
            identifier,
            prefer_tcp,
        )

        if config_err:
            forwardError(config_err)

        return config

    def cleanup(self) -> None:
        """Cleanup the vpn connection

        :param tokens: str  (Default value = ""): The OAuth tokens if available

        :raises WrappedError: An error by the Go library
        """
        cleanup_err = self.go_cookie_function(self.lib.Cleanup)

        if cleanup_err:
            forwardError(cleanup_err)

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
        location_err = self.go_cookie_function(self.lib.SetSecureLocation, country_code)

        # If there is a location event, set it so that the wait callback finishes
        # And so that the Go code can move to the next state
        if location_err:
            forwardError(location_err)

    def set_token_handler(self, getter: Callable, setter: Callable) -> None:
        self.token_setter = setter
        self.token_getter = getter
        handler_err = self.go_function(self.lib.SetTokenHandler, token_getter, token_setter)

        if handler_err:
            forwardError(handler_err)

    def cookie_reply(self, cookie: int, data: str) -> None:
        """Reply with the given cookie and data"""
        cookie_err = self.go_function(self.lib.CookieReply, cookie, data)
        if cookie_err:
            forwardError(cookie_err)

    def renew_session(self) -> None:
        """Renew the session. This invalidates the tokens and runs the necessary callbacks to log back in

        :raises WrappedError: An error by the Go library
        """
        renew_err = self.go_cookie_function(self.lib.RenewSession)

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
        dropped, dropped_err = self.go_cookie_function(
            self.lib.StartFailover,
            gateway,
            wg_mtu,
            readrxbytes,
        )
        if dropped_err:
            forwardError(dropped_err)
        return dropped

    def cancel(self):
        self.jar.cancel()


global_object: Optional[EduVPN] = None

@TokenSetter
def token_setter(server: ctypes.c_char_p, tokens: ctypes.c_char_p):
    global global_object
    if global_object is None:
        return
    if global_object.token_setter is None:
        return 0
    global_object.token_setter(server.decode(), tokens.decode())

@TokenGetter
def token_getter(server: ctypes.c_char_p, buf: ctypes.c_char_p, size: ctypes.c_size_t):
    global global_object
    if global_object is None:
        return
    if global_object.token_getter is None:
        return
    got = global_object.token_getter(server.decode())
    if got is None:
        return

    outbuf = ctypes.cast(buf, ctypes.POINTER(ctypes.c_char * size))
    outbuf.contents.value = got.encode("utf-8")

@VPNStateChange
def state_callback(old_state: int, new_state: int, data: str) -> int:
    """The internal callback that is passed to the Go library

    :param old_state: int: The old state
    :param new_state: int: The new state
    :param data: str: The data that still needs to be converted by parsing the JSON

    :meta private:
    """
    global global_object
    if global_object is None:
        return 0
    if global_object.callback is None:
        return 0
    handled = global_object.callback(old_state, new_state, data.decode("utf-8"))
    if handled:
        return 1
    return 0
