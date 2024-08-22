package middleware

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/Nero7991/devlm/internal/config"
	"github.com/Nero7991/devlm/internal/utils"
	"github.com/dgrijalva/jwt-go"
	"github.com/redis/go-redis"
)

var (
	jwtSecret   []byte
	redisClient *redis.Client
)

func init() {
	initJWTSecret()
	initRedisClient()
}

func initJWTSecret() {
	jwtSecret = []byte(config.GetString("JWT_SECRET"))
	if len(jwtSecret) == 0 {
		panic("JWT_SECRET is not set in the configuration")
	}
}

func initRedisClient() {
	redisClient = redis.NewClient(&redis.Options{
		Addr:     config.GetString("REDIS_ADDR"),
		Password: config.GetString("REDIS_PASSWORD"),
		DB:       0,
	})
}

func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := extractToken(r)
		if token == "" {
			log.Printf("Authentication failed: No token provided for %s %s", r.Method, r.URL.Path)
			http.Error(w, "Unauthorized: No token provided", http.StatusUnauthorized)
			return
		}

		claims, err := validateToken(token)
		if err != nil {
			log.Printf("Authentication failed: %v for %s %s", err, r.Method, r.URL.Path)
			http.Error(w, fmt.Sprintf("Unauthorized: %v", err), http.StatusUnauthorized)
			return
		}

		ctx := addUserToContext(r.Context(), claims)
		log.Printf("Authentication successful for user %s on %s %s", claims["username"], r.Method, r.URL.Path)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

func extractToken(r *http.Request) string {
	bearerToken := r.Header.Get("Authorization")
	if len(bearerToken) > 7 && strings.ToUpper(bearerToken[0:7]) == "BEARER " {
		return bearerToken[7:]
	}

	cookie, err := r.Cookie("auth_token")
	if err == nil {
		return cookie.Value
	}

	return r.URL.Query().Get("token")
}

func validateToken(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		if !claims.VerifyExpiresAt(time.Now().Unix(), true) {
			return nil, fmt.Errorf("token expired")
		}

		jti, ok := claims["jti"].(string)
		if !ok {
			return nil, fmt.Errorf("invalid jti claim")
		}

		isRevoked, err := checkTokenRevocation(jti)
		if err != nil {
			return nil, fmt.Errorf("error checking token revocation: %v", err)
		}
		if isRevoked {
			return nil, fmt.Errorf("token has been revoked")
		}

		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}

func checkTokenRevocation(jti string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	exists, err := redisClient.Exists(ctx, fmt.Sprintf("revoked_token:%s", jti)).Result()
	if err != nil {
		return false, err
	}
	return exists == 1, nil
}

func addUserToContext(ctx context.Context, claims jwt.MapClaims) context.Context {
	userID, _ := claims["user_id"].(string)
	username, _ := claims["username"].(string)
	email, _ := claims["email"].(string)
	role, _ := claims["role"].(string)

	if userID == "" || username == "" || email == "" || role == "" {
		log.Printf("Warning: One or more user claims are missing or invalid")
	}

	userInfo := utils.UserInfo{
		ID:       userID,
		Username: username,
		Email:    email,
		Role:     role,
	}

	return context.WithValue(ctx, utils.ContextKeyUserInfo, userInfo)
}

func RevokeToken(jti string, expiration time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return redisClient.Set(ctx, fmt.Sprintf("revoked_token:%s", jti), "revoked", expiration).Err()
}

func RefreshToken(oldToken string) (string, error) {
	claims, err := validateToken(oldToken)
	if err != nil {
		return "", err
	}

	// Remove the original expiration time
	delete(claims, "exp")

	// Set a new expiration time
	claims["exp"] = time.Now().Add(24 * time.Hour).Unix()

	// Create a new token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

func BatchRevokeTokens(jtis []string, expiration time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pipe := redisClient.Pipeline()
	for _, jti := range jtis {
		pipe.Set(ctx, fmt.Sprintf("revoked_token:%s", jti), "revoked", expiration)
	}
	_, err := pipe.Exec(ctx)
	return err
}