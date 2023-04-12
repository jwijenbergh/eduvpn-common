//go:build !windows

package failover

import (
	"net"

	"github.com/go-errors/errors"
	"golang.org/x/net/icmp"
)

func NewPinger(gateway string, size int) (*Pinger, error) {
	l, err := icmp.ListenPacket("udp4", "0.0.0.0")
	if err != nil {
		return nil, errors.WrapPrefix(err, "failed creating ping", 0)
	}
	return &Pinger{
		listener: l,
		buffer:   make([]byte, size-mtuOverhead),
		gateway:  &net.UDPAddr{IP: net.ParseIP(gateway)},
	}, nil
}
