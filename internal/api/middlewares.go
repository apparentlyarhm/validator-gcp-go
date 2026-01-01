package api

import (
	"context"
	"log"
	"net/http"
	"strings"

	"github.com/validator-gcp/v2/internal/service"
)

type contextKey string

const (
	// UserContextKey is used to store/retrieve the UserClaims in the request context
	UserContextKey contextKey = "user_claims"
)

// validates the JWT token and injects the user claims into the context.
// It acts as a factory that accepts the AuthService dependency.
func AuthMiddleware(a *service.AuthService) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "Unauthorized: Missing Authorization Header", http.StatusUnauthorized)
				return
			}

			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			if tokenStr == authHeader {
				http.Error(w, "Unauthorized: Invalid Token Format", http.StatusUnauthorized)
				return
			}

			claims, err := a.ValidateToken(tokenStr)
			if err != nil {
				log.Printf("Invalid token attempt %v by %v", err, r.RemoteAddr)
				http.Error(w, "Forbidden: Invalid or Expired Token", http.StatusForbidden)
				return
			}

			ctx := context.WithValue(r.Context(), UserContextKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole ensures the user in the context has one of the allowed roles.
// This MUST be used AFTER AuthMiddleware.
func RequireRole(allowedRoles ...string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := r.Context().Value(UserContextKey).(*service.UserClaims)
			if !ok {
				// Should result in a 500 (Server Config Error) or 401.
				http.Error(w, "Unauthorized: No User Context", http.StatusUnauthorized)
				return
			}

			hasRole := false
			for _, role := range allowedRoles {
				if claims.Role == role {
					hasRole = true
					break
				}
			}

			if !hasRole {
				http.Error(w, "Forbidden: Insufficient Permissions", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
