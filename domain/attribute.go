package domain

// AttributeMode controls how a token-scoped attribute is exposed in the picker.
type AttributeMode string

const (
	// AttributeModeActive applies values as the initial filter state. The client
	// may change or clear them.
	AttributeModeActive AttributeMode = "active"
	// AttributeModeLocked applies values and prevents the client from changing
	// them.
	AttributeModeLocked AttributeMode = "locked"
	// AttributeModeHidden applies values without exposing them to the client.
	AttributeModeHidden AttributeMode = "hidden"
)

// AttributeGrant is one token-scoped attribute. Values are defaults for active
// attributes and enforced values for locked or hidden attributes.
type AttributeGrant struct {
	Key    string        `json:"key"`
	Values []string      `json:"values"`
	Mode   AttributeMode `json:"mode"`
}

// AssetAttribute is one host-provided key/value pair stored on an asset. Keys
// are dynamic and owned by the consuming application.
type AssetAttribute struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// AttributeFacetValue is one value in use for an attribute key.
type AttributeFacetValue struct {
	Value string `json:"value"`
	Count int64  `json:"count"`
}

// AttributeFacet lists the values of one attribute key that are in use and
// visible to the current actor.
type AttributeFacet struct {
	Key    string                `json:"key"`
	Values []AttributeFacetValue `json:"values"`
}
