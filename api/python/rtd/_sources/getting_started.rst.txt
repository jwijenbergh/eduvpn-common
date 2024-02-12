==========================================
Getting Started
==========================================

.. contents::

1 Introduction
--------------

This documentation page describes the API for the python-eduvpn-common library. Eduvpn-common is a library to write eduVPN clients in. The library is written in Go and this documentation describes the Python glue code that interacts with this library. We first give a general overview and then dive deep into the various components. We recommend, however, to first read the `eduvpn-common documentation <https://eduvpn.github.io/eduvpn-common>`_ first to get an overview of how the library works and which problems it solves.

2 Overview
----------

This library interfaces with the Go library by loading the library as a shared C library (e.g. *.so* file for Linux) and then defining *ctypes*. This means that the library and the glue code needs to be in sync. When you install this library with a package manager or pip, we bundle this shared library with it so that the exact version matches. Note that you can also compile this shared library from source by following instructions at `the official documentation <https://github.com/eduvpn/eduvpn-common>`_.

There are various modules that this library defines, some are meant to be extensively used by the resulting eduVPN client, while others are purely meant for internal use. We give a general overview for each:

- *main*: This is the main entry point for the glue code. It defines a class *EduVPN* that is used to interact with the library

- *loader*: This loads the shared library and defines the functions that this shared library has

- *types*: This defines types that are returned by the library. For example, the type of an Institute Access server

- *server*: This converts the type returned by the library to a type that can be consumed by Python. You would then import these Python types from this module

- *discovery*: The same as the server module, but then for the `Discovery phase <https://github.com/eduvpn/documentation/tree/master/SERVER_DISCOVERY.md>`_

- *state*: This defines the states as used by the eduvpn-common library. You would import these to define state transitions on these states

- *event*: This defines the *EventHandler* class that is internally used to handle the FSM events/transitions

- *error*: This defines error types that are returned by the library. You would use this to import the *WrappedError* type that returns an exception from the Go library

3 Step by step simple example
-----------------------------

The first thing is to bring the library into scope

.. code:: python

    from eduvpn_common import main

We can now create the main *EduVPN* class to interface with the library

.. code:: python

    # The name of the client, use one of https://git.sr.ht/~fkooman/vpn-user-portal/tree/v3/item/src/OAuth/ClientDb.php
    # We specify the linux client
    # You can also specify a Let's Connect! variant
    client_name = "org.eduvpn.app.linux"

    # The absolute or relative directory where the configs should be stored
    config_dir = "configs"

    # The language of the client, in practice you should not use such a hard coded value
    # But rather specify one as returned by Python internationalization's modules
    language = "en"

    # Create the class
    eduvpn = main.EduVPN(client_name, config_dir, language)

This code has not yet actually done anything when it comes to interfacing with the library. For that we have the *register* method on the class

.. code:: python

    # We register as debug, meaning we want debug logging. When using for release, set this to 'False'
    eduvpn.register(debug=True)

Great! We have registered our client with the Go library and can now start to do something more interesting. Let's try to get an OpenVPN/WireGuard VPN configuration from our own server.

First we define the URL of the server we want to get a configuration for

.. code:: python

    server_url = "vpn.example.com"

Then we need to define a few state handlers that specify what the library should do when the OAuth process has started. We do that with the *.event.on* handler for the *EduVPN* class:

.. code:: python

    # Necessary imports to act on states
    from eduvpn_common.state import State, StateType

    # Enter means, run this when the Go FSM signals we have entered the OAUTH_STARTED state
    @eduvpn.event.on(State.OAUTH_STARTED, StateType.ENTER)
    def enter_oauth(old_state: State, url: str):
        print(f"Please open the browser at URL: {url}")

If our server does not use multiple profiles, we can then get the OpenVPN/WireGuard configuration as follows

.. code:: python

    # First add the server
    eduvpn.add_custom_server(server_url)

    # Then get the config
    # prefer_tcp set to False means we do not care whether or not TCP is preferred by the client
    # You can set this to True so that an OpenVPN configuration with TCP is preferred
    # Note that this can return an exception, so you should handle this
    eduvpn.get_config_custom_server(server_url, prefer_tcp=False)

Now for the final step, let's try connect to a server that has multiple profiles defined. For this, we need another callback using the *event.on* decorator

.. code:: python

    from eduvpn_common.server import Profiles

    @eduvpn.event.on(State.ASK_PROFILE, StateType.ENTER)
    def enter_ask_profile(old_state: State, profiles: Profiles):
        # We choose the first profile
        eduvpn.set_profile(profiles.profiles[0].identifier)

    server_multiple_profiles = "vpn-multiple-profiles.example.com"

    eduvpn.add_custom_server(server_multiple_profiles)
    eduvpn.get_config_custom_server(server_multiple_profiles, prefer_tcp=False)

In practice, you should define these callbacks on every state transition (at least *ENTER* transitions) such that every case is handled. For example, there are also mandatory callbacks when asking for a location to connect to in case of secure internet.

A more elaborate example of the library can be found at `the GitHub repository <https://github.com/eduvpn/eduvpn-common/tree/main/wrappers/python/main.py>`_.
Or, a full featured example can be found by looking at `the official Linux client <https://github.com/eduvpn/python-eduvpn-client>`_.

4 API Documentation
-------------------

The detailed API documentation is available on the next page.
