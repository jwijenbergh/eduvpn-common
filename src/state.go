package eduvpn

type VPNState struct {
	// Info passed by the client
	ConfigDirectory string                       `json:"-"`
	Name            string                       `json:"-"`
	StateCallback   func(string, string, string) `json:"-"`

	// The chosen server
	Server *Server `json:"server"`

	// The list of servers and organizations from disco
	DiscoList *DiscoList `json:"disco"`
}

func (state *VPNState) Register(name string, directory string, stateCallback func(string, string, string)) error {
	state.Name = name
	state.ConfigDirectory = directory
	state.StateCallback = stateCallback

	state.StateCallback("Start", "Registered", "app registered")

	// Try to load the previous configuration
	if state.LoadConfig() != nil {
		// This error can be safely ignored, as when the config does not load, the struct will not be filled
		// Make sure to log this when we have implemented a good logging system
	}
	return nil
}

func (state *VPNState) Connect(url string) (string, error) {
	if state.Server == nil {
		state.Server = &Server{}
	}
	initializeErr := state.Server.Initialize(url)

	if initializeErr != nil {
		return "", initializeErr
	}

	if !state.Server.IsAuthenticated() {
		authURL, authInitializeErr := state.InitializeOAuth()

		if authInitializeErr != nil {
			return "", authInitializeErr
		}

		go state.StateCallback("Registered", "OAuthInitialized", authURL)
		oauthErr := state.FinishOAuth()

		if oauthErr != nil {
			return "", oauthErr
		}

		state.StateCallback("OAuthInitialized", "OAuthFinished", "finished oauth")
		state.WriteConfig()
	}

	return state.Server.GetConfig()
}

var VPNStateInstance *VPNState

func GetVPNState() *VPNState {
	if VPNStateInstance == nil {
		VPNStateInstance = &VPNState{}
	}
	return VPNStateInstance
}
