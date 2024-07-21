package middleware

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"strings"

	"github.com/dev-mohitbeniwal/echo/api/config"
	logger "github.com/dev-mohitbeniwal/echo/api/logging"
	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type JSONWebKey struct {
	Kty string `json:"kty"`
	E   string `json:"e"`
	Use string `json:"use"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	N   string `json:"n"`
}

type CognitoClaims struct {
	jwt.StandardClaims
	CognitoGroups   []string `json:"cognito:groups"`
	CognitoUsername string   `json:"cognito:username"`
	EmailVerified   bool     `json:"email_verified"`
	Email           string   `json:"email"`
}

type Jwks struct {
	Keys []JSONWebKey `json:"keys"`
}

// GetCognitoPublicKey fetches the public key from a specified Cognito JWKS endpoint
func GetCognitoPublicKey(region, userPoolID string) (*rsa.PublicKey, error) {
	jwksUrl := fmt.Sprintf("https://cognito-idp.%s.amazonaws.com/%s/.well-known/jwks.json", region, userPoolID)
	logger.Info("Fetching JWKS from URL: %s", zap.String("url", jwksUrl))

	resp, err := http.Get(jwksUrl)
	if err != nil {
		logger.Error("Failed to fetch JWKS: %v", zap.Error(err))
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Error("Received non-OK HTTP status from JWKS endpoint: %d", zap.Int("statusCode", resp.StatusCode))
		return nil, fmt.Errorf("received non-OK HTTP status from JWKS endpoint: %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logger.Error("Failed to read JWKS response body: %v", zap.Error(err))
		return nil, err
	}

	var jwks Jwks
	err = json.Unmarshal(body, &jwks)
	if err != nil {
		logger.Error("Failed to unmarshal JWKS JSON: %v", zap.Error(err))
		return nil, err
	}

	if len(jwks.Keys) == 0 {
		logger.Error("No keys found in JWKS")
		return nil, fmt.Errorf("no keys found in JWKS")
	}

	key := jwks.Keys[0] // Assuming the first key is the one needed; consider more robust selection mechanisms
	nBytes, err := base64.RawURLEncoding.DecodeString(key.N)
	if err != nil {
		logger.Error("Failed to decode modulus: %v", zap.Error(err))
		return nil, err
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(key.E)
	if err != nil {
		logger.Error("Failed to decode exponent: %v", zap.Error(err))
		return nil, err
	}

	n := new(big.Int).SetBytes(nBytes)
	e := new(big.Int).SetBytes(eBytes).Int64()

	publicKey := &rsa.PublicKey{
		N: n,
		E: int(e),
	}
	logger.Info("Successfully parsed public key: %+v", zap.Any("publicKey", publicKey))
	return publicKey, nil
}

func GroupAuthMiddleware(requiredGroups []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := c.GetHeader("Authorization")
		logger.Info("Received token: %s", zap.String("token", tokenString))
		if tokenString == "" {
			logger.Warn("No Authorization token provided")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			c.Abort()
			return
		}

		claims, err := parseTokenUnverified(tokenString)
		if err != nil {
			logger.Error("Error parsing token: %v", zap.Error(err))
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			c.Abort()
			return
		}

		logger.Info("Parsed claims: %+v", zap.Any("claims", claims))
		if !isUserInGroups(claims, requiredGroups) {
			logger.Warn("User does not have the required groups")
			c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden"})
			c.Abort()
			return
		}

		// Add the user's sub to the context
		c.Set("requestingUserID", claims.Subject)
		c.Set("requestingUser", claims.CognitoUsername)
		logger.Info("Added user sub to context: %s", zap.Any("sub", claims.Subject))

		c.Next()
	}
}

func parseTokenUnverified(tokenString string) (*CognitoClaims, error) {
	tokenString = strings.TrimPrefix(tokenString, "Bearer ")
	key, err := GetCognitoPublicKey(config.GetString("auth.cognito.aws_region"), config.GetString("auth.cognito.user_pool_id"))
	if err != nil {
		logger.Error("Error getting public key: %v", zap.Error(err))
		return nil, err
	}

	// Parse the token with the custom claimsdfrcx
	token, err := jwt.ParseWithClaims(tokenString, &CognitoClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate the alg is what you expect:
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return key, nil
	})

	if err != nil {
		logger.Error("Error parsing token: %v", zap.Error(err))
		return nil, err
	}

	// Print the entire token, both claims and header, for debugging
	logger.Info("Token header: %+v", zap.Any("header", token.Header))
	logger.Info("Token claims: %+v", zap.Any("claims", token.Claims))

	// Check the validity and type assertion of the token
	if claims, ok := token.Claims.(*CognitoClaims); ok && token.Valid {
		logger.Info("Parsed claims: %+v", zap.Any("claims", claims))
		return claims, nil
	}

	logger.Error("The token is invalid or does not match the expected claim type")
	return nil, fmt.Errorf("invalid token or wrong claims type")
}

// Update the group checking function to use CognitoClaims
func isUserInGroups(claims *CognitoClaims, requiredGroups []string) bool {
	for _, group := range requiredGroups {
		for _, userGroup := range claims.CognitoGroups {
			if userGroup == group {
				return true
			}
		}
	}
	return false
}
