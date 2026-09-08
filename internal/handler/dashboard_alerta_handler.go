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

type DashboardHandler struct {
	service *service.DashboardService
}

func NewDashboardHandler(s *service.DashboardService) *DashboardHandler {
	return &DashboardHandler{service: s}
}

// GET /api/v1/dashboard
func (h *DashboardHandler) Resumen(c *gin.Context) {
	orgID := middleware.OrganizationID(c)

	resumen, err := h.service.ObtenerResumenCompleto(c.Request.Context(), orgID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, http.StatusOK, resumen)
}

type AlertaHandler struct {
	service *service.AlertaService
}

func NewAlertaHandler(s *service.AlertaService) *AlertaHandler {
	return &AlertaHandler{service: s}
}

// GET /api/v1/alertas?pendientes=true
func (h *AlertaHandler) List(c *gin.Context) {
	orgID := middleware.OrganizationID(c)
	soloPendientes := c.Query("pendientes") == "true"

	alertas, err := h.service.ListarPorOrganizacion(c.Request.Context(), orgID, soloPendientes)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, http.StatusOK, alertas)
}

// POST /api/v1/alertas/ejecutar-revision
// Endpoint administrativo para forzar manualmente la revisión diaria
// (además de la ejecución automática vía cron). Restringido a Administrador.
func (h *AlertaHandler) EjecutarRevisionManual(c *gin.Context) {
	if err := h.service.EjecutarRevisionDiaria(c.Request.Context()); err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, http.StatusOK, gin.H{"ejecutado": true})
}

// PUT /api/v1/alertas/:id/leida
func (h *AlertaHandler) MarcarLeida(c *gin.Context) {
	orgID := middleware.OrganizationID(c)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "id inválido")
		return
	}
	if err := h.service.MarcarLeida(c.Request.Context(), orgID, id); err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			response.Error(c, http.StatusNotFound, "alerta no encontrada")
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, http.StatusOK, gin.H{"leida": true})
}

// PUT /api/v1/alertas/marcar-todas-leidas
func (h *AlertaHandler) MarcarTodasLeidas(c *gin.Context) {
	orgID := middleware.OrganizationID(c)
	if err := h.service.MarcarTodasLeidas(c.Request.Context(), orgID); err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, http.StatusOK, gin.H{"actualizado": true})
}

// GET /api/v1/alertas/config
func (h *AlertaHandler) ListarConfig(c *gin.Context) {
	orgID := middleware.OrganizationID(c)
	config, err := h.service.ListarConfig(c.Request.Context(), orgID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, http.StatusOK, config)
}

type configAlertaRequest struct {
	DiasAntes int    `json:"dias_antes" binding:"gte=0"`
	Nivel     string `json:"nivel" binding:"required"`
	Activo    *bool  `json:"activo"`
}

// POST /api/v1/alertas/config
func (h *AlertaHandler) CrearConfig(c *gin.Context) {
	orgID := middleware.OrganizationID(c)
	var body configAlertaRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Error(c, http.StatusBadRequest, "datos inválidos: "+err.Error())
		return
	}
	newID, err := h.service.CrearConfig(c.Request.Context(), &models.ConfigAlerta{
		OrganizationID: orgID,
		DiasAntes:      body.DiasAntes,
		Nivel:          body.Nivel,
	})
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, http.StatusCreated, gin.H{"id": newID})
}

// PUT /api/v1/alertas/config/:id
func (h *AlertaHandler) ActualizarConfig(c *gin.Context) {
	orgID := middleware.OrganizationID(c)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "id inválido")
		return
	}
	var body configAlertaRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Error(c, http.StatusBadRequest, "datos inválidos: "+err.Error())
		return
	}
	activo := true
	if body.Activo != nil {
		activo = *body.Activo
	}
	if err := h.service.ActualizarConfig(c.Request.Context(), orgID, id, body.DiasAntes, body.Nivel, activo); err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			response.Error(c, http.StatusNotFound, "regla no encontrada")
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, http.StatusOK, gin.H{"actualizado": true})
}

// DELETE /api/v1/alertas/config/:id
func (h *AlertaHandler) EliminarConfig(c *gin.Context) {
	orgID := middleware.OrganizationID(c)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "id inválido")
		return
	}
	if err := h.service.EliminarConfig(c.Request.Context(), orgID, id); err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			response.Error(c, http.StatusNotFound, "regla no encontrada")
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, http.StatusOK, gin.H{"eliminado": true})
}
