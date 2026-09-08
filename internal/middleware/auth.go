package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// Claims personalizados emitidos por el backend tras validar el login con Google.
type Claims struct {
	UserID         int    `json:"user_id"`
	OrganizationID int    `json:"organization_id"`
	Email          string `json:"email"`
	RolNombre      string `json:"rol"`
	DependenciaID  *int   `json:"dependencia_id,omitempty"`
	jwt.RegisteredClaims
}

const (
	CtxUserID         = "user_id"
	CtxOrganizationID = "organization_id"
	CtxRol            = "rol"
	CtxDependenciaID  = "dependencia_id"
	CtxEmail          = "email"
)

// AuthRequired valida el JWT emitido por /auth/google/callback y coloca en el
// contexto de Gin el usuario, organización y rol para que todos los handlers
// puedan aplicar el filtro multi-tenant y de permisos.
func AuthRequired(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "token de autenticación no provisto",
			})
			return
		}

		tokenString := strings.TrimPrefix(header, "Bearer ")

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
			return []byte(jwtSecret), nil
		})
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "token inválido o expirado",
			})
			return
		}

		c.Set(CtxUserID, claims.UserID)
		c.Set(CtxOrganizationID, claims.OrganizationID)
		c.Set(CtxRol, claims.RolNombre)
		c.Set(CtxDependenciaID, claims.DependenciaID)
		c.Set(CtxEmail, claims.Email)

		c.Next()
	}
}

// RequireRoles restringe el acceso a un endpoint según el rol del usuario autenticado.
// Uso: router.POST("/vehiculos", middleware.RequireRoles("Administrador"), handler)
func RequireRoles(roles ...string) gin.HandlerFunc {
	allowed := make(map[string]bool, len(roles))
	for _, r := range roles {
		allowed[r] = true
	}

	return func(c *gin.Context) {
		rol, exists := c.Get(CtxRol)
		if !exists || !allowed[rol.(string)] {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"success": false,
				"error":   "el rol actual no tiene permisos para esta acción",
			})
			return
		}
		c.Next()
	}
}

// OrganizationID es un helper para que los handlers extraigan el tenant actual.
func OrganizationID(c *gin.Context) int {
	val, _ := c.Get(CtxOrganizationID)
	orgID, _ := val.(int)
	return orgID
}

// UserID es un helper para que los handlers extraigan el usuario actual.
func UserID(c *gin.Context) int {
	val, _ := c.Get(CtxUserID)
	userID, _ := val.(int)
	return userID
}
