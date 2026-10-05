package handlers

import (
	"strings"
	"time"

	"master-panel-api/database"
	"master-panel-api/middleware"
	"master-panel-api/models"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// ListLocalUsers returns list of local users from SQLite
func ListLocalUsers(c *fiber.Ctx) error {
	var users []models.LocalUser
	if err := database.LocalDB.Preload("Role").Order("id ASC").Find(&users).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Gagal mengambil daftar pengguna: " + err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    users,
	})
}

type CreateUserRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
	RoleID   uint   `json:"role_id"`
	IsActive *bool  `json:"is_active"`
}

// CreateLocalUser creates a new local user in SQLite
func CreateLocalUser(c *fiber.Ctx) error {
	var req CreateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Payload tidak valid",
		})
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.Password = strings.TrimSpace(req.Password)

	if req.Name == "" || req.Email == "" || req.Password == "" || req.RoleID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Nama, email, password, dan role wajib diisi",
		})
	}

	// Cek apakah email sudah digunakan
	var count int64
	database.LocalDB.Model(&models.LocalUser{}).Where("LOWER(email) = ?", req.Email).Count(&count)
	if count > 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Email sudah terdaftar di sistem lokal",
		})
	}

	// Cek role valid
	var role models.LocalRole
	if err := database.LocalDB.First(&role, req.RoleID).Error; err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Role yang dipilih tidak ditemukan",
		})
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Gagal mengenkripsi password",
		})
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	newUser := models.LocalUser{
		UUID:      uuid.New().String(),
		Name:      req.Name,
		Email:     req.Email,
		Password:  string(hashed),
		RoleID:    role.ID,
		IsActive:  isActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := database.LocalDB.Create(&newUser).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Gagal menyimpan user: " + err.Error(),
		})
	}

	_ = database.LocalDB.Preload("Role").First(&newUser, newUser.ID)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"message": "User berhasil dibuat",
		"data":    newUser,
	})
}

type UpdateUserRequest struct {
	Name     *string `json:"name"`
	Email    *string `json:"email"`
	Password *string `json:"password"`
	RoleID   *uint   `json:"role_id"`
	IsActive *bool   `json:"is_active"`
}

// UpdateLocalUser updates an existing local user in SQLite
func UpdateLocalUser(c *fiber.Ctx) error {
	id := c.Params("id")
	var user models.LocalUser
	if err := database.LocalDB.Preload("Role").First(&user, "id = ? OR uuid = ?", id, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Pengguna tidak ditemukan",
		})
	}

	var req UpdateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Payload tidak valid",
		})
	}

	if req.Name != nil && strings.TrimSpace(*req.Name) != "" {
		user.Name = strings.TrimSpace(*req.Name)
	}

	if req.Email != nil {
		cleanEmail := strings.ToLower(strings.TrimSpace(*req.Email))
		if cleanEmail != "" && cleanEmail != user.Email {
			var count int64
			database.LocalDB.Model(&models.LocalUser{}).Where("LOWER(email) = ? AND id != ?", cleanEmail, user.ID).Count(&count)
			if count > 0 {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"success": false,
					"message": "Email sudah digunakan oleh user lain",
				})
			}
			user.Email = cleanEmail
		}
	}

	if req.Password != nil && strings.TrimSpace(*req.Password) != "" {
		hashed, err := bcrypt.GenerateFromPassword([]byte(strings.TrimSpace(*req.Password)), bcrypt.DefaultCost)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "Gagal mengenkripsi password baru",
			})
		}
		user.Password = string(hashed)
	}

	if req.RoleID != nil && *req.RoleID != 0 {
		var role models.LocalRole
		if err := database.LocalDB.First(&role, *req.RoleID).Error; err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Role tidak ditemukan",
			})
		}
		user.RoleID = role.ID
	}

	if req.IsActive != nil {
		user.IsActive = *req.IsActive
	}

	user.UpdatedAt = time.Now()
	if err := database.LocalDB.Save(&user).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Gagal memperbarui user: " + err.Error(),
		})
	}

	_ = database.LocalDB.Preload("Role").First(&user, user.ID)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "User berhasil diperbarui",
		"data":    user,
	})
}

// DeleteLocalUser deletes a local user in SQLite
func DeleteLocalUser(c *fiber.Ctx) error {
	id := c.Params("id")
	claims, _ := c.Locals("user").(*middleware.UserClaims)

	var user models.LocalUser
	if err := database.LocalDB.Preload("Role").First(&user, "id = ? OR uuid = ?", id, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Pengguna tidak ditemukan",
		})
	}

	// Jangan izinkan user menghapus dirinya sendiri
	if claims != nil && (claims.UUID == user.UUID || claims.Email == user.Email) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Anda tidak dapat menghapus akun Anda sendiri yang sedang aktif",
		})
	}

	// Cegah penghapusan jika merupakan superadmin terakhir
	if user.Role != nil && user.Role.Code == "superadmin" {
		var superCount int64
		database.LocalDB.Model(&models.LocalUser{}).
			Joins("JOIN local_roles ON local_roles.id = local_users.role_id").
			Where("local_roles.code = 'superadmin' AND local_users.id != ?", user.ID).
			Count(&superCount)
		if superCount == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Tidak dapat menghapus satu-satunya Super Administrator di sistem",
			})
		}
	}

	if err := database.LocalDB.Delete(&user).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Gagal menghapus user: " + err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Pengguna berhasil dihapus",
	})
}

