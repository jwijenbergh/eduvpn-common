package eduvpn

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	httpw "github.com/jwijenbergh/eduvpn-common/internal/http"
	"github.com/jwijenbergh/eduvpn-common/internal/oauth"
	"github.com/jwijenbergh/eduvpn-common/internal/server"
	"github.com/jwijenbergh/eduvpn-common/internal/types"
)

func ensureLocalWellKnown() {
	wellKnown := os.Getenv("SERVER_IS_LOCAL")

	if wellKnown == "1" {
		server.WellKnownPath = "well-known.php"
	}
}

func getServerURI(t *testing.T) string {
	serverURI := os.Getenv("SERVER_URI")
	if serverURI == "" {
		t.Skip("Skipping server test as no SERVER_URI env var has been passed")
	}
	return serverURI
}

func runCommand(t *testing.T, errBuffer *strings.Builder, name string, args ...string) error {
	cmd := exec.Command(name, args...)

	cmd.Stderr = errBuffer
	err := cmd.Start()
	if err != nil {
		return err
	}

	return cmd.Wait()
}

func loginOAuthSelenium(t *testing.T, url string, state *VPNState) {
	// We could use the go selenium library
	// But it does not support the latest selenium v4 just yet
	var errBuffer strings.Builder
	err := runCommand(t, &errBuffer, "python3", "selenium_eduvpn.py", url)
	if err != nil {
		t.Fatalf(
			"Login OAuth with selenium script failed with error %v and stderr %s",
			err,
			errBuffer.String(),
		)
		_ = state.CancelOAuth()
	}
}

func stateCallback(
	t *testing.T,
	oldState FSMStateID,
	newState FSMStateID,
	data interface{},
	state *VPNState,
) {
	if newState == STATE_OAUTH_STARTED {
		url, ok := data.(string)

		if !ok {
			t.Fatalf("data  is not a string for OAuth URL")
		}
		loginOAuthSelenium(t, url, state)
	}
}

func Test_server(t *testing.T) {
	serverURI := getServerURI(t)
	state := &VPNState{}
	ensureLocalWellKnown()

	registerErr := state.Register(
		"org.eduvpn.app.linux",
		"configstest",
		func(old FSMStateID, new FSMStateID, data interface{}) {
			stateCallback(t, old, new, data, state)
		},
		false,
	)
	if registerErr != nil {
		t.Fatalf("Register error: %v", registerErr)
	}

	_, _, configErr := state.GetConfigCustomServer(serverURI, false)
	if configErr != nil {
		t.Fatalf("Connect error: %v", configErr)
	}
}

func test_connect_oauth_parameter(
	t *testing.T,
	parameters httpw.URLParameters,
	expectedErr interface{},
) {
	serverURI := getServerURI(t)
	state := &VPNState{}
	configDirectory := "test_oauth_parameters"

	registerErr := state.Register(
		"org.eduvpn.app.linux",
		configDirectory,
		func(oldState FSMStateID, newState FSMStateID, data interface{}) {
			if newState == STATE_OAUTH_STARTED {
				baseURL := "http://127.0.0.1:8000/callback"
				url, err := httpw.HTTPConstructURL(baseURL, parameters)
				if err != nil {
					t.Fatalf(
						"Error: Constructing url %s with parameters %s",
						baseURL,
						fmt.Sprint(parameters),
					)
				}
				go func() {
					_, getErr := http.Get(url)
					if getErr != nil {
						t.Logf("HTTP GET error: %v", getErr)
					}
				}()
			}
		},
		false,
	)
	if registerErr != nil {
		t.Fatalf("Register error: %v", registerErr)
	}
	_, _, configErr := state.GetConfigCustomServer(serverURI, false)

	var wrappedErr *types.WrappedErrorMessage

	// We ensure the error is of a wrappedErrorMessage
	if !errors.As(configErr, &wrappedErr) {
		t.Fatalf("error %T = %v, wantErr %T", configErr, configErr, wrappedErr)
	}

	gotExpectedErr := wrappedErr.Cause()

	// Then we check if the cause is correct
	if !errors.As(gotExpectedErr, expectedErr) {
		t.Fatalf("error %T = %v, wantErr %T", gotExpectedErr, gotExpectedErr, expectedErr)
	}
}

func Test_connect_oauth_parameters(t *testing.T) {
	var (
		failedCallbackParameterError  *oauth.OAuthCallbackParameterError
		failedCallbackStateMatchError *oauth.OAuthCallbackStateMatchError
	)

	tests := []struct {
		expectedErr interface{}
		parameters  httpw.URLParameters
	}{
		{&failedCallbackParameterError, httpw.URLParameters{}},
		{&failedCallbackParameterError, httpw.URLParameters{"code": "42"}},
		{&failedCallbackStateMatchError, httpw.URLParameters{"code": "42", "state": "21"}},
	}

	ensureLocalWellKnown()

	for _, test := range tests {
		test_connect_oauth_parameter(t, test.parameters, test.expectedErr)
	}
}

