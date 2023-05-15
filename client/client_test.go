package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	httpw "github.com/eduvpn/eduvpn-common/internal/http"
	"github.com/eduvpn/eduvpn-common/types/cookie"
	"github.com/eduvpn/eduvpn-common/types/protocol"
	srvtypes "github.com/eduvpn/eduvpn-common/types/server"
	"github.com/go-errors/errors"
)

func getServerURI(t *testing.T) string {
	serverURI := os.Getenv("SERVER_URI")
	if serverURI == "" {
		t.Skip("Skipping server test as no SERVER_URI env var has been passed")
	}
	serverURI, parseErr := httpw.EnsureValidURL(serverURI, true)
	if parseErr != nil {
		t.Skip("Skipping server test as the server uri is not valid")
	}
	return serverURI
}

func runCommand(errBuffer *strings.Builder, name string, args ...string) error {
	cmd := exec.Command(name, args...)

	cmd.Stderr = errBuffer
	err := cmd.Start()
	if err != nil {
		return err
	}

	return cmd.Wait()
}

func loginOAuthSelenium(ck *cookie.Cookie, url string) {
	// We could use the go selenium library
	// But it does not support the latest selenium v4 just yet
	var errBuffer strings.Builder
	err := runCommand(&errBuffer, "python3", "../selenium_eduvpn.py", url)
	if err != nil {
		_ = ck.Cancel()
		panic(fmt.Sprintf(
			"Login OAuth with selenium script failed with error %v and stderr %s",
			err,
			errBuffer.String(),
		))
	}
}

func stateCallback(
	t *testing.T,
	ck *cookie.Cookie,
	_ FSMStateID,
	newState FSMStateID,
	data interface{},
) {
	if newState == StateOAuthStarted {
		url, ok := data.(string)

		if !ok {
			t.Fatalf("data is not a string for OAuth URL")
		}
		loginOAuthSelenium(ck, url)
	}
}

func TestServer(t *testing.T) {
	serverURI := getServerURI(t)
	ck := cookie.NewWithContext(context.Background())
	defer ck.Cancel() //nolint:errcheck
	state, err := New(
		"org.letsconnect-vpn.app.linux",
		"0.1.0-test",
		"configstest",
		func(old FSMStateID, new FSMStateID, data interface{}) bool {
			stateCallback(t, &ck, old, new, data)
			return true
		},
		false,
	)
	if err != nil {
		t.Fatalf("Creating client error: %v", err)
	}
	err = state.Register()
	if err != nil {
		t.Fatalf("Registering error: %v", err)
	}

	addErr := state.AddServer(&ck, serverURI, srvtypes.TypeCustom, false)
	if addErr != nil {
		t.Fatalf("Add error: %v", addErr)
	}
	_, configErr := state.GetConfig(&ck, serverURI, srvtypes.TypeCustom, false)
	if configErr != nil {
		t.Fatalf("Connect error: %v", configErr)
	}
}

func testConnectOAuthParameter(
	t *testing.T,
	parameters httpw.URLParameters,
	errPrefix string,
) {
	serverURI := getServerURI(t)
	configDirectory := "test_oauth_parameters"

	state := &Client{}

	ck := cookie.NewWithContext(context.Background())
	defer ck.Cancel() //nolint:errcheck
	state, err := New(
		"org.letsconnect-vpn.app.linux",
		"0.1.0-test",
		configDirectory,
		func(oldState FSMStateID, newState FSMStateID, data interface{}) bool {
			if newState == StateOAuthStarted {
				server, serverErr := state.Servers.CustomServer(serverURI)
				if serverErr != nil {
					t.Fatalf("No server with error: %v", serverErr)
				}
				port, portErr := server.OAuth().ListenerPort()
				if portErr != nil {
					_ = ck.Cancel()
					t.Fatalf("No port with error: %v", portErr)
				}
				baseURL := fmt.Sprintf("http://127.0.0.1:%d/callback", port)
				p, err := url.Parse(baseURL)
				if err != nil {
					_ = ck.Cancel()
					t.Fatalf("Failed to parse URL with error: %v", err)
				}
				url, err := httpw.ConstructURL(p, parameters)
				if err != nil {
					_ = ck.Cancel()
					t.Fatalf(
						"Error: Constructing url %s with parameters %s",
						baseURL,
						fmt.Sprint(parameters),
					)
				}
				go func() {
					_, getErr := http.Get(url)
					if getErr != nil {
						_ = ck.Cancel()
						t.Logf("HTTP GET error: %v", getErr)
					}
				}()
			}
			return true
		},
		false,
	)
	if err != nil {
		t.Fatalf("Creating client error: %v", err)
	}
	err = state.Register()
	if err != nil {
		t.Fatalf("Registering error: %v", err)
	}

	err = state.AddServer(&ck, serverURI, srvtypes.TypeCustom, false)

	if errPrefix == "" {
		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}
		return
	}

	if err == nil {
		t.Fatalf("expected error with prefix '%s' but got nil", errPrefix)
	}

	err1, ok := err.(*errors.Error)
	// We ensure the error is of a wrappedErrorMessage
	if !ok {
		t.Fatalf("error %T = %v, wantErr %T", err, err, &errors.Error{})
	}

	msg := err1.Error()
	if err1.Err != nil {
		msg = err1.Err.Error()
	}

	// Then we check if the cause is correct
	if !strings.HasPrefix(msg, errPrefix) {
		t.Fatalf("expected error with prefix '%s' but got '%s'", errPrefix, msg)
	}
}

