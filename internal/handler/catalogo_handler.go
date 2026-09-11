package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/alcaldia/sigpa-backend/internal/service"
	"github.com/alcaldia/sigpa-backend/pkg/apperrors"
	"github.com/alcaldia/sigpa-backend/pkg/response"
)

type CatalogoHandler struct {
	service *service.CatalogoService
}

func NewCatalogoHandler(s *service.CatalogoService) *CatalogoHandler {
	return &CatalogoHandler{service: s}
}

// GET /api/v1/catalogos
func (h *CatalogoHandler) Todos(c *gin.Context) {
	catalogos, err := h.service.ObtenerTodos(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, http.StatusOK, catalogos)
}

// ── Gestión de tipos de vehículo desde Administración → Listas ─────────────

// GET /api/v1/catalogos/tipos-vehiculo/todos
// Incluye activos e inactivos, para el panel de administración.
func (h *CatalogoHandler) ListarTodosTiposVehiculo(c *gin.Context) {
	tipos, err := h.service.ListarTodosTiposVehiculo(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, http.StatusOK, tipos)
}

type tipoVehiculoRequest struct {
	Nombre string `json:"nombre" binding:"required"`
}

// POST /api/v1/catalogos/tipos-vehiculo
func (h *CatalogoHandler) CrearTipoVehiculo(c *gin.Context) {
	var body tipoVehiculoRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Error(c, http.StatusBadRequest, "el nombre es requerido")
		return
	}
	id, err := h.service.CrearTipoVehiculo(c.Request.Context(), body.Nombre)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, http.StatusCreated, gin.H{"id": id})
}

type tipoVehiculoUpdateRequest struct {
	Nombre string `json:"nombre" binding:"required"`
	Activo bool   `json:"activo"`
}

// PUT /api/v1/catalogos/tipos-vehiculo/:id
func (h *CatalogoHandler) ActualizarTipoVehiculo(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "id inválido")
		return
	}
	var body tipoVehiculoUpdateRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Error(c, http.StatusBadRequest, "datos inválidos")
		return
	}
	if err := h.service.ActualizarTipoVehiculo(c.Request.Context(), id, body.Nombre, body.Activo); err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			response.Error(c, http.StatusNotFound, "tipo de vehículo no encontrado")
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, http.StatusOK, gin.H{"actualizado": true})
}
