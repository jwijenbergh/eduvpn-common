package protocol

type Protocol int8

const (
	// Unknown indicates that the protocol is not known
	Unknown Protocol = iota
	// OpenVPN indicates that the protocol is OpenVPN
	OpenVPN
	// WireGuard indicates that the protocol is WireGuard
	WireGuard
)

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
