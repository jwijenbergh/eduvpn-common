package main

/*
#include <stdlib.h>
#include "error.h"
#include "server.h"

typedef long long int (*ReadRxBytes)();

typedef int (*StateCB)(int oldstate, int newstate, void* data);

static long long int get_read_rx_bytes(ReadRxBytes read)
{
   return read();
}
static int call_callback(StateCB callback, int oldstate, int newstate, void* data)
{
    return callback(oldstate, newstate, data);
}
*/
import "C"

import (
	"encoding/json"
	"unsafe"

	"github.com/go-errors/errors"

	"github.com/eduvpn/eduvpn-common/client"
	srvtypes "github.com/eduvpn/eduvpn-common/types/server"
)

var (
	PStateCallback C.StateCB
	VPNState       *client.Client
)

func getTokens(tokens *C.char) (t srvtypes.Tokens, err error) {
	err = json.Unmarshal([]byte(C.GoString(tokens)), &t)
	return t, err
}

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
	oldState client.FSMStateID,
	newState client.FSMStateID,
	data interface{},
) bool {
	if PStateCallback == nil {
		return false
	}
	oldStateC := C.int(oldState)
	newStateC := C.int(newState)
	d, err := getReturnData(data)
	if err != nil {
		return false
	}
	dataC := C.CString(d)
	handled := C.call_callback(PStateCallback, oldStateC, newStateC, unsafe.Pointer(dataC))
	FreeString(dataC)
	return handled != C.int(0)
}

func getVPNState() (*client.Client, error) {
	if VPNState == nil {
		return nil, errors.New("No state available, did you register the client?")
	}
	return VPNState, nil
}

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
	state := &client.Client{}
	registerErr := state.Register(
		C.GoString(name),
		C.GoString(version),
		C.GoString(configDirectory),
		StateCallback,
		debug != 0,
	)
	// Only update the VPN state if we get no error when registering
	if registerErr == nil {
		VPNState = state
		PStateCallback = stateCallback
		return nil
	}

	return getCError(registerErr)
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

