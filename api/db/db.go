// api/db/db.go
package db

import (
	"context"
	"fmt"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"go.uber.org/zap"

	"github.com/dev-mohitbeniwal/echo/api/config"
	logger "github.com/dev-mohitbeniwal/echo/api/logging"
)

var Neo4jDriver neo4j.Driver

func InitNeo4j() error {
	var err error
	uri := config.GetString("neo4j.uri")
	logger.Info("Connecting to Neo4j at URI", zap.String("uri", uri))
	Neo4jDriver, err = neo4j.NewDriver(
		uri,
		neo4j.BasicAuth(
			config.GetString("neo4j.username"),
			config.GetString("neo4j.password"),
			"",
		),
		func(c *neo4j.Config) {
			c.MaxConnectionLifetime = 30 * time.Minute
			c.MaxConnectionPoolSize = 50
			c.Log = neo4j.ConsoleLogger(neo4j.ERROR)
		},
	)

	if err != nil {
		return fmt.Errorf("failed to create Neo4j driver: %w", err)
	}

	// Test the connection
	_, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = Neo4jDriver.VerifyConnectivity()
	if err != nil {
		return fmt.Errorf("failed to connect to Neo4j: %w", err)
	}

	logger.Info("Successfully connected to Neo4j")
	return nil
}

func CloseNeo4j() {
	if Neo4jDriver != nil {
		_, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := Neo4jDriver.Close()
		if err != nil {
			logger.Error("Error closing Neo4j connection", zap.Error(err))
		} else {
			logger.Info("Neo4j connection closed successfully")
		}
	}
}

// ExecuteReadTransaction executes a read transaction
func ExecuteReadTransaction(ctx context.Context, work neo4j.TransactionWork) (interface{}, error) {
	session := Neo4jDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close()

	result, err := session.ReadTransaction(work)
	if err != nil {
		return nil, fmt.Errorf("failed to execute read transaction: %w", err)
	}

	return result, nil
}

// ExecuteWriteTransaction executes a write transaction
func ExecuteWriteTransaction(ctx context.Context, work neo4j.TransactionWork) (interface{}, error) {
	session := Neo4jDriver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close()

	result, err := session.WriteTransaction(work)
	if err != nil {
		return nil, fmt.Errorf("failed to execute write transaction: %w", err)
	}

	return result, nil
}
