// api/model/neo4j/nodes.go
package echo_neo4j

// Node Labels
const (
	// LabelOrganization represents a tenant or a company in the system
	LabelOrganization = "Organization"

	// LabelDepartment represents a department within an organization
	LabelDepartment = "Department"

	// LabelUser represents a user in the system
	LabelUser = "User"

	// LabelRole represents a role that can be assigned to users
	LabelRole = "Role"

	// LabelPermission represents a specific permission in the system
	LabelPermission = "Permission"

	// LabelResource represents a resource that can be accessed in the system
	LabelResource = "Resource"

	// LabelPolicy represents an access control policy
	LabelPolicy = "Policy"

	// LabelAttribute represents an attribute used in ABAC policies
	LabelAttribute = "Attribute"

	// LabelGroup represents a group of users
	LabelGroup = "Group"

	// LabelSession represents a user session
	LabelSession = "Session"

	// LabelAuditLog represents an audit log entry
	LabelAuditLog = "AuditLog"
)
