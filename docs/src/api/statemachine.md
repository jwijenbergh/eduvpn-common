# Finite state machine

The eduvpn-common library uses a finite state machine internally to keep track of which state the client is in and to communicate data callbacks (e.g. to communicate the Authorization URL in the OAuth process to the client).

## Viewing the FSM
To view the FSM in an image, register to the library with in debug mode. This
outputs the graph with a `.graph` extension in the client-specified
config directory. The format of this
graph is from [Mermaid](https://mermaid-js.github.io/mermaid/#/). You
can convert this to an image using the [Mermaid command-line client](https://github.com/mermaid-js/mermaid-cli) installed or from the Mermaid web site, the [Mermaid Live Editor](https://mermaid.live)

## FSM example
The following is an example of the FSM when the client has obtained a Wireguard/OpenVPN configuration from an eduVPN server

<div class="statemachine">

```mermaid
graph TD

style Deregistered fill:white
Deregistered(Deregistered) -->|Client registers| No_Server

style No_Server fill:white
No_Server(No_Server) -->|User clicks a server in the UI| Loading_Server

style Ask_Location fill:white
Ask_Location(Ask_Location) -->|Location chosen| Chosen_Location

style Ask_Location fill:white
Ask_Location(Ask_Location) -->|Go back or Error| No_Server

style Chosen_Location fill:white
Chosen_Location(Chosen_Location) -->|Server has been chosen| Chosen_Server

style Chosen_Location fill:white
Chosen_Location(Chosen_Location) -->|Go back or Error| No_Server

style Loading_Server fill:white
Loading_Server(Loading_Server) -->|Server info loaded| Chosen_Server

style Loading_Server fill:white
Loading_Server(Loading_Server) -->|User chooses a Secure Internet server but no location is configured| Ask_Location

style Loading_Server fill:white
Loading_Server(Loading_Server) -->|Go back or Error| No_Server

style Chosen_Server fill:white
Chosen_Server(Chosen_Server) -->|Found tokens in config| Authorized

style Chosen_Server fill:white
Chosen_Server(Chosen_Server) -->|No tokens found in config| OAuth_Started

style OAuth_Started fill:white
OAuth_Started(OAuth_Started) -->|User authorizes with browser| Authorized

style OAuth_Started fill:white
OAuth_Started(OAuth_Started) -->|Go back or Error| No_Server

style Authorized fill:white
Authorized(Authorized) -->|Re-authorize with OAuth| OAuth_Started

style Authorized fill:white
Authorized(Authorized) -->|Client requests a config| Request_Config

style Authorized fill:white
Authorized(Authorized) -->|Client wants to go back to the main screen| No_Server

style Request_Config fill:white
Request_Config(Request_Config) -->|Multiple profiles found and no profile chosen| Ask_Profile

style Request_Config fill:white
Request_Config(Request_Config) -->|Only one profile or profile already chosen| Chosen_Profile

style Request_Config fill:white
Request_Config(Request_Config) -->|Cancel or Error| No_Server

style Request_Config fill:white
Request_Config(Request_Config) -->|Re-authorize| OAuth_Started

style Ask_Profile fill:white
Ask_Profile(Ask_Profile) -->|Cancel or Error| No_Server

style Ask_Profile fill:white
Ask_Profile(Ask_Profile) -->|Profile has been chosen| Chosen_Profile

style Chosen_Profile fill:white
Chosen_Profile(Chosen_Profile) -->|Cancel or Error| No_Server

style Chosen_Profile fill:white
Chosen_Profile(Chosen_Profile) -->|Config has been obtained| Got_Config

style Got_Config fill:cyan
Got_Config(Got_Config) -->|Choose a new server| No_Server

style Got_Config fill:cyan
Got_Config(Got_Config) -->|Get a new configuration| Loading_Server
```

</div>

The current state is highlighted in the <span style="color:cyan">cyan</span> color.

## State explanation

For the explanation of what all the different states mean, see the [client documentation](https://github.com/eduvpn/eduvpn-common/blob/v2/client/fsm.go#L14-L50)

## States that ask data

In eduvpn-common, there are certain states that require attention from the client.

- OAuth Started: A state that must be handled by the client. How a client can 'handle' this state, we will see in the next section. In this state, the client must open the webbrowser with the authorization URL to complete to OAuth process
- Ask Profile: The state that asks for a profile selection to the client. Reply to this state by using a "cookie" and the CookieReply function. What this means will be discussed in the Python client example too
- Ask Location: Same for ask profile but for selecting a secure internet location. Only called if one must be chosen, e.g. due to a selection that is no longer valid

The rest of the states are miscellaneous states, meaning that the client can handle them however it wants to. However, it can be useful to handle most state transitions to e.g. show loading screens or for logging and debugging purposes.