func Test_token_expired(t *testing.T) {
	serverURI := getServerURI(t)
	expiredTTL := os.Getenv("OAUTH_EXPIRED_TTL")
	if expiredTTL == "" {
		t.Log(
			"No expired TTL present, skipping this test. Set OAUTH_EXPIRED_TTL env variable to run this test",
		)
		return
	}

	ensureLocalWellKnown()

	// Convert the env variable to an int and signal error if it is not possible
	expiredInt, expiredErr := strconv.Atoi(expiredTTL)
	if expiredErr != nil {
		t.Fatalf("Cannot convert EXPIRED_TTL env variable to an int with error %v", expiredErr)
	}

	// Get a vpn state
	state := &VPNState{}

	registerErr := state.Register(
		"org.eduvpn.app.linux",
		"configsexpired",
		func(old FSMStateID, new FSMStateID, data interface{}) {
			stateCallback(t, old, new, data, state)
		},
		false,
	)
	if registerErr != nil {
		t.Fatalf("Register error: %v", registerErr)
	}

	_, _, configErr := state.GetConfigCustomServer(serverURI, false)

	if configErr != nil {
		t.Fatalf("Connect error before expired: %v", configErr)
	}

	currentServer, serverErr := state.Servers.GetCurrentServer()
	if serverErr != nil {
		t.Fatalf("No server found")
	}

	oauth := currentServer.GetOAuth()

	accessToken := oauth.Token.Access
	refreshToken := oauth.Token.Refresh

	// Wait for TTL so that the tokens expire
	time.Sleep(time.Duration(expiredInt) * time.Second)

	_, _, configErr = state.GetConfigCustomServer(serverURI, false)

	if configErr != nil {
		t.Fatalf("Connect error after expiry: %v", configErr)
	}

	// Check if tokens have changed
	accessTokenAfter := oauth.Token.Access
	refreshTokenAfter := oauth.Token.Refresh

	if accessToken == accessTokenAfter {
		t.Errorf("Access token is the same after refresh")
	}

	if refreshToken == refreshTokenAfter {
		t.Errorf("Refresh token is the same after refresh")
	}
}

func Test_token_invalid(t *testing.T) {
	serverURI := getServerURI(t)
	state := &VPNState{}

	ensureLocalWellKnown()

	registerErr := state.Register(
		"org.eduvpn.app.linux",
		"configsinvalid",
		func(old FSMStateID, new FSMStateID, data interface{}) {
			stateCallback(t, old, new, data, state)
		},
		false,
	)
	if registerErr != nil {
		t.Fatalf("Register error: %v", registerErr)
	}

	_, _, configErr := state.GetConfigCustomServer(serverURI, false)

	if configErr != nil {
		t.Fatalf("Connect error before invalid: %v", configErr)
	}

	dummy_value := "37"

	currentServer, serverErr := state.Servers.GetCurrentServer()
	if serverErr != nil {
		t.Fatalf("No server found")
	}

	oauth := currentServer.GetOAuth()

	// Override tokens with invalid values
	oauth.Token.Access = dummy_value
	oauth.Token.Refresh = dummy_value

	_, _, configErr = state.GetConfigCustomServer(serverURI, false)

	if configErr != nil {
		t.Fatalf("Connect error after invalid: %v", configErr)
	}

	if oauth.Token.Access == dummy_value {
		t.Errorf("Access token is equal to dummy value: %s", dummy_value)
	}

	if oauth.Token.Refresh == dummy_value {
		t.Errorf("Refresh token is equal to dummy value: %s", dummy_value)
	}
}

// Test if an invalid profile will be corrected
func Test_invalid_profile_corrected(t *testing.T) {
	serverURI := getServerURI(t)
	state := &VPNState{}

	ensureLocalWellKnown()

	registerErr := state.Register(
		"org.eduvpn.app.linux",
		"configscancelprofile",
		func(old FSMStateID, new FSMStateID, data interface{}) {
			stateCallback(t, old, new, data, state)
		},
		false,
	)
	if registerErr != nil {
		t.Fatalf("Register error: %v", registerErr)
	}

	_, _, configErr := state.GetConfigCustomServer(serverURI, false)

	if configErr != nil {
		t.Fatalf("First connect error: %v", configErr)
	}

	currentServer, serverErr := state.Servers.GetCurrentServer()
	if serverErr != nil {
		t.Fatalf("No server found")
	}

	base, baseErr := currentServer.GetBase()
	if baseErr != nil {
		t.Fatalf("No base found")
	}

	previousProfile := base.Profiles.Current
	base.Profiles.Current = "IDONOTEXIST"

	_, _, configErr = state.GetConfigCustomServer(serverURI, false)

	if configErr != nil {
		t.Fatalf("Second connect error: %v", configErr)
	}

	if base.Profiles.Current != previousProfile {
		t.Fatalf(
			"Profiles do no match: current %s and previous %s",
			base.Profiles.Current,
			previousProfile,
		)
	}
}