// ListLocalRoles returns all roles from SQLite
func ListLocalRoles(c *fiber.Ctx) error {
	var roles []models.LocalRole
	if err := database.LocalDB.Preload("Users").Order("id ASC").Find(&roles).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Gagal mengambil daftar role: " + err.Error(),
		})
	}

	type RoleResponse struct {
		models.LocalRole
		UserCount int `json:"user_count"`
	}

	resp := make([]RoleResponse, len(roles))
	for i, r := range roles {
		resp[i] = RoleResponse{
			LocalRole: r,
			UserCount: len(r.Users),
		}
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    resp,
	})
}

type UpsertRoleRequest struct {
	Code             string `json:"code"`
	Name             string `json:"name"`
	Description      string `json:"description"`
	CanManageUsers   *bool  `json:"can_manage_users"`
	CanManageRoles   *bool  `json:"can_manage_roles"`
	CanViewWinlose   *bool  `json:"can_view_winlose"`
	CanManageDomains *bool  `json:"can_manage_domains"`
	CanUseSimulator  *bool  `json:"can_use_simulator"`
}

// CreateLocalRole creates a new role in SQLite
func CreateLocalRole(c *fiber.Ctx) error {
	var req UpsertRoleRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Payload tidak valid",
		})
	}

	code := strings.ToLower(strings.TrimSpace(req.Code))
	name := strings.TrimSpace(req.Name)

	if code == "" || name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Kode dan nama role wajib diisi",
		})
	}

	var count int64
	database.LocalDB.Model(&models.LocalRole{}).Where("code = ?", code).Count(&count)
	if count > 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Kode role sudah digunakan",
		})
	}

	newRole := models.LocalRole{
		Code:        code,
		Name:        name,
		Description: req.Description,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if req.CanManageUsers != nil {
		newRole.CanManageUsers = *req.CanManageUsers
	}
	if req.CanManageRoles != nil {
		newRole.CanManageRoles = *req.CanManageRoles
	}
	if req.CanViewWinlose != nil {
		newRole.CanViewWinlose = *req.CanViewWinlose
	}
	if req.CanManageDomains != nil {
		newRole.CanManageDomains = *req.CanManageDomains
	} else {
		newRole.CanManageDomains = true
	}
	if req.CanUseSimulator != nil {
		newRole.CanUseSimulator = *req.CanUseSimulator
	} else {
		newRole.CanUseSimulator = true
	}

	if err := database.LocalDB.Create(&newRole).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Gagal membuat role: " + err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"message": "Role berhasil dibuat",
		"data":    newRole,
	})
}

// UpdateLocalRole updates an existing role in SQLite
func UpdateLocalRole(c *fiber.Ctx) error {
	id := c.Params("id")
	var role models.LocalRole
	if err := database.LocalDB.First(&role, "id = ? OR code = ?", id, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Role tidak ditemukan",
		})
	}

	var req UpsertRoleRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Payload tidak valid",
		})
	}

	// Jangan ubah kode role bawaan penting seperti superadmin
	if role.Code != "superadmin" && strings.TrimSpace(req.Code) != "" {
		newCode := strings.ToLower(strings.TrimSpace(req.Code))
		if newCode != role.Code {
			var count int64
			database.LocalDB.Model(&models.LocalRole{}).Where("code = ? AND id != ?", newCode, role.ID).Count(&count)
			if count > 0 {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"success": false,
					"message": "Kode role sudah digunakan",
				})
			}
			role.Code = newCode
		}
	}

	if strings.TrimSpace(req.Name) != "" {
		role.Name = strings.TrimSpace(req.Name)
	}
	if req.Description != "" {
		role.Description = req.Description
	}

	// Superadmin wajib mempertahankan akses penuh
	if role.Code == "superadmin" {
		role.CanManageUsers = true
		role.CanManageRoles = true
		role.CanViewWinlose = true
		role.CanManageDomains = true
		role.CanUseSimulator = true
	} else {
		if req.CanManageUsers != nil {
			role.CanManageUsers = *req.CanManageUsers
		}
		if req.CanManageRoles != nil {
			role.CanManageRoles = *req.CanManageRoles
		}
		if req.CanViewWinlose != nil {
			role.CanViewWinlose = *req.CanViewWinlose
		}
		if req.CanManageDomains != nil {
			role.CanManageDomains = *req.CanManageDomains
		}
		if req.CanUseSimulator != nil {
			role.CanUseSimulator = *req.CanUseSimulator
		}
	}

	role.UpdatedAt = time.Now()
	if err := database.LocalDB.Save(&role).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Gagal memperbarui role: " + err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Role berhasil diperbarui",
		"data":    role,
	})
}
