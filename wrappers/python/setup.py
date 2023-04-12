#!/usr/bin/env python3

import os
import shutil
import sys
import typing
from collections import defaultdict

from setuptools import setup
from wheel.bdist_wheel import bdist_wheel as _bdist_wheel

_libname = "eduvpn_common"
__version__ = "1.1.0"


def getlibpath(plat_name: str) -> typing.Union[str, None]:
    """Get library path for plat_name relative to exports/lib/ folder."""

    plat_map = defaultdict(
        lambda: plat_name,
        {
            "win32": "win-x86",
        },
    )

    plat_split = plat_map[plat_name].split("-", 1)
    if len(plat_split) != 2:
        return None
    plat_os, plat_arch = plat_split

    os_map = defaultdict(
        lambda: plat_os,
        {
            "win": "windows",
        },
    )
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
    arch_map = defaultdict(
        lambda: plat_arch,
        {
            "aarch64_be": "arm64",
            "aarch64": "arm64",
            "armv8b": "arm64",
            "armv8l": "arm64",
            "x86": "386",
            "x86pc": "386",
            "i86pc": "386",
            "i386": "386",
            "i686": "386",
            "x86_64": "amd64",
            "i686-64": "amd64",
        },
    )

    processed_os = os_map[plat_os]
    return (
        processed_os
        + "/"
        + arch_map[plat_arch]
        + "/"
        + lib_prefixes[processed_os]
        + _libname
        + "-"
        + __version__
        + lib_suffixes[processed_os]
    )


# Adapted from https://stackoverflow.com/a/51794740
# You would say there would be a better way to do all of this, but I couldn't find it


class bdist_wheel(_bdist_wheel):
    user_options = _bdist_wheel.user_options + [
        ("exports-lib-path=", None, "path to exports/lib directory"),
    ]

    def initialize_options(self):
        super().initialize_options()
        self.exports_lib_path = "../../exports/lib"  # default

    def run(self):
        self.plat_name_supplied = True  # Force use platform

        libpath = getlibpath(self.plat_name)
        if not libpath:
            print("Unknown platform:", self.plat_name)
            sys.exit(1)

        print("Building wheel for platform:", self.plat_name)

        # setuptools will only use paths inside the package for package_data, so we copy the library
        tmp_lib = shutil.copy(
            self.exports_lib_path + "/" + libpath, "eduvpn_common/lib/"
        )
        _bdist_wheel.run(self)
        os.remove(tmp_lib)


setup(
    name="eduvpn_common",
    version=__version__,
    packages=["eduvpn_common"],
    python_requires=">=3.6",
    package_dir={"eduvpn_common": "eduvpn_common"},
    package_data={"eduvpn_common": ["lib/*" + _libname + "*", "py.typed"]},
    cmdclass={"bdist_wheel": bdist_wheel},
)
