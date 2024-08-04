// api/db/redis.go
package db

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	logger "github.com/dev-mohitbeniwal/echo/api/logging"
	"github.com/dev-mohitbeniwal/echo/api/model"
)

var (
	RedisClient   *redis.Client
	encryptionKey []byte
)

func InitRedis() error {
	RedisClient = redis.NewClient(&redis.Options{
		Addr:         viper.GetString("redis.addr"),
		Password:     viper.GetString("redis.password"),
		DB:           viper.GetInt("redis.db"),
		DialTimeout:  viper.GetDuration("redis.dialTimeout"),
		ReadTimeout:  viper.GetDuration("redis.readTimeout"),
		WriteTimeout: viper.GetDuration("redis.writeTimeout"),
		PoolSize:     viper.GetInt("redis.poolSize"),
		PoolTimeout:  viper.GetDuration("redis.poolTimeout"),
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := RedisClient.Ping(ctx).Result()
	if err != nil {
		return fmt.Errorf("failed to connect to Redis: %w", err)
	}

	encryptionKey = []byte(viper.GetString("redis.encryptionKey"))
	if len(encryptionKey) != 32 {
		return fmt.Errorf("invalid encryption key length: must be 32 bytes")
	}

	logger.Info("Successfully connected to Redis")
	return nil
}

func CloseRedis() {
	if RedisClient != nil {
		if err := RedisClient.Close(); err != nil {
			logger.Error("Error closing Redis connection", zap.Error(err))
		}
	}
}

func encrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func decrypt(ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

func CachePolicy(ctx context.Context, policy *model.Policy) error {
	policyJSON, err := json.Marshal(policy)
	if err != nil {
		return fmt.Errorf("failed to marshal policy: %w", err)
	}

	encryptedPolicy, err := encrypt(policyJSON)
	if err != nil {
		return fmt.Errorf("failed to encrypt policy: %w", err)
	}

	key := fmt.Sprintf("policy:%s", policy.ID)
	defaultTTL := viper.GetDuration("redis.defaultCacheTTL")
	err = RedisClient.Set(ctx, key, base64.StdEncoding.EncodeToString(encryptedPolicy), defaultTTL).Err()
	if err != nil {
		return fmt.Errorf("failed to cache policy: %w", err)
	}

	logger.Debug("Policy cached successfully", zap.String("policyID", policy.ID))
	return nil
}

func GetCachedPolicy(ctx context.Context, policyID string) (*model.Policy, error) {
	key := fmt.Sprintf("policy:%s", policyID)
	encryptedPolicyStr, err := RedisClient.Get(ctx, key).Result()
	if err == redis.Nil {
		logger.Debug("Policy not found in cache", zap.String("policyID", policyID))
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to get policy from cache: %w", err)
	}

	encryptedPolicy, err := base64.StdEncoding.DecodeString(encryptedPolicyStr)
	if err != nil {
		return nil, fmt.Errorf("failed to decode policy: %w", err)
	}

	policyJSON, err := decrypt(encryptedPolicy)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt policy: %w", err)
	}

	var policy model.Policy
	err = json.Unmarshal(policyJSON, &policy)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal policy: %w", err)
	}

	logger.Debug("Policy retrieved from cache", zap.String("policyID", policyID))
	return &policy, nil
}

func DeleteCachedPolicy(ctx context.Context, policyID string) error {
	key := fmt.Sprintf("policy:%s", policyID)
	err := RedisClient.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to delete policy from cache: %w", err)
	}
	logger.Debug("Policy deleted from cache", zap.String("policyID", policyID))
	return nil
}

func CacheOrganization(ctx context.Context, organization *model.Organization) error {
	organizationJSON, err := json.Marshal(organization)
	if err != nil {
		return fmt.Errorf("failed to marshal organization: %w", err)
	}

	key := fmt.Sprintf("organization:%s", organization.ID)
	defaultTTL := viper.GetDuration("redis.defaultCacheTTL")
	err = RedisClient.Set(ctx, key, organizationJSON, defaultTTL).Err()
	if err != nil {
		return fmt.Errorf("failed to cache organization: %w", err)
	}

	logger.Debug("Organization cached successfully", zap.String("organizationID", organization.ID))
	return nil
}