func TestConnectOAuthParameters(t *testing.T) {
	const (
		callbackParameterErrorPrefix  = "failed retrieving parameter '"
		callbackStateMatchErrorPrefix = "failed matching state"
		callbackISSMatchErrorPrefix   = "failed matching ISS"
	)

	serverURI := getServerURI(t)
	// serverURI already ends with a / due to using the util EnsureValidURL function
	iss := serverURI
	tests := []struct {
		errPrefix  string
		parameters httpw.URLParameters
	}{
		// missing state and code
		{callbackParameterErrorPrefix, httpw.URLParameters{"iss": iss}},
		// missing state
		{callbackParameterErrorPrefix, httpw.URLParameters{"iss": iss, "code": "42"}},
		// invalid state
		{
			callbackStateMatchErrorPrefix,
			httpw.URLParameters{"iss": iss, "code": "42", "state": "21"},
		},
		// invalid iss
		{
			callbackISSMatchErrorPrefix,
			httpw.URLParameters{"iss": "37", "code": "42", "state": "21"},
		},
	}

	for _, test := range tests {
		testConnectOAuthParameter(t, test.parameters, test.errPrefix)
	}
}

func TestTokenExpired(t *testing.T) {
	serverURI := getServerURI(t)
	expiredTTL := os.Getenv("OAUTH_EXPIRED_TTL")
	if expiredTTL == "" {
		t.Log(
			"No expired TTL present, skipping this test. Set OAUTH_EXPIRED_TTL env variable to run this test",
		)
		return
	}

	// Convert the env variable to an int and signal error if it is not possible
	expiredInt, expiredErr := strconv.Atoi(expiredTTL)
	if expiredErr != nil {
		t.Fatalf("Cannot convert EXPIRED_TTL env variable to an int with error %v", expiredErr)
	}

	// Get a vpn state
	ck := cookie.NewWithContext(context.Background())
	defer ck.Cancel() //nolint:errcheck
	state, err := New(
		"org.letsconnect-vpn.app.linux",
		"0.1.0-test",
		"configsexpired",
		func(old FSMStateID, new FSMStateID, data interface{}) bool {
			stateCallback(t, &ck, old, new, data)
			return true
		},
		false,
	)
	if err != nil {
		t.Fatalf("Creating client error: %v", err)
	}
	err = state.Register()
	if err != nil {
		t.Fatalf("Registering error: %v", err)
	}

	addErr := state.AddServer(&ck, serverURI, srvtypes.TypeCustom, false)
	if addErr != nil {
		t.Fatalf("Add error: %v", addErr)
	}

	_, configErr := state.GetConfig(&ck, serverURI, srvtypes.TypeCustom, false)

	if configErr != nil {
		t.Fatalf("Connect error before expired: %v", configErr)
	}

	currentServer, serverErr := state.Servers.Current()
	if serverErr != nil {
		t.Fatalf("No server found")
	}

	serverOAuth := currentServer.OAuth()

	accessToken, accessTokenErr := serverOAuth.AccessToken(ck.Context())
	if accessTokenErr != nil {
		t.Fatalf("Failed to get token: %v", accessTokenErr)
	}

	// Wait for TTL so that the tokens expire
	time.Sleep(time.Duration(expiredInt) * time.Second)

	_, configErr = state.GetConfig(&ck, serverURI, srvtypes.TypeCustom, false)

	if configErr != nil {
		t.Fatalf("Connect error after expiry: %v", configErr)
	}

	// Check if tokens have changed
	accessTokenAfter, accessTokenAfterErr := serverOAuth.AccessToken(ck.Context())
	if accessTokenAfterErr != nil {
		t.Fatalf("Failed to get token: %v", accessTokenAfterErr)
	}

	if accessToken == accessTokenAfter {
		t.Errorf("Access token is the same after refresh")
	}
}

