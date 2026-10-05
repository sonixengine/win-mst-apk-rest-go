package middleware

import (
	"strings"

	"master-panel-api/config"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

type UserClaims struct {
	UUID          string `json:"uuid"`
	Email         string `json:"email"`
	Name          string `json:"name"`
	Master        bool   `json:"master"`
	MasterAgentID *uint  `json:"master_agent_id"`
	Type          string `json:"type"`

	// Local Auth extensions
	AuthSource       string `json:"auth_source"` // "sqlite" or "engine_auth"
	RoleCode         string `json:"role_code"`   // "superadmin", "admin", "operator", "legacy_engine_auth"
	RoleName         string `json:"role_name"`   // "Super Administrator", etc.
	CanManageUsers   bool   `json:"can_manage_users"`
	CanManageRoles   bool   `json:"can_manage_roles"`
	CanViewWinlose   bool   `json:"can_view_winlose"`
	CanManageDomains bool   `json:"can_manage_domains"`
	CanUseSimulator  bool   `json:"can_use_simulator"`

	jwt.RegisteredClaims
}

// IsSuperMaster returns true if the user has global master access to all agents:
// - local role "superadmin" (GOD Admin)
// - local role "admin" (Administrator)
// - local user with CanManageDomains = true
// - engine-auth user with Master = true or MasterAgentID = 1
func (u *UserClaims) IsSuperMaster() bool {
	if u == nil {
		return false
	}
	if u.RoleCode == "superadmin" || u.RoleCode == "admin" {
		return true
	}
	if u.AuthSource == "sqlite" && u.CanManageDomains {
		return true
	}
	if u.Master {
		return true
	}
	if u.MasterAgentID != nil && *u.MasterAgentID == 1 {
		return true
	}
	return false
}

// RequireUserManagement middleware checks if user can manage users & roles
func RequireUserManagement() fiber.Handler {
	return func(c *fiber.Ctx) error {
		claims, ok := c.Locals("user").(*UserClaims)
		if !ok || claims == nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"success": false,
				"message": "Unauthorized",
			})
		}
		if !claims.CanManageUsers && !claims.IsSuperMaster() && claims.RoleCode != "superadmin" && claims.RoleCode != "admin" {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"success": false,
				"message": "Akses ditolak: Hanya Super Administrator dan Administrator yang dapat mengakses manajemen pengguna",
			})
		}
		return c.Next()
	}
}

// RequireWinloseAccess middleware blocks engine_auth legacy users and operators from viewing winlose
func RequireWinloseAccess() fiber.Handler {
	return func(c *fiber.Ctx) error {
		claims, ok := c.Locals("user").(*UserClaims)
		if !ok || claims == nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"success": false,
				"message": "Unauthorized",
			})
		}
		if !claims.CanViewWinlose {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"success": false,
				"message": "Akses ditolak: Akun Anda tidak memiliki izin untuk melihat laporan winlose",
			})
		}
		return c.Next()
	}
}

// Protected verifies the JWT access token from the Authorization header
func Protected() fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"success": false,
				"message": "Missing Authorization header",
			})
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"success": false,
				"message": "Invalid token format, expected 'Bearer <token>'",
			})
		}

		tokenString := parts[1]
		claims := &UserClaims{}

		token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
			return []byte(config.AppConfig.JWTSecret), nil
		})

		if err != nil || !token.Valid {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"success": false,
				"message": "Token expired or invalid",
			})
		}

		c.Locals("user", claims)
		return c.Next()
	}
}
