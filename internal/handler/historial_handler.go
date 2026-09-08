package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/alcaldia/sigpa-backend/internal/middleware"
	"github.com/alcaldia/sigpa-backend/internal/repository"
	"github.com/alcaldia/sigpa-backend/pkg/response"
)

type HistorialHandler struct {
	repo *repository.HistorialRepository
}

func NewHistorialHandler(repo *repository.HistorialRepository) *HistorialHandler {
	return &HistorialHandler{repo: repo}
}

// GET /api/v1/historial/reciente?limite=10
// Feed global de actividad de toda la organización (cualquier entidad),
// usado por el panel "Actividad reciente" del dashboard.
func (h *HistorialHandler) Reciente(c *gin.Context) {
	orgID := middleware.OrganizationID(c)
	limite, _ := strconv.Atoi(c.DefaultQuery("limite", "10"))

	historial, err := h.repo.ListarRecientePorOrganizacion(c.Request.Context(), orgID, limite)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, http.StatusOK, historial)
}
