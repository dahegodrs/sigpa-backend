package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/alcaldia/sigpa-backend/internal/service"
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
