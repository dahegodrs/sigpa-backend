package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/alcaldia/sigpa-backend/internal/middleware"
	"github.com/alcaldia/sigpa-backend/internal/models"
	"github.com/alcaldia/sigpa-backend/internal/repository"
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
	svc         *service.ProgramacionService
	usuarioRepo *repository.UsuarioRepository
}

func NewProgramacionHandler(svc *service.ProgramacionService, usuarioRepo *repository.UsuarioRepository) *ProgramacionHandler {
	return &ProgramacionHandler{svc: svc, usuarioRepo: usuarioRepo}
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

// ─────────────────────────────────────────────────────────────────────────────
// Solicitud de vehículo — formulario para dependencias/secretarías (reemplaza
// el flujo actual por correo electrónico).
// ─────────────────────────────────────────────────────────────────────────────

type solicitudVehiculoRequest struct {
	Fecha          string `json:"fecha" binding:"required"`           // "YYYY-MM-DD"
	HoraSolicitada string `json:"hora_solicitada" binding:"required"` // "HH:MM"
	PuntoEncuentro string `json:"punto_encuentro" binding:"required"`
	Destino        string `json:"destino" binding:"required"`
	Actividad      string `json:"actividad" binding:"required"`
	Motivo         string `json:"motivo" binding:"required"`
	Dependencia    string `json:"dependencia"`
}

// POST /api/v1/solicitudes-vehiculo
// Cualquier usuario autenticado (típicamente rol "Solicitante") puede pedir
// un vehículo. El backend busca o crea automáticamente la programación del
// día indicado, y agrega una fila SIN conductor/vehículo asignado —el
// director la completa y confirma después desde "Editar Programación".
func (h *ProgramacionHandler) SolicitarVehiculo(c *gin.Context) {
	orgID := middleware.OrganizationID(c)
	userID := middleware.UserID(c)
	email := middleware.Email(c)

	// El nombre del solicitante se toma del registro de usuario (no viene
	// en el JWT) — se usa para mostrar "quién pidió el servicio" en la
	// programación sin depender de que el formulario lo vuelva a pedir.
	nombreUsuario := email
	if u, err := h.usuarioRepo.GetByID(c.Request.Context(), orgID, userID); err == nil {
		nombreUsuario = u.Nombre
	}

	var body solicitudVehiculoRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Error(c, http.StatusBadRequest, "todos los campos son requeridos: fecha, hora_solicitada, punto_encuentro, destino, actividad, motivo")
		return
	}

	dependencia := body.Dependencia
	if dependencia == "" {
		dependencia = "DISPONIBLE PATIO"
	}

	// El texto compuesto que ya consumen la vista de "Editar Programación"
	// y el PDF (hora_salida_punto) se genera a partir de los datos crudos
	// del formulario, para no tener que tocar esa lógica ya construida.
	horaSalidaPunto := body.HoraSolicitada + " - " + body.PuntoEncuentro

	motivo := body.Motivo
	puntoEncuentro := body.PuntoEncuentro
	horaSolicitada := body.HoraSolicitada
	nombreSolicitante := nombreUsuario
	emailSolicitante := email

	item := &models.ProgramacionItem{
		Conductor:         "DISPONIBLE PATIO",
		VehiculoID:        nil,
		Dependencia:       dependencia,
		Destino:           body.Destino,
		HoraSalidaPunto:   horaSalidaPunto,
		HoraFinalizacion:  "DISPONIBLE PATIO",
		Actividad:         body.Actividad,
		EsVacaciones:      false,
		Programado:        false,
		Motivo:            &motivo,
		Origen:            "solicitud",
		SolicitanteNombre: &nombreSolicitante,
		SolicitanteEmail:  &emailSolicitante,
		HoraSolicitada:    &horaSolicitada,
		PuntoEncuentro:    &puntoEncuentro,
	}

	programacionID, itemID, err := h.svc.SolicitarVehiculo(c.Request.Context(), orgID, userID, item, body.Fecha)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "no se pudo registrar la solicitud: "+err.Error())
		return
	}

	response.Success(c, http.StatusCreated, gin.H{
		"programacion_id": programacionID,
		"item_id":         itemID,
	})
}

// GET /api/v1/solicitudes-vehiculo/mias
// Lista las solicitudes hechas por el usuario autenticado, para que pueda
// ver el estado (pendiente de asignar / ya confirmada con conductor y
// vehículo).
func (h *ProgramacionHandler) MisSolicitudes(c *gin.Context) {
	orgID := middleware.OrganizationID(c)
	email := middleware.Email(c)

	items, err := h.svc.MisSolicitudes(c.Request.Context(), orgID, email)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, http.StatusOK, items)
}
