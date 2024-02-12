package api

// customRedirects supplies redirect values that should be handled by the app itself
// here we hardcode the redirect values that we should use in the OAuth requests
// these values were taken from https://git.sr.ht/~fkooman/vpn-user-portal/tree/v3/item/src/OAuth/VpnClientDb.php
var customRedirects = map[string]string{
	"org.letsconnect-vpn.app.ios": "org.letsconnect-vpn.app.ios:/api/callback",
	// TODO: change to org.letsconnect-vpn.app.android:/api/callback once most servers have commit:
	// https://git.sr.ht/~fkooman/vpn-user-portal/commit/9c0463103c61a55668fff800e83f77a7b6d26e4f#src/OAuth/VpnClientDb.php
	"org.letsconnect-vpn.app.android": "org.letsconnect-vpn.app:/api/callback",
	"org.eduvpn.app.ios":              "org.eduvpn.app.ios:/api/callback",
	// TODO: change to org.eduvpn.app.android:/api/callback once most servers have commit:
	// https://git.sr.ht/~fkooman/vpn-user-portal/commit/9c0463103c61a55668fff800e83f77a7b6d26e4f#src/OAuth/VpnClientDb.php
	"org.eduvpn.app.android": "org.eduvpn.app:/api/callback",
	"org.govvpn.app.ios":     "org.govvpn.app.ios:/api/callback",
	"org.govvpn.app.android": "org.govvpn.app.android:/api/callback",
}

// customRedirect returns the custom redirect string for the clientID `cid`
// Empty string if none is defined or one is defined but is empty.
// In both empty string cases, eduvpn-common handles the redirects as 127.0.0.1 local server redirects
// If a non-empty string is returned, the redirect should be handled by the client and we only use the redirect URI value in our OAuth requests
func customRedirect(cid string) string {
	v, ok := customRedirects[cid]
	if !ok {
		return ""
	}
	return v
}
