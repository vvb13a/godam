package domain

import "strings"

const (
	CapabilityAssetsRead       = "assets:read"
	CapabilityAssetsUpload     = "assets:upload"
	CapabilityAssetsUpdate     = "assets:update"
	CapabilityAssetsDelete     = "assets:delete"
	CapabilitySettingsStorage  = "settings:storage"
	CapabilitySettingsPresets  = "settings:presets"
	CapabilitySettingsUpload   = "settings:upload"
	CapabilityConversionsWarm  = "conversions:warm"
	CapabilityConversionsPurge = "conversions:purge"
	CapabilityLibraryUpdate    = "library:update"
)

// AllCapabilities is the explicit set granted to trusted direct access.
var AllCapabilities = []string{
	CapabilityAssetsRead,
	CapabilityAssetsUpload,
	CapabilityAssetsUpdate,
	CapabilityAssetsDelete,
	CapabilitySettingsStorage,
	CapabilitySettingsPresets,
	CapabilitySettingsUpload,
	CapabilityConversionsWarm,
	CapabilityConversionsPurge,
	CapabilityLibraryUpdate,
}

// Can reports whether the capability set allows want. It supports exact
// matches, namespace wildcards such as "assets:*", and the global wildcard "*".
func Can(capabilities []string, want string) bool {
	want = strings.TrimSpace(want)
	if want == "" {
		return true
	}
	for _, raw := range capabilities {
		got := strings.TrimSpace(raw)
		if got == "*" || got == want {
			return true
		}
		if strings.HasSuffix(got, ":*") {
			prefix := strings.TrimSuffix(got, "*")
			if strings.HasPrefix(want, prefix) {
				return true
			}
		}
	}
	return false
}

// NormalizeCapabilities trims, deduplicates, and validates wildcard prefixes.
func NormalizeCapabilities(capabilities []string) []string {
	seen := make(map[string]bool, len(capabilities))
	out := make([]string, 0, len(capabilities))
	for _, raw := range capabilities {
		value := strings.TrimSpace(raw)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
