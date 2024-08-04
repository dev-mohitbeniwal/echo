package dao

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"go.uber.org/zap"

	"github.com/dev-mohitbeniwal/echo/api/audit"
	echo_errors "github.com/dev-mohitbeniwal/echo/api/errors"
	logger "github.com/dev-mohitbeniwal/echo/api/logging"
	"github.com/dev-mohitbeniwal/echo/api/model"
	echo_neo4j "github.com/dev-mohitbeniwal/echo/api/model/neo4j"
	helper_util "github.com/dev-mohitbeniwal/echo/api/util/helper"
)

type OrganizationDAO struct {
	Driver       neo4j.Driver
	AuditService audit.Service
}

func NewOrganizationDAO(driver neo4j.Driver, auditService audit.Service) *OrganizationDAO {
	dao := &OrganizationDAO{Driver: driver, AuditService: auditService}
	ctx := context.Background()
	if err := dao.EnsureUniqueConstraint(ctx); err != nil {
		logger.Fatal("Failed to ensure unique constraint for Organization", zap.Error(err))
	}
	return dao
}

func (dao *OrganizationDAO) EnsureUniqueConstraint(ctx context.Context) error {
	logger.Info("Ensuring unique constraint on Organization ID")
	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()

	_, err := session.WriteTransaction(func(transaction neo4j.Transaction) (interface{}, error) {
		query := `
        CREATE CONSTRAINT unique_org_id IF NOT EXISTS
        FOR (o:` + echo_neo4j.LabelOrganization + `) REQUIRE o.id IS UNIQUE
        `
		_, err := transaction.Run(query, nil)
		return nil, err
	})

	if err != nil {
		logger.Error("Failed to ensure unique constraint on Organization ID", zap.Error(err))
		return err
	}

	logger.Info("Successfully ensured unique constraint on Organization ID")
	return nil
}

func (dao *OrganizationDAO) CreateOrganization(ctx context.Context, org model.Organization) (string, error) {
	start := time.Now()
	logger.Info("Creating new organization", zap.String("orgName", org.Name))
	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()

	if org.ID == "" {
		org.ID = uuid.New().String()
	}

	result, err := session.WriteTransaction(func(transaction neo4j.Transaction) (interface{}, error) {
		query := `
        MERGE (o:` + echo_neo4j.LabelOrganization + ` {id: $id})
        ON CREATE SET o += $props
        RETURN o.id as id
        `

		params := map[string]interface{}{
			"id": org.ID,
			"props": map[string]interface{}{
				"name":      org.Name,
				"createdAt": time.Now().Format(time.RFC3339),
				"updatedAt": time.Now().Format(time.RFC3339),
			},
		}

		result, err := transaction.Run(query, params)
		if err != nil {
			return nil, echo_errors.ErrDatabaseOperation
		}

		if result.Next() {
			return result.Record().Values[0], nil
		}

		return nil, echo_errors.ErrInternalServer
	})

	duration := time.Since(start)
	if err != nil {
		logger.Error("Failed to create organization",
			zap.Error(err),
			zap.String("orgName", org.Name),
			zap.Duration("duration", duration))
		return "", err
	}

	orgID := fmt.Sprintf("%v", result)
	logger.Info("Organization created successfully",
		zap.String("orgID", orgID),
		zap.Duration("duration", duration))

	// Audit trail
	auditLog := audit.AuditLog{
		Timestamp:     time.Now(),
		UserID:        ctx.Value("requestingUserID").(string),
		Action:        "CREATE_ORGANIZATION",
		ResourceID:    orgID,
		AccessGranted: true,
		ChangeDetails: createOrgChangeDetails(nil, &org),
	}
	if err := dao.AuditService.LogAccess(ctx, auditLog); err != nil {
		logger.Error("Failed to create audit log", zap.Error(err))
	}

	return orgID, nil
}

