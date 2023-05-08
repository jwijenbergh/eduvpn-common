import pathlib
import platform
from collections import defaultdict
from ctypes import CDLL, c_char_p, c_int, c_void_p, cdll

from eduvpn_common import __version__
from eduvpn_common.types import (
    BoolError,
    DataError,
    ReadRxBytes,
    TokenGetter,
    TokenSetter,
    VPNStateChange,
)


def load_lib() -> CDLL:
    """The function that loads the Go shared library

    :meta private:

    :return: The Go shared library loaded with cdll.LoadLibrary from ctypes
    :rtype: CDLL
    """
    lib_prefixes = defaultdict(
        lambda: "lib",
        {
            "windows": "",
        },
    )

    lib_suffixes = defaultdict(
        lambda: ".so",
        {
            "windows": ".dll",
            "darwin": ".dylib",
        },
    )

    os = platform.system().lower()

    libname = "eduvpn_common"
    libfile = f"{lib_prefixes[os]}{libname}-{__version__}{lib_suffixes[os]}"

    lib = None

    # Try to load in the normal path
    try:
        lib = cdll.LoadLibrary(libfile)
        # Otherwise, library should have been copied to the lib/ folder
    except:
        lib = cdll.LoadLibrary(str(pathlib.Path(__file__).parent / "lib" / libfile))

    return lib


def initialize_functions(lib: CDLL) -> None:
    """Initializes the Go shared library functions

    :param lib: CDLL: The Go shared library

    :meta private:
    """
    # Exposed functions
    # We have to use c_void_p instead of c_char_p to free it properly
    # See https://stackoverflow.com/questions/13445568/python-ctypes-how-to-free-memory-getting-invalid-pointer-error
    lib.Deregister.argtypes, lib.Deregister.restype = [], None
    lib.ExpiryTimes.argtypes, lib.ExpiryTimes.restype = [], DataError
    lib.FreeString.argtypes, lib.FreeString.restype = [c_void_p], None
    lib.DiscoOrganizations.argtypes, lib.DiscoOrganizations.restype = [c_int], DataError
    lib.DiscoServers.argtypes, lib.DiscoServers.restype = [c_int], DataError
    lib.GetConfig.argtypes, lib.GetConfig.restype = [
        c_int,
        c_int,
        c_char_p,
        c_int,
    ], DataError
    lib.AddServer.argtypes, lib.AddServer.restype = [
        c_int,
        c_int,
        c_char_p,
        c_int,
    ], c_char_p
    lib.CurrentServer.argtypes, lib.CurrentServer.restype = [], DataError
    lib.RemoveServer.argtypes, lib.RemoveServer.restype = [
        c_int,
        c_char_p,
    ], c_char_p
    lib.ServerList.argtypes, lib.ServerList.restype = [], DataError
    lib.Register.argtypes, lib.Register.restype = [
        c_char_p,
        c_char_p,
        c_char_p,
        VPNStateChange,
        c_int,
    ], c_void_p
    lib.RenewSession.argtypes, lib.RenewSession.restype = [c_int], c_void_p
    lib.SetTokenHandler.argtypes, lib.SetTokenHandler.restype = [
        TokenGetter,
        TokenSetter,
    ], c_void_p
    lib.Cleanup.argtypes, lib.Cleanup.restype = [c_int], c_void_p
    lib.SetProfileID.argtypes, lib.SetProfileID.restype = [c_char_p], c_void_p
    lib.CookieNew.argtypes, lib.CookieNew.restype = [], c_int
    lib.CookieReply.argtypes, lib.CookieReply.restype = [c_int, c_char_p], c_void_p
    lib.CookieCancel.argtypes, lib.CookieCancel.restype = [c_int], c_void_p
    lib.CookieDelete.argtypes, lib.CookieDelete.restype = [c_int], c_void_p
    lib.SetSecureLocation.argtypes, lib.SetSecureLocation.restype = [
        c_int,
        c_char_p,
    ], c_void_p
    lib.SetSupportWireguard.argtypes, lib.SetSupportWireguard.restype = [
        c_int,
    ], c_void_p
    lib.SetState.argtypes, lib.SetState.restype = [
        c_int,
    ], c_void_p
    lib.InState.argtypes, lib.InState.restype = [
        c_int,
    ], BoolError
    lib.StartFailover.argtypes, lib.StartFailover.restype = [
        c_int,
        c_char_p,
        c_int,
        ReadRxBytes,
    ], BoolError