func DeleteCachedOrganization(ctx context.Context, organizationID string) error {
	key := fmt.Sprintf("organization:%s", organizationID)
	err := RedisClient.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to delete organization from cache: %w", err)
	}
	logger.Debug("Organization deleted from cache", zap.String("organizationID", organizationID))
	return nil
}

func GetCachedOrganization(ctx context.Context, organizationID string) (*model.Organization, error) {
	key := fmt.Sprintf("organization:%s", organizationID)
	organizationJSON, err := RedisClient.Get(ctx, key).Result()
	if err == redis.Nil {
		logger.Debug("Organization not found in cache", zap.String("organizationID", organizationID))
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to get organization from cache: %w", err)
	}

	var organization model.Organization
	err = json.Unmarshal([]byte(organizationJSON), &organization)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal organization: %w", err)
	}

	logger.Debug("Organization retrieved from cache", zap.String("organizationID", organizationID))
	return &organization, nil
}

func CacheDepartment(ctx context.Context, department *model.Department) error {
	departmentJSON, err := json.Marshal(department)
	if err != nil {
		return fmt.Errorf("failed to marshal department: %w", err)
	}

	key := fmt.Sprintf("department:%s", department.ID)
	defaultTTL := viper.GetDuration("redis.defaultCacheTTL")
	err = RedisClient.Set(ctx, key, departmentJSON, defaultTTL).Err()
	if err != nil {
		return fmt.Errorf("failed to cache department: %w", err)
	}

	logger.Debug("Department cached successfully", zap.String("departmentID", department.ID))
	return nil
}

func DeleteCachedDepartment(ctx context.Context, departmentID string) error {
	key := fmt.Sprintf("department:%s", departmentID)
	err := RedisClient.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to delete department from cache: %w", err)
	}
	logger.Debug("Department deleted from cache", zap.String("departmentID", departmentID))
	return nil
}

func GetCachedDepartment(ctx context.Context, departmentID string) (*model.Department, error) {
	key := fmt.Sprintf("department:%s", departmentID)
	departmentJSON, err := RedisClient.Get(ctx, key).Result()
	if err == redis.Nil {
		logger.Debug("Department not found in cache", zap.String("departmentID", departmentID))
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to get department from cache: %w", err)
	}

	var department model.Department
	err = json.Unmarshal([]byte(departmentJSON), &department)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal department: %w", err)
	}

	logger.Debug("Department retrieved from cache", zap.String("departmentID", departmentID))
	return &department, nil
}

func CacheUser(ctx context.Context, user *model.User) error {
	userJSON, err := json.Marshal(user)
	if err != nil {
		return fmt.Errorf("failed to marshal user: %w", err)
	}

	key := fmt.Sprintf("user:%s", user.ID)
	defaultTTL := viper.GetDuration("redis.defaultCacheTTL")
	err = RedisClient.Set(ctx, key, userJSON, defaultTTL).Err()
	if err != nil {
		return fmt.Errorf("failed to cache user: %w", err)
	}

	logger.Debug("User cached successfully", zap.String("userID", user.ID))
	return nil
}

func DeleteCachedUser(ctx context.Context, userID string) error {
	key := fmt.Sprintf("user:%s", userID)
	err := RedisClient.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to delete user from cache: %w", err)
	}
	logger.Debug("User deleted from cache", zap.String("userID", userID))
	return nil
}

func GetCachedUser(ctx context.Context, userID string) (*model.User, error) {
	key := fmt.Sprintf("user:%s", userID)
	userJSON, err := RedisClient.Get(ctx, key).Result()
	if err == redis.Nil {
		logger.Debug("User not found in cache", zap.String("userID", userID))
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to get user from cache: %w", err)
	}

	var user model.User
	err = json.Unmarshal([]byte(userJSON), &user)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal user: %w", err)
	}

	logger.Debug("User retrieved from cache", zap.String("userID", userID))
	return &user, nil
}

