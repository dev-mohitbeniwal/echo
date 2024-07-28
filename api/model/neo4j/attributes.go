// api/model/neo4j/attributes.go
package echo_neo4j

// Attribute Keys
const (
	// AttrName represents the name attribute of a node
	AttrName = "name"

	// AttrDescription represents the description attribute of a node
	AttrDescription = "description"

	// AttrID represents the unique identifier of a node
	AttrID = "id"

	// AttrParentID represents the parent identifier of a node
	AttrParentID = "parentID"

	// AttrOrganizationID represents the organization identifier of a node
	AttrOrganizationID = "organizationID"

	// AttrDepartmentID represents the department identifier of a node
	AttrDepartmentID = "departmentID"

	// AttrEmail represents the email attribute of a user
	AttrEmail = "email"

	// AttrUserType represents the type of user (e.g., "AliveLife", "CorporateAdmin", "DepartmentUser")
	AttrUserType = "userType"

	// AttrCreatedAt represents the creation timestamp of a node
	AttrCreatedAt = "createdAt"

	// AttrUpdatedAt represents the last update timestamp of a node
	AttrUpdatedAt = "updatedAt"

	// AttrActive represents whether a node is active
	AttrActive = "active"

	// AttrExpiredAt represents the expiration timestamp of a node (e.g., for sessions or policies)
	AttrExpiredAt = "expiredAt"
)
