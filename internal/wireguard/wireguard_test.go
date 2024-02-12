package wireguard

import (
	"fmt"
	"testing"

	"github.com/eduvpn/eduvpn-common/internal/test"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func TestConfigReplace(t *testing.T) {
	k, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		t.Fatalf("Failed to generate key for wg config replace: %v", err)
	}

	cases := []struct {
		config string
		proxy  string
		want   string
		wantep string
		werr   string
	}{
		{
			config: `
`,
			want:   "",
			wantep: "",
			proxy:  "",
			werr:   "parsed ini is empty",
		},
		{
			config: `
[Interface]
PrivateKey = bla
[interface]

[interface2]

interface

 [Interface]

[Interface]test
`,
			want: fmt.Sprintf(`[Interface]
PrivateKey = %s
[interface]
[interface2]
`, k.String()),
			wantep: "",
			proxy:  "",
			werr:   "",
		},
		{
			config: `
[Interface]
MTU = 1392
PrivateKey =
Address = 10.146.176.5/24,fdee:1ead:29e8:22a2::5/64
DNS = 9.9.9.9,2620:fe::fe

[Peer]
PublicKey =
AllowedIPs = 0.0.0.0/0,::/0
ProxyEndpoint = vpn.example.org:51820
`,
			want: fmt.Sprintf(`[Interface]
MTU = 1392
PrivateKey = %s
Address = 10.146.176.5/24,fdee:1ead:29e8:22a2::5/64
DNS = 9.9.9.9,2620:fe::fe
[Peer]
PublicKey =
AllowedIPs = 0.0.0.0/0,::/0
Endpoint = 127.0.0.1:1337
`, k.String()),
			wantep: "vpn.example.org:51820",
			proxy:  "127.0.0.1:1337",
			werr:   "",
		},
	}

	for _, c := range cases {
		gcfg, gep, err := configReplace(c.config, k, c.proxy)
		test.AssertError(t, err, c.werr)
		if gcfg != c.want {
			t.Fatalf("Got config: %s, not equal to config: %s", gcfg, c.want)
		}

		if gep != c.wantep {
			t.Fatalf("Got endpoint: %s, not equal to endpoint: %s", gep, c.wantep)
		}
	}
}