func CacheRole(ctx context.Context, role *model.Role) error {
	roleJSON, err := json.Marshal(role)
	if err != nil {
		return fmt.Errorf("failed to marshal role: %w", err)
	}

	key := fmt.Sprintf("role:%s", role.ID)
	defaultTTL := viper.GetDuration("redis.defaultCacheTTL")
	err = RedisClient.Set(ctx, key, roleJSON, defaultTTL).Err()
	if err != nil {
		return fmt.Errorf("failed to cache role: %w", err)
	}

	logger.Debug("Role cached successfully", zap.String("roleID", role.ID))
	return nil
}

func DeleteCachedRole(ctx context.Context, roleID string) error {
	key := fmt.Sprintf("role:%s", roleID)
	err := RedisClient.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to delete role from cache: %w", err)
	}
	logger.Debug("Role deleted from cache", zap.String("roleID", roleID))
	return nil
}

func GetCachedRole(ctx context.Context, roleID string) (*model.Role, error) {
	key := fmt.Sprintf("role:%s", roleID)
	roleJSON, err := RedisClient.Get(ctx, key).Result()
	if err == redis.Nil {
		logger.Debug("Role not found in cache", zap.String("roleID", roleID))
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to get role from cache: %w", err)
	}

	var role model.Role
	err = json.Unmarshal([]byte(roleJSON), &role)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal role: %w", err)
	}

	logger.Debug("Role retrieved from cache", zap.String("roleID", roleID))
	return &role, nil
}

// CacheGroup
func CacheGroup(ctx context.Context, group *model.Group) error {
	groupJSON, err := json.Marshal(group)
	if err != nil {
		return fmt.Errorf("failed to marshal group: %w", err)
	}

	key := fmt.Sprintf("group:%s", group.ID)
	defaultTTL := viper.GetDuration("redis.defaultCacheTTL")
	err = RedisClient.Set(ctx, key, groupJSON, defaultTTL).Err()
	if err != nil {
		return fmt.Errorf("failed to cache group: %w", err)
	}

	logger.Debug("Group cached successfully", zap.String("groupID", group.ID))
	return nil
}

// DeleteGroup
func DeleteCachedGroup(ctx context.Context, groupID string) error {
	key := fmt.Sprintf("group:%s", groupID)
	err := RedisClient.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to delete group from cache: %w", err)
	}
	logger.Debug("Group deleted from cache", zap.String("groupID", groupID))
	return nil
}

// GetGroup
func GetCachedGroup(ctx context.Context, groupID string) (*model.Group, error) {
	key := fmt.Sprintf("group:%s", groupID)
	groupJSON, err := RedisClient.Get(ctx, key).Result()
	if err == redis.Nil {
		logger.Debug("Group not found in cache", zap.String("groupID", groupID))
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to get group from cache: %w", err)
	}

	var group model.Group
	err = json.Unmarshal([]byte(groupJSON), &group)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal group: %w", err)
	}

	logger.Debug("Group retrieved from cache", zap.String("groupID", groupID))
	return &group, nil
}

// CachePermission
func CachePermission(ctx context.Context, permission *model.Permission) error {
	permissionJSON, err := json.Marshal(permission)
	if err != nil {
		return fmt.Errorf("failed to marshal permission: %w", err)
	}

	key := fmt.Sprintf("permission:%s", permission.ID)
	defaultTTL := viper.GetDuration("redis.defaultCacheTTL")
	err = RedisClient.Set(ctx, key, permissionJSON, defaultTTL).Err()
	if err != nil {
		return fmt.Errorf("failed to cache permission: %w", err)
	}

	logger.Debug("Permission cached successfully", zap.String("permissionID", permission.ID))
	return nil
}

