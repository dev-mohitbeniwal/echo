// api/model/neo4j/permissions.go

package echo_neo4j

// Permission Types
const (
	// PermissionCreate allows the creation of new resources
	PermissionCreate = "CAN_CREATE"

	// PermissionRead allows reading or viewing existing resources
	PermissionRead = "CAN_READ"

	// PermissionUpdate allows modifying existing resources
	PermissionUpdate = "CAN_UPDATE"

	// PermissionDelete allows removing existing resources
	PermissionDelete = "CAN_DELETE"

	// PermissionExecute allows running or executing processes or scripts
	PermissionExecute = "CAN_EXECUTE"

	// PermissionManage allows overall management of a resource or system
	PermissionManage = "CAN_MANAGE"

	// PermissionList allows viewing a list or index of resources
	PermissionList = "CAN_LIST"

	// PermissionSearch allows performing search operations
	PermissionSearch = "CAN_SEARCH"

	// PermissionShare allows sharing resources with other users
	PermissionShare = "CAN_SHARE"

	// PermissionInvite allows inviting new users to the system
	PermissionInvite = "CAN_INVITE"

	// PermissionAssignRole allows assigning roles to users
	PermissionAssignRole = "CAN_ASSIGN_ROLE"

	// PermissionRevokeRole allows removing roles from users
	PermissionRevokeRole = "CAN_REVOKE_ROLE"

	// PermissionCreatePolicy allows creating new access policies
	PermissionCreatePolicy = "CAN_CREATE_POLICY"

	// PermissionModifyPolicy allows modifying existing access policies
	PermissionModifyPolicy = "CAN_MODIFY_POLICY"

	// PermissionApprove allows approving requests or changes
	PermissionApprove = "CAN_APPROVE"

	// PermissionReject allows rejecting requests or changes
	PermissionReject = "CAN_REJECT"

	// PermissionAudit allows performing audit operations
	PermissionAudit = "CAN_AUDIT"

	// PermissionViewLogs allows viewing system or access logs
	PermissionViewLogs = "CAN_VIEW_LOGS"

	// PermissionCreateTenant allows creating new tenants in a multi-tenant system
	PermissionCreateTenant = "CAN_CREATE_TENANT"

	// PermissionManageTenant allows managing existing tenants
	PermissionManageTenant = "CAN_MANAGE_TENANT"

	// PermissionAccessConfidential allows access to confidential resources
	PermissionAccessConfidential = "CAN_ACCESS_CONFIDENTIAL"

	// PermissionExport allows exporting data from the system
	PermissionExport = "CAN_EXPORT"

	// PermissionImport allows importing data into the system
	PermissionImport = "CAN_IMPORT"

	// PermissionAccessDuringBusinessHours allows access only during defined business hours
	PermissionAccessDuringBusinessHours = "CAN_ACCESS_DURING_BUSINESS_HOURS"

	// PermissionAccessFromApprovedLocation allows access only from approved physical or network locations
	PermissionAccessFromApprovedLocation = "CAN_ACCESS_FROM_APPROVED_LOCATION"

	// PermissionUseAdvancedFeatures allows use of advanced system features
	PermissionUseAdvancedFeatures = "CAN_USE_ADVANCED_FEATURES"

	// PermissionAccessBetaFeatures allows access to beta or unreleased features
	PermissionAccessBetaFeatures = "CAN_ACCESS_BETA_FEATURES"
)
