package main

/*
#include <stdint.h>
#include <stdlib.h>

typedef long long int (*ReadRxBytes)();

typedef int (*StateCB)(int oldstate, int newstate, void* data);

typedef void (*TokenGetter)(const char* server, char* out, size_t len);
typedef void (*TokenSetter)(const char* server, const char* tokens);

static long long int get_read_rx_bytes(ReadRxBytes read)
{
   return read();
}
static int call_callback(StateCB callback, int oldstate, int newstate, void* data)
{
    return callback(oldstate, newstate, data);
}
static void call_token_getter(TokenGetter getter, const char* server, char* out, size_t len)
{
   getter(server, out, len);
}
static void call_token_setter(TokenSetter setter, const char* server, const char* tokens)
{
   setter(server, tokens);
}
*/
import "C"

import (
	"bytes"
	"context"
	"encoding/json"
	"runtime/cgo"
	"unsafe"

	"github.com/go-errors/errors"

	"github.com/eduvpn/eduvpn-common/client"
	"github.com/eduvpn/eduvpn-common/internal/log"
	"github.com/eduvpn/eduvpn-common/types/cookie"
	srvtypes "github.com/eduvpn/eduvpn-common/types/server"
)

var VPNState *client.Client

func getCError(err error) *C.char {
	if err == nil {
		return nil
	}
	return C.CString(err.Error())
}

