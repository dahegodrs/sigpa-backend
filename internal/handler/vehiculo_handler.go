package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/alcaldia/sigpa-backend/internal/middleware"
	"github.com/alcaldia/sigpa-backend/internal/models"
	"github.com/alcaldia/sigpa-backend/internal/service"
	"github.com/alcaldia/sigpa-backend/pkg/apperrors"
	"github.com/alcaldia/sigpa-backend/pkg/response"
)

type VehiculoHandler struct {
	service *service.VehiculoService
}

func NewVehiculoHandler(s *service.VehiculoService) *VehiculoHandler {
	return &VehiculoHandler{service: s}
}

// GET /api/v1/vehiculos
func (h *VehiculoHandler) List(c *gin.Context) {
	orgID := middleware.OrganizationID(c)

	f := models.VehiculoFiltro{
		OrganizationID: orgID,
		Placa:          c.Query("placa"),
		Marca:          c.Query("marca"),
	}
	if v := c.Query("dependencia_id"); v != "" {
		if id, err := strconv.Atoi(v); err == nil {
			f.DependenciaID = &id
		}
	}
	if v := c.Query("tipo_vehiculo_id"); v != "" {
		if id, err := strconv.Atoi(v); err == nil {
			f.TipoVehiculoID = &id
		}
	}
	if v := c.Query("estado_id"); v != "" {
		if id, err := strconv.Atoi(v); err == nil {
			f.EstadoID = &id
		}
	}
	f.Page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	f.PageSize, _ = strconv.Atoi(c.DefaultQuery("page_size", "20"))

	vehiculos, total, err := h.service.List(c.Request.Context(), f)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	totalPages := int((total + int64(f.PageSize) - 1) / int64(f.PageSize))
	response.SuccessWithMeta(c, http.StatusOK, vehiculos, response.Meta{
		Page: f.Page, PageSize: f.PageSize, TotalItems: total, TotalPages: totalPages,
	})
}

// GET /api/v1/vehiculos/:id
func (h *VehiculoHandler) Get(c *gin.Context) {
	orgID := middleware.OrganizationID(c)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "id inválido")
		return
	}

	v, err := h.service.Get(c.Request.Context(), orgID, id)
	if errors.Is(err, apperrors.ErrNotFound) {
		response.Error(c, http.StatusNotFound, "vehículo no encontrado")
		return
	}
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, http.StatusOK, v)
}

// POST /api/v1/vehiculos
func (h *VehiculoHandler) Create(c *gin.Context) {
	orgID := middleware.OrganizationID(c)
	userID := middleware.UserID(c)

	var v models.Vehiculo
	if err := c.ShouldBindJSON(&v); err != nil {
		response.Error(c, http.StatusBadRequest, "datos inválidos: "+err.Error())
		return
	}
	v.OrganizationID = orgID

	newID, err := h.service.Create(c.Request.Context(), &v, userID)
	if errors.Is(err, apperrors.ErrDuplicatePlaca) {
		response.Error(c, http.StatusConflict, err.Error())
		return
	}
	if errors.Is(err, apperrors.ErrValidation) {
		response.Error(c, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, http.StatusCreated, gin.H{"id": newID})
}

// PUT /api/v1/vehiculos/:id
func (h *VehiculoHandler) Update(c *gin.Context) {
	orgID := middleware.OrganizationID(c)
	userID := middleware.UserID(c)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "id inválido")
		return
	}

	var body struct {
		models.Vehiculo
		Motivo string `json:"motivo"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Error(c, http.StatusBadRequest, "datos inválidos: "+err.Error())
		return
	}
	body.Vehiculo.ID = id
	body.Vehiculo.OrganizationID = orgID

	if err := h.service.Update(c.Request.Context(), &body.Vehiculo, userID, body.Motivo); err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			response.Error(c, http.StatusNotFound, "vehículo no encontrado")
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, http.StatusOK, gin.H{"actualizado": true})
}

// DELETE /api/v1/vehiculos/:id
func (h *VehiculoHandler) Delete(c *gin.Context) {
	orgID := middleware.OrganizationID(c)
	userID := middleware.UserID(c)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "id inválido")
		return
	}

	if err := h.service.Delete(c.Request.Context(), orgID, id, userID); err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			response.Error(c, http.StatusNotFound, "vehículo no encontrado")
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, http.StatusOK, gin.H{"eliminado": true})
}

// GET /api/v1/vehiculos/:id/historial
func (h *VehiculoHandler) Historial(c *gin.Context) {
	orgID := middleware.OrganizationID(c)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "id inválido")
		return
	}

	historial, err := h.service.Historial(c.Request.Context(), orgID, id)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, http.StatusOK, historial)
}
