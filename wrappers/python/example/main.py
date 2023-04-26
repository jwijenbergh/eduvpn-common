import eduvpn_common.main as eduvpn
from eduvpn_common.state import State, StateType
from eduvpn_common.server import Config, Profiles
from typing import Optional, List, Tuple
import webbrowser
import sys


# Asks the user for a profile index
# It loops up until a valid input is given
def ask_ranged_input(total: int, label: str) -> int:
    range_index = None

    while range_index is None:
        try:
            range_index = int(
                input(f"Please select a {label} by inputting a number (e.g. 1): ")
            )
            if (range_index > total) or (range_index < 1):
                print(f"Invalid input, input must be between 1 and {total} (inclusive)")
                range_index = None
        except ValueError:
            print("Please enter a valid input")

    # The profile is one based, move to zero based input
    return range_index - 1


def setup_callbacks(edu: eduvpn.EduVPN):
    @edu.event.on(State.OAUTH_STARTED, StateType.ENTER)
    def enter_oauth(old_state: State, url: str):
        print("OAuth started", url)
        webbrowser.open(url)

    @edu.event.on(State.ASK_PROFILE, StateType.ENTER)
    def enter_ask_profile(old_state: State, profiles: Profiles):
        print("This server has multiple available profiles")
        for index, profile in enumerate(profiles.profiles):
            print(f"[{index+1}]: {profile}")
        index = ask_ranged_input(len(profiles.profiles), "profile")
        _eduvpn.set_profile(profiles.profiles[index].identifier)

    @edu.event.on(State.ASK_LOCATION, StateType.ENTER)
    def enter_ask_location(old_state: State, locations: List[str]):
        print("This server has multiple available locations")
        for index, location in enumerate(locations):
            print(f"[{index+1}]: {location}")
        index = ask_ranged_input(len(locations), "location")
        _eduvpn.set_secure_location(locations[index])


def do_custom_server(edu: eduvpn.EduVPN) -> Optional[Config]:
    server_url = input(
        "Enter a server URL to get a configuration for (e.g. vpn.example.com): "
    )
    edu.add_custom_server(server_url)
    return edu.get_config_custom_server(server_url)


def do_institute_access(edu: eduvpn.EduVPN) -> Optional[Config]:
    print("Please choose an institute access server:")
    disco_servers = edu.get_disco_servers()
    if not disco_servers:
        raise Exception("No discovery servers found")
    index = 0
    institute_servers = []
    for disco_server in sorted(disco_servers.servers, key=lambda x: str(x)):
        if disco_server.server_type == "institute_access":
            print(f"[{index+1}]: {disco_server}")
            institute_servers.append(disco_server)
            index += 1

    ranged_index = ask_ranged_input(len(institute_servers), "institute access server")
    base_url = institute_servers[ranged_index].base_url
    edu.add_institute_access(base_url)
    return edu.get_config_institute_access(base_url)


def do_secure_internet(edu: eduvpn.EduVPN) -> Optional[Config]:
    print("Please choose a secure internet server:")
    disco_orgs = edu.get_disco_organizations()
    if not disco_orgs:
        raise Exception("No discovery organizations found")
    index = 0
    secure_internet_servers = []
    for disco_org in sorted(disco_orgs.organizations, key=lambda x: str(x)):
        print(f"[{index+1}]: {disco_org}")
        secure_internet_servers.append(disco_org)
        index += 1

    ranged_index = ask_ranged_input(
        len(secure_internet_servers), "secure internet server"
    )
    org_id = secure_internet_servers[ranged_index].org_id
    edu.add_secure_internet_home(org_id)
    return edu.get_config_secure_internet(org_id)


# The main entry point
if __name__ == "__main__":
    _eduvpn = eduvpn.EduVPN("org.eduvpn.app.linux", "2.0.0-cli-py", "configs", "en")
    setup_callbacks(_eduvpn)

    # Register with the eduVPN-common library
    try:
        _eduvpn.register()
    except Exception as e:
        print("Failed registering:", e, file=sys.stderr)
        sys.exit(1)

    input_dict = {
        "c": do_custom_server,
        "i": do_institute_access,
        "s": do_secure_internet,
    }

    type_input = input(
        "Which type of server do you want to connect to, choose one of (c)ustom/(i)nstitute access/(s)ecure internet): "
    )
    to_call = input_dict.get(type_input)
    if to_call is None:
        print("Invalid type chosen", file=sys.stderr)
        sys.exit(1)

    try:
        config = to_call(_eduvpn)
    except Exception as e:
        print("Failed getting a config:", e, file=sys.stderr)
        sys.exit(1)

    if config is None:
        print("Failed getting a config: no configuration returned")
        sys.exit(1)
    print("Got a config:\n", config.config)
    print("The config is of type:", config.config_type)

    # Save and exit
    _eduvpn.deregister()