// DeletePermission
func DeleteCachedPermission(ctx context.Context, permissionID string) error {
	key := fmt.Sprintf("permission:%s", permissionID)
	err := RedisClient.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to delete permission from cache: %w", err)
	}
	logger.Debug("Permission deleted from cache", zap.String("permissionID", permissionID))
	return nil
}

// GetPermission
func GetCachedPermission(ctx context.Context, permissionID string) (*model.Permission, error) {
	key := fmt.Sprintf("permission:%s", permissionID)
	permissionJSON, err := RedisClient.Get(ctx, key).Result()
	if err == redis.Nil {
		logger.Debug("Permission not found in cache", zap.String("permissionID", permissionID))
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to get permission from cache: %w", err)
	}

	var permission model.Permission
	err = json.Unmarshal([]byte(permissionJSON), &permission)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal permission: %w", err)
	}

	logger.Debug("Permission retrieved from cache", zap.String("permissionID", permissionID))
	return &permission, nil
}

// CacheResource
func CacheResource(ctx context.Context, resource *model.Resource) error {
	resourceJSON, err := json.Marshal(resource)
	if err != nil {
		return fmt.Errorf("failed to marshal resource: %w", err)
	}

	key := fmt.Sprintf("resource:%s", resource.ID)
	defaultTTL := viper.GetDuration("redis.defaultCacheTTL")
	err = RedisClient.Set(ctx, key, resourceJSON, defaultTTL).Err()
	if err != nil {
		return fmt.Errorf("failed to cache resource: %w", err)
	}

	logger.Debug("Resource cached successfully", zap.String("resourceID", resource.ID))
	return nil
}

// DeleteResource
func DeleteCachedResource(ctx context.Context, resourceID string) error {
	key := fmt.Sprintf("resource:%s", resourceID)
	err := RedisClient.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to delete resource from cache: %w", err)
	}
	logger.Debug("Resource deleted from cache", zap.String("resourceID", resourceID))
	return nil
}

// GetResource
func GetCachedResource(ctx context.Context, resourceID string) (*model.Resource, error) {
	key := fmt.Sprintf("resource:%s", resourceID)
	resourceJSON, err := RedisClient.Get(ctx, key).Result()
	if err == redis.Nil {
		logger.Debug("Resource not found in cache", zap.String("resourceID", resourceID))
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to get resource from cache: %w", err)
	}

	var resource model.Resource
	err = json.Unmarshal([]byte(resourceJSON), &resource)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal resource: %w", err)
	}

	logger.Debug("Resource retrieved from cache", zap.String("resourceID", resourceID))
	return &resource, nil
}

// GetCachedResourceType
func GetCachedResourceType(ctx context.Context, resourceTypeID string) (*model.ResourceType, error) {
	key := fmt.Sprintf("resourceType:%s", resourceTypeID)
	resourceTypeJSON, err := RedisClient.Get(ctx, key).Result()
	if err == redis.Nil {
		logger.Debug("ResourceType not found in cache", zap.String("resourceTypeID", resourceTypeID))
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to get resourceType from cache: %w", err)
	}

	var resourceType model.ResourceType
	err = json.Unmarshal([]byte(resourceTypeJSON), &resourceType)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal resourceType: %w", err)
	}

	logger.Debug("ResourceType retrieved from cache", zap.String("resourceTypeID", resourceTypeID))
	return &resourceType, nil
}

// CacheResourceType
func CacheResourceType(ctx context.Context, resourceType *model.ResourceType) error {
	resourceTypeJSON, err := json.Marshal(resourceType)
	if err != nil {
		return fmt.Errorf("failed to marshal resourceType: %w", err)
	}

	key := fmt.Sprintf("resourceType:%s", resourceType.ID)
	defaultTTL := viper.GetDuration("redis.defaultCacheTTL")
	err = RedisClient.Set(ctx, key, resourceTypeJSON, defaultTTL).Err()
	if err != nil {
		return fmt.Errorf("failed to cache resourceType: %w", err)
	}

	logger.Debug("ResourceType cached successfully", zap.String("resourceTypeID", resourceType.ID))
	return nil
}

