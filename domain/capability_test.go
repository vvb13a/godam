package domain

import "testing"

func TestCan(t *testing.T) {
	caps := []string{"assets:*", "settings:upload"}
	for _, capability := range []string{
		CapabilityAssetsRead,
		CapabilityAssetsUpload,
		CapabilityAssetsUpdate,
		CapabilityAssetsDelete,
		CapabilitySettingsUpload,
	} {
		if !Can(caps, capability) {
			t.Fatalf("%q should be allowed", capability)
		}
	}
	if Can(caps, CapabilitySettingsStorage) {
		t.Fatal("settings:storage should not be allowed")
	}
	if !Can([]string{"*"}, CapabilitySettingsStorage) {
		t.Fatal("global wildcard should allow everything")
	}
}
