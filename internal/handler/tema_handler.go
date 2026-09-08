package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/alcaldia/sigpa-backend/internal/middleware"
	"github.com/alcaldia/sigpa-backend/internal/service"
	"github.com/alcaldia/sigpa-backend/pkg/apperrors"
	"github.com/alcaldia/sigpa-backend/pkg/response"
)

type TemaHandler struct {
	service *service.TemaService
}

func NewTemaHandler(s *service.TemaService) *TemaHandler {
	return &TemaHandler{service: s}
}

// GET /api/v1/public/tema?dominio=alcaldiadefunza.gov.co
// Sin autenticación: el frontend la usa en la pantalla de login para saber
// qué logo/colores mostrar antes de que el usuario inicie sesión. Si el
// dominio no corresponde a ninguna Alcaldía registrada, se retorna 404 y el
// frontend cae a un tema neutro genérico.
func (h *TemaHandler) ObtenerPorDominio(c *gin.Context) {
	dominio := c.Query("dominio")
	if dominio == "" {
		response.Error(c, http.StatusBadRequest, "el parámetro 'dominio' es obligatorio")
		return
	}

	tema, err := h.service.ObtenerPorDominio(c.Request.Context(), dominio)
	if errors.Is(err, apperrors.ErrNotFound) {
		response.Error(c, http.StatusNotFound, "no hay tema configurado para este dominio")
		return
	}
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, http.StatusOK, tema)
}

// GET /api/v1/tema
// Requiere sesión: retorna el tema de la organización del usuario autenticado
// (útil para refrescar el branding dentro de la app, ej. tras cambiar de rol).
func (h *TemaHandler) ObtenerPropio(c *gin.Context) {
	orgID := middleware.OrganizationID(c)

	tema, err := h.service.ObtenerPorOrganizacion(c.Request.Context(), orgID)
	if errors.Is(err, apperrors.ErrNotFound) {
		response.Error(c, http.StatusNotFound, "la organización no tiene un tema configurado")
		return
	}
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, http.StatusOK, tema)
}
