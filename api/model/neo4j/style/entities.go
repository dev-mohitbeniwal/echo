// api/model/neo4j/style/entities.go
package style

// NodeStyle represents the visual styling of a node in Neo4j
type NodeStyle struct {
	Color       string `json:"color,omitempty"`
	Size        int    `json:"size,omitempty"`
	Caption     string `json:"caption,omitempty"`
	BorderColor string `json:"border_color,omitempty"`
	BorderWidth int    `json:"border_width,omitempty"`
}

// RelationshipStyle represents the visual styling of a relationship in Neo4j
type RelationshipStyle struct {
	Color     string `json:"color,omitempty"`
	Width     int    `json:"width,omitempty"`
	Style     string `json:"style,omitempty"` // "solid", "dashed", "dotted"
	Caption   string `json:"caption,omitempty"`
	ArrowSize int    `json:"arrow_size,omitempty"`
}

// NodeStyles defines the styles for different node types
var NodeStyles = map[string]NodeStyle{
	"User": {
		Color:       "#FF0000",
		Size:        30,
		Caption:     "name",
		BorderColor: "#CC0000",
		BorderWidth: 2,
	},
	"Organization": {
		Color:       "#00FF00",
		Size:        40,
		Caption:     "name",
		BorderColor: "#008000",
		BorderWidth: 3,
	},
	"Department": {
		Color:       "#0000FF",
		Size:        35,
		Caption:     "name",
		BorderColor: "#000080",
		BorderWidth: 2,
	},
	"Role": {
		Color:       "#FFFF00",
		Size:        25,
		Caption:     "name",
		BorderColor: "#808000",
		BorderWidth: 1,
	},
	"Group": {
		Color:       "#008000",
		Size:        30,
		Caption:     "name",
		BorderColor: "#004000",
		BorderWidth: 2,
	},
	"Permission": {
		Color:       "#FF00FF",
		Size:        20,
		Caption:     "name",
		BorderColor: "#800080",
		BorderWidth: 1,
	},
	"Resource": {
		Color:       "#00FFFF",
		Size:        25,
		Caption:     "name",
		BorderColor: "#008080",
		BorderWidth: 1,
	},
	"Policy": {
		Color:       "#FFA500",
		Size:        30,
		Caption:     "name",
		BorderColor: "#805300",
		BorderWidth: 2,
	},
	"Attribute": {
		Color:       "#800080",
		Size:        20,
		Caption:     "name",
		BorderColor: "#400040",
		BorderWidth: 1,
	},
}

// RelationshipStyles defines the styles for different relationship types
var RelationshipStyles = map[string]RelationshipStyle{
	"PART_OF": {
		Color:     "#333333",
		Width:     2,
		Style:     "solid",
		Caption:   "type",
		ArrowSize: 5,
	},
	"WORKS_FOR": {
		Color:     "#666666",
		Width:     2,
		Style:     "solid",
		Caption:   "type",
		ArrowSize: 5,
	},
	"MEMBER_OF": {
		Color:     "#999999",
		Width:     2,
		Style:     "dashed",
		Caption:   "type",
		ArrowSize: 4,
	},
	"HAS_ROLE": {
		Color:     "#CCCCCC",
		Width:     1,
		Style:     "solid",
		Caption:   "type",
		ArrowSize: 3,
	},
	"HAS_PERMISSION": {
		Color:     "#AAAAAA",
		Width:     1,
		Style:     "dotted",
		Caption:   "type",
		ArrowSize: 3,
	},
	"OWNS": {
		Color:     "#555555",
		Width:     3,
		Style:     "solid",
		Caption:   "type",
		ArrowSize: 6,
	},
	"CAN_ACCESS": {
		Color:     "#777777",
		Width:     1,
		Style:     "dashed",
		Caption:   "type",
		ArrowSize: 4,
	},
	"APPLIES_TO": {
		Color:     "#888888",
		Width:     2,
		Style:     "dotted",
		Caption:   "type",
		ArrowSize: 4,
	},
	"HAS_ATTRIBUTE": {
		Color:     "#BBBBBB",
		Width:     1,
		Style:     "solid",
		Caption:   "type",
		ArrowSize: 3,
	},
	"BELONGS_TO_GROUP": {
		Color:     "#DDDDDD",
		Width:     2,
		Style:     "dashed",
		Caption:   "type",
		ArrowSize: 4,
	},
	"CREATED_BY": {
		Color:     "#EEEEEE",
		Width:     1,
		Style:     "solid",
		Caption:   "type",
		ArrowSize: 3,
	},
	"UPDATED_BY": {
		Color:     "#FFFFFF",
		Width:     1,
		Style:     "dotted",
		Caption:   "type",
		ArrowSize: 3,
	},
}
