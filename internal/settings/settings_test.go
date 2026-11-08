package settings

import "testing"

func TestDefaultChoosesNoDevice(t *testing.T) {
	c := Default()
	if c.Backend != BackendPCSC || c.Backends[BackendPCSC].Reader != "" || c.Backends[BackendAT].Port != "" {
		t.Fatalf("a launch must start with no device: %+v", c)
	}
	if c.AID != AIDDefault || !c.AutoNotify || c.HideIdentifiers || c.DebugAPDU || c.DebugHTTP {
		t.Fatalf("defaults %+v", c)
	}
}