func (dao *OrganizationDAO) UpdateOrganization(ctx context.Context, org model.Organization) (*model.Organization, error) {
	start := time.Now()
	logger.Info("Updating organization", zap.String("orgID", org.ID))

	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()

	var updatedOrg *model.Organization
	oldOrg, err := dao.GetOrganization(ctx, org.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}

	_, err = session.WriteTransaction(func(transaction neo4j.Transaction) (interface{}, error) {
		query := `
        MATCH (o:` + echo_neo4j.LabelOrganization + ` {id: $id})
        SET o += $props
        RETURN o
        `

		params := map[string]interface{}{
			"id": org.ID,
			"props": map[string]interface{}{
				"name":      org.Name,
				"updatedAt": time.Now().Format(time.RFC3339),
			},
		}

		result, err := transaction.Run(query, params)
		if err != nil {
			return nil, echo_errors.ErrDatabaseOperation
		}

		if result.Next() {
			node := result.Record().Values[0].(neo4j.Node)
			updatedOrg, err = mapNodeToOrganization(node)
			if err != nil {
				return nil, fmt.Errorf("failed to map organization node to struct: %w", err)
			}
			return nil, nil
		}

		return nil, echo_errors.ErrOrganizationNotFound
	})

	duration := time.Since(start)
	if err != nil {
		logger.Error("Failed to update organization",
			zap.Error(err),
			zap.String("orgID", org.ID),
			zap.Duration("duration", duration))
		return nil, err
	}

	logger.Info("Organization updated successfully",
		zap.String("orgID", org.ID),
		zap.Duration("duration", duration))

	// Audit trail
	auditLog := audit.AuditLog{
		Timestamp:     time.Now(),
		UserID:        ctx.Value("requestingUserID").(string),
		Action:        "UPDATE_ORGANIZATION",
		ResourceID:    org.ID,
		AccessGranted: true,
		ChangeDetails: createOrgChangeDetails(oldOrg, updatedOrg),
	}
	if err := dao.AuditService.LogAccess(ctx, auditLog); err != nil {
		logger.Error("Failed to create audit log", zap.Error(err))
	}

	return updatedOrg, nil
}

func (dao *OrganizationDAO) DeleteOrganization(ctx context.Context, orgID string) error {
	start := time.Now()
	logger.Info("Deleting organization", zap.String("orgID", orgID))

	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()

	_, err := session.WriteTransaction(func(transaction neo4j.Transaction) (interface{}, error) {
		query := `
        MATCH (o:` + echo_neo4j.LabelOrganization + ` {id: $id})
        DETACH DELETE o
        `
		result, err := transaction.Run(query, map[string]interface{}{"id": orgID})
		if err != nil {
			return nil, echo_errors.ErrDatabaseOperation
		}

		summary, err := result.Consume()
		if err != nil {
			return nil, echo_errors.ErrDatabaseOperation
		}

		if summary.Counters().NodesDeleted() == 0 {
			return nil, echo_errors.ErrOrganizationNotFound
		}

		return nil, nil
	})

	duration := time.Since(start)
	if err != nil {
		logger.Error("Failed to delete organization",
			zap.Error(err),
			zap.String("orgID", orgID),
			zap.Duration("duration", duration))
		return err
	}

	logger.Info("Organization deleted successfully",
		zap.String("orgID", orgID),
		zap.Duration("duration", duration))

	// Audit trail
	auditLog := audit.AuditLog{
		Timestamp:     time.Now(),
		UserID:        ctx.Value("requestingUserID").(string),
		Action:        "DELETE_ORGANIZATION",
		ResourceID:    orgID,
		AccessGranted: true,
	}
	if err := dao.AuditService.LogAccess(ctx, auditLog); err != nil {
		logger.Error("Failed to create audit log", zap.Error(err))
	}

	return nil
}

func (dao *OrganizationDAO) GetOrganization(ctx context.Context, orgID string) (*model.Organization, error) {
	start := time.Now()
	logger.Info("Retrieving organization", zap.String("orgID", orgID))

	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close()

	query := `
    MATCH (o:` + echo_neo4j.LabelOrganization + ` {id: $id})
    RETURN o
    `
	result, err := session.Run(query, map[string]interface{}{"id": orgID})
	if err != nil {
		logger.Error("Failed to execute get organization query",
			zap.Error(err),
			zap.String("orgID", orgID),
			zap.Duration("duration", time.Since(start)))
		return nil, echo_errors.ErrDatabaseOperation
	}

	if result.Next() {
		node := result.Record().Values[0].(neo4j.Node)
		org, err := mapNodeToOrganization(node)
		if err != nil {
			logger.Error("Failed to map organization node to struct",
				zap.Error(err),
				zap.String("orgID", orgID),
				zap.Duration("duration", time.Since(start)))
			return nil, echo_errors.ErrInternalServer
		}
		logger.Info("Organization retrieved successfully",
			zap.String("orgID", orgID),
			zap.Duration("duration", time.Since(start)))
		return org, nil
	}

	logger.Warn("Organization not found",
		zap.String("orgID", orgID),
		zap.Duration("duration", time.Since(start)))
	return nil, echo_errors.ErrOrganizationNotFound
}

