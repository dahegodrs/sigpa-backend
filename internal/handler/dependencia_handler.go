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

type DependenciaHandler struct {
	service *service.DependenciaService
}

func NewDependenciaHandler(s *service.DependenciaService) *DependenciaHandler {
	return &DependenciaHandler{service: s}
}

// GET /api/v1/dependencias
func (h *DependenciaHandler) List(c *gin.Context) {
	orgID := middleware.OrganizationID(c)
	dependencias, err := h.service.List(c.Request.Context(), orgID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, http.StatusOK, dependencias)
}

// POST /api/v1/dependencias
func (h *DependenciaHandler) Create(c *gin.Context) {
	orgID := middleware.OrganizationID(c)
	userID := middleware.UserID(c)

	var d models.Dependencia
	if err := c.ShouldBindJSON(&d); err != nil {
		response.Error(c, http.StatusBadRequest, "datos inválidos: "+err.Error())
		return
	}
	d.OrganizationID = orgID

	newID, err := h.service.Create(c.Request.Context(), &d, userID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, http.StatusCreated, gin.H{"id": newID})
}

// PUT /api/v1/dependencias/:id
func (h *DependenciaHandler) Update(c *gin.Context) {
	orgID := middleware.OrganizationID(c)
	userID := middleware.UserID(c)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "id inválido")
		return
	}

	var d models.Dependencia
	if err := c.ShouldBindJSON(&d); err != nil {
		response.Error(c, http.StatusBadRequest, "datos inválidos: "+err.Error())
		return
	}
	d.ID = id
	d.OrganizationID = orgID

	if err := h.service.Update(c.Request.Context(), &d, userID); err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			response.Error(c, http.StatusNotFound, "dependencia no encontrada")
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, http.StatusOK, gin.H{"actualizado": true})
}

// DELETE /api/v1/dependencias/:id
func (h *DependenciaHandler) Delete(c *gin.Context) {
	orgID := middleware.OrganizationID(c)
	userID := middleware.UserID(c)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "id inválido")
		return
	}

	if err := h.service.Delete(c.Request.Context(), orgID, id, userID); err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			response.Error(c, http.StatusNotFound, "dependencia no encontrada")
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, http.StatusOK, gin.H{"eliminado": true})
}