//export SetTokenUpdater
func SetTokenUpdater(name *C.char, updater C.UpdateToken) *C.error {
	nameStr := C.GoString(name)
	state, stateErr := GetVPNState(nameStr)
	if stateErr != nil {
		return getError(stateErr)
	}
	state.SetTokenUpdater(func(srv server.Server, tok oauth.Token) {
		b, err := srv.Base()
		if err != nil {
			log.Logger.Warningf("No server base found for token updating with error: %v", err)
			return
		}
		cName := C.CString(nameStr)
		cSrv := getCPtrServer(state, b)
		cTok := cToken(tok)
		C.update_token(updater, cName, cSrv, cTok)
		FreeString(cName)
	})
	return nil
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

//export CancelOAuth
func CancelOAuth() *C.char {
	state, stateErr := getVPNState()
	if stateErr != nil {
		return getCError(stateErr)
	}
	cancelErr := state.CancelOAuth()
	return getCError(cancelErr)
}

//export AddServer
func AddServer(_type C.int, id *C.char) *C.char {
	// TODO: type
	state, stateErr := getVPNState()
	if stateErr != nil {
		return getCError(stateErr)
	}
	t := int(_type)
	var err error
	switch t {
	case int(srvtypes.TypeInstituteAccess):
	     err = state.AddInstituteServer(C.GoString(id))
	case int(srvtypes.TypeSecureInternet):
		err = state.AddSecureInternetHomeServer(C.GoString(id))
	case int(srvtypes.TypeCustom):
		err = state.AddCustomServer(C.GoString(id))
	default:
		err = errors.Errorf("invalid type: %v", t)
	}
	return getCError(err)
}

//export RemoveServer
func RemoveServer(_type C.int, id *C.char) *C.char {
	state, stateErr := getVPNState()
	if stateErr != nil {
		return getCError(stateErr)
	}
	t := int(_type)
	var err error
	switch t {
	case int(srvtypes.TypeInstituteAccess):
		err = state.RemoveInstituteAccess(C.GoString(id))
	case int(srvtypes.TypeSecureInternet):
		err = state.RemoveSecureInternet()
	case int(srvtypes.TypeCustom):
		err = state.RemoveCustomServer(C.GoString(id))
	default:
		err = errors.Errorf("invalid type: %v", t)
	}
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
func GetConfig(_type C.int, id *C.char, pTCP C.int, tokens *C.char) (*C.char, *C.char) {
	state, stateErr := getVPNState()
	if stateErr != nil {
		return nil, getCError(stateErr)
	}
	preferTCPBool := pTCP != 0
	tok, err := getTokens(tokens)
	if err != nil {
		return nil, getCError(err)
	}
	t := int(_type)
	var cfg *srvtypes.Configuration
	switch t {
	case int(srvtypes.TypeInstituteAccess):
		cfg, err = state.GetConfigInstituteAccess(C.GoString(id), preferTCPBool, tok)
	case int(srvtypes.TypeSecureInternet):
		cfg, err = state.GetConfigSecureInternet(C.GoString(id), preferTCPBool, tok)
	case int(srvtypes.TypeCustom):
		cfg, err = state.GetConfigCustomServer(C.GoString(id), preferTCPBool, tok)
	default:
		err = errors.Errorf("invalid type: %v", t)
	}
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
func SetSecureLocation(data *C.char) *C.char {
	state, stateErr := getVPNState()
	if stateErr != nil {
		return getCError(stateErr)
	}
	locationErr := state.SetSecureLocation(C.GoString(data))
	return getCError(locationErr)
}

//export DiscoServers
func DiscoServers() (*C.char, *C.char) {
	state, stateErr := getVPNState()
	if stateErr != nil {
		return nil, getCError(stateErr)
	}
	servers, err := state.DiscoServers()
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
func DiscoOrganizations() (*C.char, *C.char) {
	state, stateErr := getVPNState()
	if stateErr != nil {
		return nil, getCError(stateErr)
	}
	orgs, err := state.DiscoOrganizations()
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
func Cleanup(prevTokens *C.char) *C.char {
	state, stateErr := getVPNState()
	if stateErr != nil {
		return getCError(stateErr)
	}
	t, err := getTokens(prevTokens)
	if err != nil {
		return getCError(err)
	}
	err = state.Cleanup(t)
	return getCError(err)
}

//export RenewSession
func RenewSession() *C.char {
	state, stateErr := getVPNState()
	if stateErr != nil {
		return getCError(stateErr)
	}
	renewSessionErr := state.RenewSession()
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

//export SecureLocationList
func SecureLocationList() (*C.char, *C.char) {
	state, stateErr := getVPNState()
	if stateErr != nil {
		return nil, getCError(stateErr)
	}
	locs := state.Discovery.SecureLocationList()
	l, err := getReturnData(locs)
	if err != nil {
		return nil, getCError(err)
	}
	return C.CString(l), nil
}

//export StartFailover
func StartFailover(gateway *C.char, mtu C.int, readRxBytes C.ReadRxBytes) (C.int, *C.char) {
	state, stateErr := getVPNState()
	if stateErr != nil {
		return C.int(0), getCError(stateErr)
	}
	dropped, droppedErr := state.StartFailover(C.GoString(gateway), int(mtu), func() (int64, error) {
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

//export CancelFailover
func CancelFailover() *C.char {
	state, stateErr := getVPNState()
	if stateErr != nil {
		return getCError(stateErr)
	}
	cancelErr := state.CancelFailover()
	if cancelErr != nil {
		return getCError(cancelErr)
	}
	return nil
}

//export FreeString
func FreeString(addr *C.char) {
	C.free(unsafe.Pointer(addr))
}

// Not used in library, but needed to compile.
func main() { panic("compile with -buildmode=c-shared") }
