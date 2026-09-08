package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/alcaldia/sigpa-backend/internal/middleware"
	"github.com/alcaldia/sigpa-backend/internal/service"
	"github.com/alcaldia/sigpa-backend/pkg/apperrors"
	"github.com/alcaldia/sigpa-backend/pkg/response"
)

type UsuarioHandler struct {
	service *service.UsuarioService
}

func NewUsuarioHandler(s *service.UsuarioService) *UsuarioHandler {
	return &UsuarioHandler{service: s}
}

type invitarUsuarioRequest struct {
	Email         string `json:"email" binding:"required,email"`
	Nombre        string `json:"nombre" binding:"required"`
	RolID         int    `json:"rol_id" binding:"required"`
	DependenciaID *int   `json:"dependencia_id"`
}

// POST /api/v1/usuarios
// Pre-registra a alguien con rol/dependencia ya asignados, antes de su
// primer login real con Google.
func (h *UsuarioHandler) Invitar(c *gin.Context) {
	orgID := middleware.OrganizationID(c)
	invitadoPor := middleware.UserID(c)

	var body invitarUsuarioRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Error(c, http.StatusBadRequest, "datos inválidos: "+err.Error())
		return
	}

	newID, err := h.service.InvitarUsuario(c.Request.Context(), orgID, body.Email, body.Nombre, body.RolID, body.DependenciaID, invitadoPor)
	if errors.Is(err, apperrors.ErrDuplicado) {
		response.Error(c, http.StatusConflict, "ya existe un usuario con ese correo")
		return
	}
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, http.StatusCreated, gin.H{"id": newID})
}

// GET /api/v1/usuarios
func (h *UsuarioHandler) List(c *gin.Context) {
	orgID := middleware.OrganizationID(c)

	var rolID, dependenciaID *int
	if v := c.Query("rol_id"); v != "" {
		if id, err := strconv.Atoi(v); err == nil {
			rolID = &id
		}
	}
	if v := c.Query("dependencia_id"); v != "" {
		if id, err := strconv.Atoi(v); err == nil {
			dependenciaID = &id
		}
	}

	usuarios, err := h.service.List(c.Request.Context(), orgID, rolID, dependenciaID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, http.StatusOK, usuarios)
}

type actualizarRolRequest struct {
	RolID         int  `json:"rol_id" binding:"required"`
	DependenciaID *int `json:"dependencia_id"`
}

// PUT /api/v1/usuarios/:id/rol
func (h *UsuarioHandler) ActualizarRol(c *gin.Context) {
	orgID := middleware.OrganizationID(c)
	ejecutadoPor := middleware.UserID(c)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "id inválido")
		return
	}

	var body actualizarRolRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Error(c, http.StatusBadRequest, "datos inválidos: "+err.Error())
		return
	}

	err = h.service.ActualizarRolYDependencia(c.Request.Context(), orgID, id, body.RolID, body.DependenciaID, ejecutadoPor)
	if errors.Is(err, apperrors.ErrNotFound) {
		response.Error(c, http.StatusNotFound, "usuario no encontrado")
		return
	}
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, http.StatusOK, gin.H{"actualizado": true})
}

type actualizarActivoRequest struct {
	Activo bool `json:"activo"`
}

// PUT /api/v1/usuarios/:id/activo
func (h *UsuarioHandler) ActualizarActivo(c *gin.Context) {
	orgID := middleware.OrganizationID(c)
	ejecutadoPor := middleware.UserID(c)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "id inválido")
		return
	}

	var body actualizarActivoRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Error(c, http.StatusBadRequest, "datos inválidos: "+err.Error())
		return
	}

	err = h.service.CambiarActivo(c.Request.Context(), orgID, id, body.Activo, ejecutadoPor)
	if errors.Is(err, apperrors.ErrNotFound) {
		response.Error(c, http.StatusNotFound, "usuario no encontrado")
		return
	}
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, http.StatusOK, gin.H{"actualizado": true})
}
