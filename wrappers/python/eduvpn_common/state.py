from enum import IntEnum


class StateType(IntEnum):
    """
    The State Type enum.
    """

    ENTER = 1
    LEAVE = 2


StateEnum = IntEnum


class State(StateEnum):
    # Go states
    INITIAL = 0
    MAIN = 1
    ASK_LOCATION = 2
    CHOSEN_LOCATION = 3
    LOADING_SERVER = 4
    CHOSEN_SERVER = 5
    OAUTH_STARTED = 6
    AUTHORIZED = 7
    REQUEST_CONFIG = 8
    ASK_PROFILE = 9
    CHOSEN_PROFILE = 10
    GOT_CONFIG = 11
    CONNECTING = 12
    DISCONNECTING = 13
    CONNECTED = 14
