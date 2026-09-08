package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/alcaldia/sigpa-backend/internal/middleware"
	"github.com/alcaldia/sigpa-backend/internal/repository"
	"github.com/alcaldia/sigpa-backend/pkg/response"
)

type OrganizacionHandler struct {
	repo *repository.OrganizacionRepository
}

func NewOrganizacionHandler(repo *repository.OrganizacionRepository) *OrganizacionHandler {
	return &OrganizacionHandler{repo: repo}
}

// GET /api/v1/organizacion
// Retorna el nombre/dominio de la organización del usuario autenticado.
// Se usa en el pie del sidebar del frontend (ej: "Alcaldía de Funza").
func (h *OrganizacionHandler) ObtenerPropia(c *gin.Context) {
	orgID := middleware.OrganizationID(c)

	org, err := h.repo.GetByID(c.Request.Context(), orgID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, http.StatusOK, org)
}