func (dao *OrganizationDAO) ListOrganizations(ctx context.Context, limit int, offset int) ([]*model.Organization, error) {
	start := time.Now()
	logger.Info("Listing organizations", zap.Int("limit", limit), zap.Int("offset", offset))

	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close()

	query := `
    MATCH (o:` + echo_neo4j.LabelOrganization + `)
    RETURN o
    ORDER BY o.createdAt DESC
    SKIP $offset
    LIMIT $limit
    `
	result, err := session.Run(query, map[string]interface{}{
		"limit":  limit,
		"offset": offset,
	})
	if err != nil {
		logger.Error("Failed to execute list organizations query",
			zap.Error(err),
			zap.Duration("duration", time.Since(start)))
		return nil, echo_errors.ErrDatabaseOperation
	}

	var orgs []*model.Organization
	for result.Next() {
		node := result.Record().Values[0].(neo4j.Node)
		org, err := mapNodeToOrganization(node)
		if err != nil {
			logger.Error("Failed to map organization node to struct",
				zap.Error(err),
				zap.Duration("duration", time.Since(start)))
			return nil, echo_errors.ErrInternalServer
		}
		orgs = append(orgs, org)
	}

	logger.Info("Organizations listed successfully",
		zap.Int("count", len(orgs)),
		zap.Duration("duration", time.Since(start)))

	return orgs, nil
}

func (dao *OrganizationDAO) SearchOrganizations(ctx context.Context, criteria model.OrganizationSearchCriteria) ([]*model.Organization, error) {
	start := time.Now()
	logger.Info("Searching organizations", zap.Any("criteria", criteria))

	session := dao.Driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close()

	var queryBuilder strings.Builder
	queryBuilder.WriteString(fmt.Sprintf("MATCH (o:%s) WHERE 1=1", echo_neo4j.LabelOrganization))

	params := make(map[string]interface{})

	if criteria.Name != "" {
		queryBuilder.WriteString(" AND toLower(o.name) CONTAINS toLower($name)")
		params["name"] = criteria.Name
	}

	if criteria.ID != "" {
		queryBuilder.WriteString(" AND o.id = $id")
		params["id"] = criteria.ID
	}

	if criteria.FromDate != nil {
		queryBuilder.WriteString(" AND o.createdAt >= $fromDate")
		params["fromDate"] = criteria.FromDate.Format(time.RFC3339)
	}

	if criteria.ToDate != nil {
		queryBuilder.WriteString(" AND o.createdAt <= $toDate")
		params["toDate"] = criteria.ToDate.Format(time.RFC3339)
	}

	queryBuilder.WriteString(" RETURN o")

	if criteria.SortBy != "" {
		queryBuilder.WriteString(fmt.Sprintf(" ORDER BY o.%s", criteria.SortBy))
		if criteria.SortOrder != "" {
			queryBuilder.WriteString(" " + strings.ToUpper(criteria.SortOrder))
		} else {
			queryBuilder.WriteString(" ASC")
		}
	} else {
		queryBuilder.WriteString(" ORDER BY o.createdAt DESC")
	}

	if criteria.Offset > 0 {
		queryBuilder.WriteString(" SKIP $offset")
		params["offset"] = criteria.Offset
	}

	if criteria.Limit > 0 {
		queryBuilder.WriteString(" LIMIT $limit")
		params["limit"] = criteria.Limit
	}

	logger.Info("Executing query", zap.String("query", queryBuilder.String()), zap.Any("params", params))

	result, err := session.Run(queryBuilder.String(), params)
	if err != nil {
		logger.Error("Failed to execute search organizations query",
			zap.Error(err),
			zap.Duration("duration", time.Since(start)))
		return nil, echo_errors.ErrDatabaseOperation
	}

	var orgs []*model.Organization
	for result.Next() {
		node := result.Record().Values[0].(neo4j.Node)
		org, err := mapNodeToOrganization(node)
		if err != nil {
			logger.Error("Failed to map organization node to struct",
				zap.Error(err),
				zap.Duration("duration", time.Since(start)))
			return nil, echo_errors.ErrInternalServer
		}
		orgs = append(orgs, org)
	}

	logger.Info("Organizations searched successfully",
		zap.Int("count", len(orgs)),
		zap.Duration("duration", time.Since(start)))

	return orgs, nil
}

// Helper function to map Neo4j Node to Organization struct
func mapNodeToOrganization(node neo4j.Node) (*model.Organization, error) {
	props := node.Props
	org := &model.Organization{}

	org.ID = props["id"].(string)
	org.Name = props["name"].(string)
	org.CreatedAt, _ = helper_util.ParseTime(props["createdAt"].(string))
	org.UpdatedAt, _ = helper_util.ParseTime(props["updatedAt"].(string))

	return org, nil
}

// Helper function to create change details for audit log
func createOrgChangeDetails(oldOrg, newOrg *model.Organization) json.RawMessage {
	changes := make(map[string]interface{})
	if oldOrg == nil {
		changes["action"] = "created"
	} else if newOrg == nil {
		changes["action"] = "deleted"
	} else {
		changes["action"] = "updated"
		if oldOrg.Name != newOrg.Name {
			changes["name"] = map[string]string{"old": oldOrg.Name, "new": newOrg.Name}
		}
		// Add more fields as needed
	}
	changeDetails, _ := json.Marshal(changes)
	return changeDetails
}
