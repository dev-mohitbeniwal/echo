// api/model/neo4j/nodes.go
package echo_neo4j

// Node Labels
const (
	// LabelOrganization represents a tenant or a company in the system
	LabelOrganization = "ORGANIZATION"

	// LabelDepartment represents a department within an organization
	LabelDepartment = "DEPARTMENT"

	// LabelUser represents a user in the system
	LabelUser = "USER"

	// LabelRole represents a role that can be assigned to users
	LabelRole = "ROLE"

	// LabelPermission represents a specific permission in the system
	LabelPermission = "` + echo_neo4j.LabelPermission + `"

	// LabelResource represents a resource that can be accessed in the system
	LabelResource = "RESOURCE"

	// LabelPolicy represents an access control policy
	LabelPolicy = "POLICY"

	// LabelAttribute represents an attribute used in ABAC policies
	LabelAttribute = "ATTRIBUTE"

	// LabelGroup represents a group of users
	LabelGroup = "GROUP"

	// LabelSession represents a user session
	LabelSession = "SESSION"

	// LabelAuditLog represents an audit log entry
	LabelAuditLog = "AUDIT_LOG"
)
