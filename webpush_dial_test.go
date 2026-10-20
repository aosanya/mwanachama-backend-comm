package mwanachamacomm

import "testing"

func TestTheDefaultPushClientReachesOnlyPublicAddresses(t *testing.T) {
	for address, public := range map[string]bool{
		"127.0.0.1:443":          false,
		"10.1.2.3:443":           false,
		"192.168.0.10:443":       false,
		"169.254.169.254:443":    false,
		"[::1]:443":              false,
		"[fd00::1]:443":          false,
		"0.0.0.0:443":            false,
		"142.250.180.10:443":     true,
		"[2a00:1450:4001::]:443": true,
	} {
		err := refusePrivateAddress("tcp", address, nil)
		if public && err != nil {
			t.Errorf("%s was refused: %v", address, err)
		}
		if !public && err == nil {
			t.Errorf("%s was allowed", address)
		}
	}
}