// Test if an invalid profile will be corrected.
func TestInvalidProfileCorrected(t *testing.T) {
	serverURI := getServerURI(t)
	ck := cookie.NewWithContext(context.Background())
	defer ck.Cancel() //nolint:errcheck
	state, err := New(
		"org.letsconnect-vpn.app.linux",
		"0.1.0-test",
		"configscancelprofile",
		func(old FSMStateID, new FSMStateID, data interface{}) bool {
			stateCallback(t, &ck, old, new, data)
			return true
		},
		false,
	)
	if err != nil {
		t.Fatalf("Creating client error: %v", err)
	}
	err = state.Register()
	if err != nil {
		t.Fatalf("Registering error: %v", err)
	}

	addErr := state.AddServer(&ck, serverURI, srvtypes.TypeCustom, false)
	if addErr != nil {
		t.Fatalf("Add error: %v", addErr)
	}

	_, configErr := state.GetConfig(&ck, serverURI, srvtypes.TypeCustom, false)

	if configErr != nil {
		t.Fatalf("First connect error: %v", configErr)
	}

	currentServer, serverErr := state.Servers.Current()
	if serverErr != nil {
		t.Fatalf("No server found")
	}

	base, baseErr := currentServer.Base()
	if baseErr != nil {
		t.Fatalf("No base found")
	}

	previousProfile := base.Profiles.Current
	base.Profiles.Current = "IDONOTEXIST"

	_, configErr = state.GetConfig(&ck, serverURI, srvtypes.TypeCustom, false)
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

// Test if prefer tcp is handled correctly by checking the returned config and config type.
func TestPreferTCP(t *testing.T) {
	serverURI := getServerURI(t)
	ck := cookie.NewWithContext(context.Background())
	defer ck.Cancel() //nolint:errcheck
	state, err := New(
		"org.letsconnect-vpn.app.linux",
		"0.1.0-test",
		"configsprefertcp",
		func(old FSMStateID, new FSMStateID, data interface{}) bool {
			stateCallback(t, &ck, old, new, data)
			return true
		},
		false,
	)
	if err != nil {
		t.Fatalf("Creating client error: %v", err)
	}
	err = state.Register()
	if err != nil {
		t.Fatalf("Registering error: %v", err)
	}

	addErr := state.AddServer(&ck, serverURI, srvtypes.TypeCustom, false)
	if addErr != nil {
		t.Fatalf("Add error: %v", addErr)
	}

	// get a config with preferTCP set to true
	config, configErr := state.GetConfig(&ck, serverURI, srvtypes.TypeCustom, true)

	// Test server should accept prefer TCP!
	if config.Protocol != protocol.OpenVPN {
		t.Fatalf("Invalid protocol for prefer TCP, got: WireGuard, want: OpenVPN")
	}

	if configErr != nil {
		t.Fatalf("Config error: %v", configErr)
	}

	// We also test for script security 0 here
	if !strings.HasSuffix(config.VPNConfig, "udp\nscript-security 0") {
		t.Fatalf("Suffix for prefer TCP is not in the right order for config: %s", config.VPNConfig)
	}

	// get a config with preferTCP set to false
	config, configErr = state.GetConfig(&ck, serverURI, srvtypes.TypeCustom, false)
	if configErr != nil {
		t.Fatalf("Config error: %v", configErr)
	}

	// We also test for script security 0 here
	if config.Protocol == protocol.OpenVPN &&
		!strings.HasSuffix(config.VPNConfig, "tcp\nscript-security 0") {
		t.Fatalf("Suffix for disable prefer TCP is not in the right order for config: %s", config.VPNConfig)
	}
}

func TestInvalidClientID(t *testing.T) {
	tests := map[string]bool{
		"test":                          false,
		"org.letsconnect-vpn.app.linux": true,
		"org.letsconnect-vpn":           false,
		"org.letsconnect-vpn.app":       false,
		"org.letsconnect-vpn.linuxsd":   false,
		"org.letsconnect-vpn.app.macos": true,
	}

	for k, v := range tests {
		_, err := New(
			k,
			"0.1.0-test",
			"configsclientid",
			func(old FSMStateID, new FSMStateID, data interface{}) bool {
				return true
			},
			false,
		)
		if v {
			if err != nil {
				t.Fatalf("expected valid register with clientID: %v, got error: %v", k, err)
			}
			continue
		}
		if err == nil {
			t.Fatalf("expected invalid register with clientID: %v, but got no error", k)
		}
		if !strings.HasPrefix(err.Error(), "The client registered with an invalid client ID") {
			t.Fatalf("register error has invalid prefix: %v", err.Error())
		}
	}
}
