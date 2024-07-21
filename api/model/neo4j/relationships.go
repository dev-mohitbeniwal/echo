// api/model/neo4j/relationships.go
package echo_neo4j

// Relationship Types
const (
	// RelPartOf represents the relationship between a department and its organization
	RelPartOf = "PART_OF"

	// RelWorksFor represents the relationship between a user and their organization
	RelWorksFor = "WORKS_FOR"

	// RelMemberOf represents the relationship between a user and their department
	RelMemberOf = "MEMBER_OF"

	// RelHasRole represents the relationship between a user and their assigned roles
	RelHasRole = "HAS_ROLE"

	// RelHasPermission represents the relationship between a role and its permissions
	RelHasPermission = "HAS_PERMISSION"

	// RelOwns represents the relationship between an organization and its resources
	RelOwns = "OWNS"

	// RelCanAccess represents the relationship between a role and the resources it can access
	RelCanAccess = "CAN_ACCESS"

	// RelAppliesTo represents the relationship between a policy and the resources it applies to
	RelAppliesTo = "APPLIES_TO"

	// RelHasAttribute represents the relationship between a node and its attributes
	RelHasAttribute = "HAS_ATTRIBUTE"

	// RelBelongsToGroup represents the relationship between a user and their groups
	RelBelongsToGroup = "BELONGS_TO_GROUP"

	// RelCreatedBy represents the relationship between a node and its creator
	RelCreatedBy = "CREATED_BY"

	// RelUpdatedBy represents the relationship between a node and its last updater
	RelUpdatedBy = "UPDATED_BY"
)
