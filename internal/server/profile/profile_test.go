package profile

import "testing"

func Test_CurrentProfileIndex(t *testing.T) {
	testCases := []struct {
		profiles []Profile
		current  string
		index    int
	}{
		{
			profiles: []Profile{
				{
					ID:           "a",
					DisplayName:  "b",
					VPNProtoList: []string{"openvpn", "wireguard"},
				},
			},
			current: "a",
			index:   0,
		},
		{
			profiles: []Profile{
				{
					ID:           "a",
					DisplayName:  "a",
					VPNProtoList: []string{"openvpn", "wireguard"},
				},
				{
					ID:           "b",
					DisplayName:  "b",
					VPNProtoList: []string{"openvpn", "wireguard"},
				},
			},
			current: "b",
			index:   1,
		},
		{
			profiles: []Profile{
				{
					ID:           "a",
					DisplayName:  "a",
					VPNProtoList: []string{"openvpn", "wireguard"},
				},
				{
					ID:           "b",
					DisplayName:  "b",
					VPNProtoList: []string{"openvpn", "wireguard"},
				},
			},
			current: "",
			index:   0,
		},
		{
			profiles: []Profile{
				{
					ID:           "a",
					DisplayName:  "a",
					VPNProtoList: []string{"openvpn", "wireguard"},
				},
				{
					ID:           "b",
					DisplayName:  "b",
					VPNProtoList: []string{"openvpn", "wireguard"},
				},
			},
			current: "",
			index:   0,
		},
		{
			profiles: []Profile{
				{
					ID:           "a",
					DisplayName:  "a",
					VPNProtoList: []string{"openvpn", "wireguard"},
				},
				{
					ID:           "b",
					DisplayName:  "b",
					VPNProtoList: []string{"openvpn", "wireguard"},
				},
			},
			current: "idonotexist",
			index:   0,
		},
	}

	for _, tc := range testCases {
		pri := &Info{
			Current: tc.current,
			Info: ListInfo{
				ProfileList: tc.profiles,
			},
		}
		got := pri.CurrentProfileIndex()
		if got != tc.index {
			t.Fatalf("failed getting profile index, got: '%v', want: '%v'", got, tc.index)
		}
	}
}
