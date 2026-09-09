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

// ─────────────────────────────────────────────────────────────────────────────
// ListaHandler  —  CRUD para listas configurables (conductores, actividades…)
// ─────────────────────────────────────────────────────────────────────────────

type ListaHandler struct {
	svc *service.ListaService
}

func NewListaHandler(svc *service.ListaService) *ListaHandler {
	return &ListaHandler{svc: svc}
}

// GET /api/v1/listas?tipo=conductor
func (h *ListaHandler) Listar(c *gin.Context) {
	orgID := middleware.OrganizationID(c)
	tipo := c.Query("tipo")
	if tipo != "" {
		items, err := h.svc.ListarPorTipo(c.Request.Context(), orgID, tipo)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err.Error())
			return
		}
		response.Success(c, http.StatusOK, items)
		return
	}
	// Sin filtro de tipo → todos
	items, err := h.svc.ListarTodos(c.Request.Context(), orgID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, http.StatusOK, items)
}

type listaRequest struct {
	Tipo   string `json:"tipo" binding:"required"`
	Nombre string `json:"nombre" binding:"required"`
	Orden  int    `json:"orden"`
}

// POST /api/v1/listas
func (h *ListaHandler) Crear(c *gin.Context) {
	orgID := middleware.OrganizationID(c)
	var body listaRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Error(c, http.StatusBadRequest, "tipo y nombre son requeridos")
		return
	}
	id, err := h.svc.Crear(c.Request.Context(), orgID, body.Tipo, body.Nombre, body.Orden)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, http.StatusCreated, gin.H{"id": id})
}

type listaUpdateRequest struct {
	Nombre string `json:"nombre" binding:"required"`
	Orden  int    `json:"orden"`
	Activo bool   `json:"activo"`
}

// PUT /api/v1/listas/:id
func (h *ListaHandler) Actualizar(c *gin.Context) {
	orgID := middleware.OrganizationID(c)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "id inválido")
		return
	}
	var body listaUpdateRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Error(c, http.StatusBadRequest, "datos inválidos")
		return
	}
	if err := h.svc.Actualizar(c.Request.Context(), orgID, id, body.Nombre, body.Orden, body.Activo); err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			response.Error(c, http.StatusNotFound, "item no encontrado")
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, http.StatusOK, gin.H{"actualizado": true})
}

// DELETE /api/v1/listas/:id
func (h *ListaHandler) Eliminar(c *gin.Context) {
	orgID := middleware.OrganizationID(c)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "id inválido")
		return
	}
	if err := h.svc.Eliminar(c.Request.Context(), orgID, id); err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			response.Error(c, http.StatusNotFound, "item no encontrado")
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, http.StatusOK, gin.H{"eliminado": true})
}

// ─────────────────────────────────────────────────────────────────────────────
// ProgramacionHandler  —  CRUD de programaciones diarias 15-FR-36
// ─────────────────────────────────────────────────────────────────────────────

type ProgramacionHandler struct {
	svc *service.ProgramacionService
}

func NewProgramacionHandler(svc *service.ProgramacionService) *ProgramacionHandler {
	return &ProgramacionHandler{svc: svc}
}

// GET /api/v1/programaciones
func (h *ProgramacionHandler) Listar(c *gin.Context) {
	orgID := middleware.OrganizationID(c)
	lista, err := h.svc.Listar(c.Request.Context(), orgID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, http.StatusOK, lista)
}

// GET /api/v1/programaciones/ultima
// Devuelve la programación más reciente (con items) para que el frontend
// pueda ofrecer "Copiar programación anterior" al crear una nueva.
func (h *ProgramacionHandler) ObtenerUltima(c *gin.Context) {
	orgID := middleware.OrganizationID(c)
	prog, err := h.svc.ObtenerUltima(c.Request.Context(), orgID)
	if errors.Is(err, apperrors.ErrNotFound) {
		response.Error(c, http.StatusNotFound, "no hay programaciones previas para copiar")
		return
	}
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, http.StatusOK, prog)
}

// GET /api/v1/programaciones/:id
func (h *ProgramacionHandler) Obtener(c *gin.Context) {
	orgID := middleware.OrganizationID(c)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "id inválido")
		return
	}
	prog, err := h.svc.Obtener(c.Request.Context(), orgID, id)
	if errors.Is(err, apperrors.ErrNotFound) {
		response.Error(c, http.StatusNotFound, "programación no encontrada")
		return
	}
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, http.StatusOK, prog)
}

type programacionRequest struct {
	Fecha         string                    `json:"fecha" binding:"required"`
	Observaciones *string                   `json:"observaciones"`
	Items         []models.ProgramacionItem `json:"items"`
}

// POST /api/v1/programaciones
func (h *ProgramacionHandler) Crear(c *gin.Context) {
	orgID := middleware.OrganizationID(c)
	userID := middleware.UserID(c)
	var body programacionRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Error(c, http.StatusBadRequest, "datos inválidos: "+err.Error())
		return
	}
	prog := &models.Programacion{
		OrganizationID: orgID,
		Fecha:          body.Fecha,
		Observaciones:  body.Observaciones,
		CreadoPor:      &userID,
		Items:          body.Items,
	}
	id, err := h.svc.Crear(c.Request.Context(), prog)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, http.StatusCreated, gin.H{"id": id})
}

// PUT /api/v1/programaciones/:id
func (h *ProgramacionHandler) Actualizar(c *gin.Context) {
	orgID := middleware.OrganizationID(c)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "id inválido")
		return
	}
	var body programacionRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Error(c, http.StatusBadRequest, "datos inválidos: "+err.Error())
		return
	}
	prog := &models.Programacion{
		Fecha:         body.Fecha,
		Observaciones: body.Observaciones,
		Items:         body.Items,
	}
	if err := h.svc.Actualizar(c.Request.Context(), orgID, id, prog); err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			response.Error(c, http.StatusNotFound, "programación no encontrada")
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, http.StatusOK, gin.H{"actualizado": true})
}

// DELETE /api/v1/programaciones/:id
func (h *ProgramacionHandler) Eliminar(c *gin.Context) {
	orgID := middleware.OrganizationID(c)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "id inválido")
		return
	}
	if err := h.svc.Eliminar(c.Request.Context(), orgID, id); err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			response.Error(c, http.StatusNotFound, "programación no encontrada")
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, http.StatusOK, gin.H{"eliminado": true})
}
