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
