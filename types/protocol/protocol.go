// package protocol contains hte public type that have to do with VPN protocols
package protocol

// Protocol defines an 'enumeration' of protocols
type Protocol int8

const (
	// Unknown indicates that the protocol is not known
	Unknown Protocol = iota
	// OpenVPN indicates that the protocol is OpenVPN
	OpenVPN
	// WireGuard indicates that the protocol is WireGuard
	WireGuard
)

// New creates a new protocol type from a string
func New(p string) Protocol {
	switch p {
	case "openvpn":
		return OpenVPN
	case "wireguard":
		return WireGuard
	default:
		return Unknown
	}
}