// DeleteResourceType
func DeleteCachedResourceType(ctx context.Context, resourceTypeID string) error {
	key := fmt.Sprintf("resourceType:%s", resourceTypeID)
	err := RedisClient.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to delete resourceType from cache: %w", err)
	}
	logger.Debug("ResourceType deleted from cache", zap.String("resourceTypeID", resourceTypeID))
	return nil
}

// CacheAttributeGroup
func CacheAttributeGroup(ctx context.Context, attributeGroup *model.AttributeGroup) error {
	attributeGroupJSON, err := json.Marshal(attributeGroup)
	if err != nil {
		return fmt.Errorf("failed to marshal attributeGroup: %w", err)
	}

	key := fmt.Sprintf("attributeGroup:%s", attributeGroup.ID)
	defaultTTL := viper.GetDuration("redis.defaultCacheTTL")
	err = RedisClient.Set(ctx, key, attributeGroupJSON, defaultTTL).Err()
	if err != nil {
		return fmt.Errorf("failed to cache attributeGroup: %w", err)
	}

	logger.Debug("AttributeGroup cached successfully", zap.String("attributeGroupID", attributeGroup.ID))
	return nil
}

// DeleteAttributeGroup
func DeleteCachedAttributeGroup(ctx context.Context, attributeGroupID string) error {
	key := fmt.Sprintf("attributeGroup:%s", attributeGroupID)
	err := RedisClient.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to delete attributeGroup from cache: %w", err)
	}
	logger.Debug("AttributeGroup deleted from cache", zap.String("attributeGroupID", attributeGroupID))
	return nil
}

// GetAttributeGroup
func GetCachedAttributeGroup(ctx context.Context, attributeGroupID string) (*model.AttributeGroup, error) {
	key := fmt.Sprintf("attributeGroup:%s", attributeGroupID)
	attributeGroupJSON, err := RedisClient.Get(ctx, key).Result()
	if err == redis.Nil {
		logger.Debug("AttributeGroup not found in cache", zap.String("attributeGroupID", attributeGroupID))
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to get attributeGroup from cache: %w", err)
	}

	var attributeGroup model.AttributeGroup
	err = json.Unmarshal([]byte(attributeGroupJSON), &attributeGroup)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal attributeGroup: %w", err)
	}

	logger.Debug("AttributeGroup retrieved from cache", zap.String("attributeGroupID", attributeGroupID))
	return &attributeGroup, nil
}

func RateLimit(ctx context.Context, key string, limit int, per time.Duration) (bool, error) {
	pipe := RedisClient.Pipeline()
	now := time.Now().UnixNano()
	key = fmt.Sprintf("ratelimit:%s", key)

	pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", now-(per.Nanoseconds())))
	pipe.ZAdd(ctx, key, redis.Z{Score: float64(now), Member: now})
	pipe.ZCard(ctx, key)
	pipe.Expire(ctx, key, per)

	cmds, err := pipe.Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to execute rate limit commands: %w", err)
	}

	count := cmds[2].(*redis.IntCmd).Val()
	allowed := count <= int64(limit)
	logger.Debug("Rate limit check",
		zap.String("key", key),
		zap.Int64("count", count),
		zap.Int("limit", limit),
		zap.Bool("allowed", allowed))
	return allowed, nil
}

func LockResource(ctx context.Context, resourceName string, ttl time.Duration) (bool, error) {
	key := fmt.Sprintf("lock:%s", resourceName)
	locked, err := RedisClient.SetNX(ctx, key, "locked", ttl).Result()
	if err != nil {
		return false, fmt.Errorf("failed to acquire lock: %w", err)
	}
	logger.Debug("Lock acquisition attempt",
		zap.String("resource", resourceName),
		zap.Bool("locked", locked))
	return locked, nil
}

func UnlockResource(ctx context.Context, resourceName string) error {
	key := fmt.Sprintf("lock:%s", resourceName)
	err := RedisClient.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to release lock: %w", err)
	}
	logger.Debug("Lock released", zap.String("resource", resourceName))
	return nil
}