func getReturnData(data interface{}) (string, error) {
	// If it is already a string return directly
	if x, ok := data.(string); ok {
		return x, nil
	}

	// Otherwise use JSON
	b, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func StateCallback(
	stateCallback C.StateCB,
	oldState client.FSMStateID,
	newState client.FSMStateID,
	data interface{},
) bool {
	oldStateC := C.int(oldState)
	newStateC := C.int(newState)
	d, err := getReturnData(data)
	if err != nil {
		return false
	}
	dataC := C.CString(d)
	handled := C.call_callback(stateCallback, oldStateC, newStateC, unsafe.Pointer(dataC))
	FreeString(dataC)
	return handled != C.int(0)
}

func getVPNState() (*client.Client, error) {
	if VPNState == nil {
		return nil, errors.New("No state available, did you register the client?")
	}
	return VPNState, nil
}

// Register creates a new client and also registers the FSM to go to the initial state
//
//export Register
func Register(
	name *C.char,
	version *C.char,
	configDirectory *C.char,
	stateCallback C.StateCB,
	debug C.int,
) *C.char {
	_, stateErr := getVPNState()
	if stateErr == nil {
		return getCError(errors.New("failed to register, a VPN state is already present"))
	}
	c, err := client.New(
		C.GoString(name),
		C.GoString(version),
		C.GoString(configDirectory),
		func(old client.FSMStateID, new client.FSMStateID, data interface{}) bool {
			return StateCallback(stateCallback, old, new, data)
		},
		debug != 0,
	)
	// Only update the state if we get no error
	if err == nil {
		// Update the global client such that other functions can retrieve it
		// TODO: Use a sync.Once or return a CGO handler instead of a global state?
		VPNState = c
		// finally register the newly created client
		err = c.Register()
		if err != nil {
			// Note: Registering can only fail for non-newly created clients
			// We have obtained a fresh copy here
			// This error is only there for the Go API where you can call register multiple times on an already client
			panic(err)
		}
	}

	return getCError(err)
}

//export ExpiryTimes
func ExpiryTimes() (*C.char, *C.char) {
	state, stateErr := getVPNState()
	if stateErr != nil {
		return nil, getCError(stateErr)
	}
	exp, err := state.ExpiryTimes()
	if err != nil {
		return nil, getCError(err)
	}
	ret, err := getReturnData(exp)
	if err != nil {
		return nil, getCError(err)
	}
	return C.CString(ret), nil
}

//export Deregister
func Deregister() *C.char {
	state, stateErr := getVPNState()
	if stateErr != nil {
		return getCError(stateErr)
	}
	state.Deregister()
	VPNState = nil
	return nil
}

//export AddServer
func AddServer(c C.uintptr_t, _type C.int, id *C.char, ni C.int) *C.char {
	// TODO: type
	state, stateErr := getVPNState()
	if stateErr != nil {
		return getCError(stateErr)
	}
	v, err := getCookie(c)
	if err != nil {
		return getCError(err)
	}
	err = state.AddServer(v, C.GoString(id), srvtypes.Type(_type), ni != 0)
	return getCError(err)
}

//export RemoveServer
func RemoveServer(_type C.int, id *C.char) *C.char {
	state, stateErr := getVPNState()
	if stateErr != nil {
		return getCError(stateErr)
	}
	err := state.RemoveServer(C.GoString(id), srvtypes.Type(_type))
	return getCError(err)
}

//export CurrentServer
func CurrentServer() (*C.char, *C.char) {
	state, stateErr := getVPNState()
	if stateErr != nil {
		return nil, getCError(stateErr)
	}
	srv, err := state.CurrentServer()
	if err != nil {
		return nil, getCError(err)
	}
	ret, err := getReturnData(srv)
	if err != nil {
		return nil, getCError(err)
	}
	return C.CString(ret), nil
}

//export ServerList
func ServerList() (*C.char, *C.char) {
	state, stateErr := getVPNState()
	if stateErr != nil {
		return nil, getCError(stateErr)
	}
	list, err := state.ServerList()
	if err != nil {
		return nil, getCError(err)
	}
	ret, err := getReturnData(list)
	if err != nil {
		return nil, getCError(err)
	}
	return C.CString(ret), nil
}

//export GetConfig
func GetConfig(c C.uintptr_t, _type C.int, id *C.char, pTCP C.int) (*C.char, *C.char) {
	state, stateErr := getVPNState()
	if stateErr != nil {
		return nil, getCError(stateErr)
	}
	ck, err := getCookie(c)
	if err != nil {
		return nil, getCError(err)
	}
	preferTCPBool := pTCP != 0
	cfg, err := state.GetConfig(ck, C.GoString(id), srvtypes.Type(_type), preferTCPBool)
	if cfg != nil && err == nil {
		d, err := getReturnData(cfg)
		if err == nil {
			return C.CString(d), nil
		}
	}
	return nil, getCError(err)
}

//export SetProfileID
func SetProfileID(data *C.char) *C.char {
	state, stateErr := getVPNState()
	if stateErr != nil {
		return getCError(stateErr)
	}
	profileErr := state.SetProfileID(C.GoString(data))
	return getCError(profileErr)
}

//export SetSecureLocation
func SetSecureLocation(c C.uintptr_t, data *C.char) *C.char {
	state, stateErr := getVPNState()
	if stateErr != nil {
		return getCError(stateErr)
	}
	ck, err := getCookie(c)
	if err != nil {
		return getCError(err)
	}
	locationErr := state.SetSecureLocation(ck, C.GoString(data))
	return getCError(locationErr)
}

//export DiscoServers
func DiscoServers(c C.uintptr_t) (*C.char, *C.char) {
	state, stateErr := getVPNState()
	if stateErr != nil {
		return nil, getCError(stateErr)
	}
	ck, err := getCookie(c)
	if err != nil {
		return nil, getCError(err)
	}
	servers, err := state.DiscoServers(ck)
	if servers == nil && err != nil {
		return nil, getCError(err)
	}
	s, reterr := getReturnData(servers)
	if reterr != nil {
		return nil, getCError(reterr)
	}
	return C.CString(s), getCError(err)
}

//export DiscoOrganizations
func DiscoOrganizations(c C.uintptr_t) (*C.char, *C.char) {
	state, stateErr := getVPNState()
	if stateErr != nil {
		return nil, getCError(stateErr)
	}
	ck, err := getCookie(c)
	if err != nil {
		return nil, getCError(err)
	}
	orgs, err := state.DiscoOrganizations(ck)
	if orgs == nil && err != nil {
		return nil, getCError(err)
	}
	s, reterr := getReturnData(orgs)
	if reterr != nil {
		return nil, getCError(reterr)
	}
	return C.CString(s), getCError(err)
}

//export Cleanup
func Cleanup(c C.uintptr_t) *C.char {
	state, stateErr := getVPNState()
	if stateErr != nil {
		return getCError(stateErr)
	}
	ck, err := getCookie(c)
	if err != nil {
		return getCError(err)
	}
	err = state.Cleanup(ck)
	return getCError(err)
}

//export RenewSession
func RenewSession(c C.uintptr_t) *C.char {
	state, stateErr := getVPNState()
	if stateErr != nil {
		return getCError(stateErr)
	}
	ck, err := getCookie(c)
	if err != nil {
		return getCError(err)
	}
	renewSessionErr := state.RenewSession(ck)
	return getCError(renewSessionErr)
}

//export SetSupportWireguard
func SetSupportWireguard(support C.int) *C.char {
	state, stateErr := getVPNState()
	if stateErr != nil {
		return getCError(stateErr)
	}
	state.SupportsWireguard = support != 0
	return nil
}

//export StartFailover
func StartFailover(c C.uintptr_t, gateway *C.char, mtu C.int, readRxBytes C.ReadRxBytes) (C.int, *C.char) {
	state, stateErr := getVPNState()
	if stateErr != nil {
		return C.int(0), getCError(stateErr)
	}
	ck, err := getCookie(c)
	if err != nil {
		return C.int(0), getCError(err)
	}
	dropped, droppedErr := state.StartFailover(ck, C.GoString(gateway), int(mtu), func() (int64, error) {
		rxBytes := int64(C.get_read_rx_bytes(readRxBytes))
		if rxBytes < 0 {
			return 0, errors.New("client gave an invalid rx bytes value")
		}
		return rxBytes, nil
	})
	if droppedErr != nil {
		return C.int(0), getCError(droppedErr)
	}
	droppedC := C.int(0)
	if dropped {
		droppedC = C.int(1)
	}
	return droppedC, nil
}

//export FreeString
func FreeString(addr *C.char) {
	C.free(unsafe.Pointer(addr))
}

func getCookie(c C.uintptr_t) (*cookie.Cookie, error) {
	if c == 0 {
		return nil, errors.New("cookie is nil")
	}
	h := cgo.Handle(c)
	v, ok := h.Value().(*cookie.Cookie)
	if !ok {
		return nil, errors.New("value is not a cookie")
	}
	// the cookie itself has a reference to the handle
	// such that we can return the same exact handle in callbacks
	// TODO: On first glance this might not make any sense, find a better way
	v.H = h
	return v, nil
}

//export SetTokenHandler
func SetTokenHandler(getter C.TokenGetter, setter C.TokenSetter) *C.char {
	state, stateErr := getVPNState()
	if stateErr != nil {
		return getCError(stateErr)
	}
	state.TokenSetter = func(c srvtypes.Current, t srvtypes.Tokens) {
		cJSON, err := getReturnData(c)
		if err != nil {
			log.Logger.Warningf("failed to get current server for setting tokens in exports: %v", err)
			return
		}
		tJSON, err := getReturnData(t)
		if err != nil {
			log.Logger.Warningf("failed to get tokens for setting tokens in exports: %v", err)
			return
		}
		c1 := C.CString(cJSON)
		c2 := C.CString(tJSON)
		C.call_token_setter(setter, c1, c2)
		FreeString(c1)
		FreeString(c2)
	}

	state.TokenGetter = func(c srvtypes.Current) *srvtypes.Tokens {
		cJSON, err := getReturnData(c)
		if err != nil {
			log.Logger.Warningf("failed to get current server for getting tokens in exports: %v", err)
			return nil
		}
		c1 := C.CString(cJSON)
		// create an output buffer with size 2048
		// In my testing tokens seem to be ~1033 bytes marshalled as JSON
		d := make([]byte, 2048)

		C.call_token_getter(getter, c1, (*C.char)(unsafe.Pointer(&d[0])), C.size_t(len(d)))
		FreeString(c1)

		// get null pointer index as unmarshalling wants it without
		null := bytes.IndexByte(d, 0)
		if null < 0 {
			log.Logger.Warningf("output buffer is not NULL terminated")
			return nil
		}

		var gotT srvtypes.Tokens
		err = json.Unmarshal(d[:null], &gotT)
		if err != nil {
			log.Logger.Warningf("failed to get json data for getting tokens in exports: %v", err)
			return nil
		}
		return &gotT
	}


	return nil
}

//export CookieNew
func CookieNew() C.uintptr_t {
	c := cookie.NewWithContext(context.Background())
	return C.uintptr_t(cgo.NewHandle(&c))
}

//export CookieReply
func CookieReply(c C.uintptr_t, data *C.char) *C.char {
	v, err := getCookie(c)
	if err != nil {
		return getCError(err)
	}
	err = v.Send(C.GoString(data))
	return getCError(err)
}

//export CookieDelete
func CookieDelete(c C.uintptr_t) *C.char {
	v, err := getCookie(c)
	if err != nil {
		return getCError(err)
	}
	// cancel the cookie and then delete the handle
	err = v.Cancel()
	cgo.Handle(c).Delete()
	return getCError(err)
}

//export CookieCancel
func CookieCancel(c C.uintptr_t) *C.char {
	v, err := getCookie(c)
	if err != nil {
		return getCError(err)
	}
	err = v.Cancel()
	if err != nil {
		return getCError(err)
	}
	return nil
}

// Not used in library, but needed to compile.
func main() { panic("compile with -buildmode=c-shared") }
