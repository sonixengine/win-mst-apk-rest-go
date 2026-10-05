package handlers

import (
	"strings"
	"time"

	"master-panel-api/config"
	"master-panel-api/database"
	"master-panel-api/middleware"
	"master-panel-api/models"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func Login(c *fiber.Ctx) error {
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" || req.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Email and password are required",
		})
	}

	// 1. Coba verifikasi dengan akun lokal SQLite terlebih dahulu
	var localUser models.LocalUser
	errLocal := database.LocalDB.Preload("Role").Where("LOWER(email) = ?", email).First(&localUser).Error
	if errLocal == nil {
		if !localUser.IsActive {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"success": false,
				"message": "Akun lokal dinonaktifkan. Hubungi Super Administrator.",
			})
		}

		if err := bcrypt.CompareHashAndPassword([]byte(localUser.Password), []byte(req.Password)); err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"success": false,
				"message": "Password salah atau kredensial tidak valid",
			})
		}

		// Update last login
		now := time.Now()
		localUser.LastLoginAt = &now
		_ = database.LocalDB.Model(&localUser).Update("last_login_at", &now)

		roleCode := "operator"
		roleName := "Operator"
		canManageUsers := false
		canManageRoles := false
		canViewWinlose := false
		canManageDomains := false
		canUseSimulator := true

		if localUser.Role != nil {
			roleCode = localUser.Role.Code
			roleName = localUser.Role.Name
			canManageUsers = localUser.Role.CanManageUsers
			canManageRoles = localUser.Role.CanManageRoles
			canViewWinlose = localUser.Role.CanViewWinlose
			canManageDomains = localUser.Role.CanManageDomains
			canUseSimulator = localUser.Role.CanUseSimulator
		}

		claims := middleware.UserClaims{
			UUID:             localUser.UUID,
			Email:            localUser.Email,
			Name:             localUser.Name,
			Master:           roleCode == "superadmin" || roleCode == "admin" || canManageDomains,
			Type:             "access",
			AuthSource:       "sqlite",
			RoleCode:         roleCode,
			RoleName:         roleName,
			CanManageUsers:   canManageUsers,
			CanManageRoles:   canManageRoles,
			CanViewWinlose:   canViewWinlose,
			CanManageDomains: canManageDomains,
			CanUseSimulator:  canUseSimulator,
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
			},
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString([]byte(config.AppConfig.JWTSecret))
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "Gagal generate access token",
			})
		}

		return c.JSON(fiber.Map{
			"success": true,
			"token":   tokenString,
			"user": fiber.Map{
				"uuid":               localUser.UUID,
				"name":               localUser.Name,
				"email":              localUser.Email,
				"auth_source":        "sqlite",
				"role_code":          roleCode,
				"role_name":          roleName,
				"can_manage_users":   canManageUsers,
				"can_manage_roles":   canManageRoles,
				"can_view_winlose":   canViewWinlose,
				"can_manage_domains": canManageDomains,
				"can_use_simulator":  canUseSimulator,
			},
		})
	}

	// 2. Fallback: Verifikasi dengan PostgreSQL (engine-auth) untuk user legacy
	if database.AdminDB != nil {
		var user models.User
		if err := database.AdminDB.Preload("MasterAgent").Preload("UserGroup").Where("LOWER(email) = ?", email).First(&user).Error; err == nil {
			if !user.IsActive {
				return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
					"success": false,
					"message": "Account is inactive",
				})
			}

			if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
					"success": false,
					"message": "Invalid credentials",
				})
			}

			// Sesuai aturan spesifikasi:
			// User engine_auth legacy:
			// - TIDAK BISA melihat laporan winlose (CanViewWinlose = false)
			// - TIDAK BISA mengelola user / role (CanManageUsers = false)
			// - BISA menggunakan worker simulator (CanUseSimulator = true)
			// - BISA melihat & mengelola domain agen terkait (CanManageDomains = true)
			claims := middleware.UserClaims{
				UUID:             user.UUID,
				Email:            user.Email,
				Name:             user.Name,
				Master:           false,
				MasterAgentID:    user.MasterAgentID,
				Type:             "access",
				AuthSource:       "engine_auth",
				RoleCode:         "legacy_engine_auth",
				RoleName:         "Engine Auth Member",
				CanManageUsers:   false,
				CanManageRoles:   false,
				CanViewWinlose:   false, // strictly false
				CanManageDomains: true,
				CanUseSimulator:  true, // strictly true
				RegisteredClaims: jwt.RegisteredClaims{
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
					IssuedAt:  jwt.NewNumericDate(time.Now()),
				},
			}

			token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
			tokenString, err := token.SignedString([]byte(config.AppConfig.JWTSecret))
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"success": false,
					"message": "Failed to generate token",
				})
			}

			return c.JSON(fiber.Map{
				"success": true,
				"token":   tokenString,
				"user": fiber.Map{
					"uuid":               user.UUID,
					"name":               user.Name,
					"email":              user.Email,
					"auth_source":        "engine_auth",
					"is_master":          user.IsMaster,
					"master_agent_id":    user.MasterAgentID,
					"master_agent":       user.MasterAgent,
					"role_code":          "legacy_engine_auth",
					"role_name":          "Engine Auth Member",
					"can_manage_users":   false,
					"can_manage_roles":   false,
					"can_view_winlose":   false,
					"can_manage_domains": true,
					"can_use_simulator":  true,
				},
			})
		}
	}

	return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
		"success": false,
		"message": "Kredensial tidak valid (email atau password salah)",
	})
}

func Me(c *fiber.Ctx) error {
	claims, ok := c.Locals("user").(*middleware.UserClaims)
	if !ok || claims == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "Unauthorized",
		})
	}

	if claims.AuthSource == "sqlite" {
		var localUser models.LocalUser
		if err := database.LocalDB.Preload("Role").Where("uuid = ?", claims.UUID).First(&localUser).Error; err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"message": "Local user not found",
			})
		}

		roleCode := "operator"
		roleName := "Operator"
		if localUser.Role != nil {
			roleCode = localUser.Role.Code
			roleName = localUser.Role.Name
		}

		return c.JSON(fiber.Map{
			"success": true,
			"user": fiber.Map{
				"uuid":               localUser.UUID,
				"name":               localUser.Name,
				"email":              localUser.Email,
				"auth_source":        "sqlite",
				"role_code":          roleCode,
				"role_name":          roleName,
				"role":               localUser.Role,
				"can_manage_users":   claims.CanManageUsers,
				"can_manage_roles":   claims.CanManageRoles,
				"can_view_winlose":   claims.CanViewWinlose,
				"can_manage_domains": claims.CanManageDomains,
				"can_use_simulator":  claims.CanUseSimulator,
			},
		})
	}

	// Legacy engine_auth user
	var user models.User
	if err := database.AdminDB.Preload("MasterAgent").Preload("UserGroup").Where("uuid = ?", claims.UUID).First(&user).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "User not found in engine-auth",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"user": fiber.Map{
			"uuid":               user.UUID,
			"name":               user.Name,
			"email":              user.Email,
			"auth_source":        "engine_auth",
			"is_master":          user.IsMaster,
			"master_agent_id":    user.MasterAgentID,
			"master_agent":       user.MasterAgent,
			"role_code":          "legacy_engine_auth",
			"role_name":          "Engine Auth Member",
			"can_manage_users":   false,
			"can_manage_roles":   false,
			"can_view_winlose":   false,
			"can_manage_domains": true,
			"can_use_simulator":  true,
		},
	})
}
