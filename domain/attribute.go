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

const (
	AttributeSortCountAsc  = "count:asc"
	AttributeSortCountDesc = "count:desc"
	AttributeSortABCDesc   = "abc:desc"
	AttributeSortABCAsc    = "abc:asc"
	AttributeSortCustom    = "custom"
	AttributeSortOpen      = "open"
)

// AttributeSorts is the canonical set of attribute ordering settings. "open"
// lets each user choose their own order without the sort capability.
var AttributeSorts = []string{
	AttributeSortCountAsc,
	AttributeSortCountDesc,
	AttributeSortABCDesc,
	AttributeSortABCAsc,
	AttributeSortCustom,
	AttributeSortOpen,
}

// AttributeDefaultSort is used when no metadata row exists. Open keeps the
// per-user sort control available by default.
const AttributeDefaultSort = AttributeSortOpen

// ValidAttributeSort reports whether value is a supported ordering setting.
func ValidAttributeSort(value string) bool {
	for _, sort := range AttributeSorts {
		if sort == value {
			return true
		}
	}
	return false
}

const (
	AttributeRenderSelect   = "select"
	AttributeRenderCheckbox = "checkbox"
	AttributeRenderTags     = "tags"
	AttributeRenderTree     = "tree"
	AttributeRenderOpen     = "open"
)

// AttributeRenders is the canonical set of attribute rendering settings.
// "open" lets each user choose their preferred renderer.
var AttributeRenders = []string{
	AttributeRenderSelect,
	AttributeRenderCheckbox,
	AttributeRenderTags,
	AttributeRenderTree,
	AttributeRenderOpen,
}

// AttributeDefaultRender is used when no metadata row exists. Open keeps the
// per-user render control available by default.
const AttributeDefaultRender = AttributeRenderOpen

// AttributeUnassigned is the synthetic facet value for assets that have no
// value for a key. It is only used in filters, never stored on an asset.
const AttributeUnassigned = "__godam_unassigned__"

// ValidAttributeRender reports whether value is a supported rendering setting.
func ValidAttributeRender(value string) bool {
	for _, render := range AttributeRenders {
		if render == value {
			return true
		}
	}
	return false
}

// AttributeFacetValue is one value in use for an attribute key.
type AttributeFacetValue struct {
	Value       string `json:"value"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Color       string `json:"color,omitempty"`
	Position    int    `json:"position"`
	Count       int64  `json:"count"`
}

// AttributeFacet lists the values of one attribute key that are in use and
// visible to the current actor.
type AttributeFacet struct {
	Key               string                `json:"key"`
	Label             string                `json:"label"`
	Description       string                `json:"description,omitempty"`
	Color             string                `json:"color,omitempty"`
	Sort              string                `json:"sort"`
	Render            string                `json:"render"`
	Searchable        bool                  `json:"searchable"`
	ValueSearch       bool                  `json:"value_search"`
	Multi             bool                  `json:"multi"`
	IncludeUnassigned bool                  `json:"include_unassigned"`
	ValuesLocked      bool                  `json:"values_locked"`
	ParentKey         string                `json:"parent_key,omitempty"`
	Position          int                   `json:"position"`
	Values            []AttributeFacetValue `json:"values"`
}

// AttributeTreeNode is one value in a nested attribute tree. Children are
// values of attributes whose parent_key points at this node's key.
type AttributeTreeNode struct {
	Key         string              `json:"key"`
	Value       string              `json:"value"`
	Label       string              `json:"label"`
	Description string              `json:"description,omitempty"`
	Color       string              `json:"color,omitempty"`
	Position    int                 `json:"position"`
	Count       int64               `json:"count"`
	Children    []AttributeTreeNode `json:"children,omitempty"`
}

// AttributeTree is one root attribute and its nested values.
type AttributeTree struct {
	Key    string              `json:"key"`
	Label  string              `json:"label"`
	Values []AttributeTreeNode `json:"values"`
}
