#!/usr/bin/env python3

import unittest
import eduvpn_common.main as eduvpn
import sys
import os

# Import project root directory where the selenium python utility is
sys.path.append(
    os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
)

from selenium_eduvpn import login_eduvpn

def handler(_old_state, new_state, data):
    if new_state == 6:
        login_eduvpn(data)
        return True

class ConfigTests(unittest.TestCase):
    def testConfig(self):
        _eduvpn = eduvpn.EduVPN("org.letsconnect-vpn.app.linux", "0.1.0", "testconfigs")
        # This can throw an exception
        _eduvpn.register(handler=handler)

        server_uri = os.getenv("SERVER_URI")
        if not server_uri:
            print("No SERVER_URI environment variable given, skipping...")
            _eduvpn.deregister()
            return

        # This can throw an exception
        _eduvpn.add_server(eduvpn.ServerType.CUSTOM, server_uri)
        _eduvpn.get_config(eduvpn.ServerType.CUSTOM, server_uri)

        # Deregister
        _eduvpn.deregister()

    def testDoubleRegister(self):
        _eduvpn = eduvpn.EduVPN("org.letsconnect-vpn.app.linux", "0.1.0", "testconfigs")
        # This can throw an exception
        _eduvpn.register()
        # This should throw
        try:
            _eduvpn.register()
        except Exception as e:
            return
        self.fail("No exception thrown on second register")


if __name__ == "__main__":
    unittest.main()
