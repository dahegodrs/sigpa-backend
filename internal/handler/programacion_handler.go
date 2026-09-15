package handler

import (
	"errors"
	"net/http"
	"strconv"
	"time"

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
	svc             *service.ProgramacionService
	usuarioRepo     *repository.UsuarioRepository
	dependenciaRepo *repository.DependenciaRepository
}

func NewProgramacionHandler(svc *service.ProgramacionService, usuarioRepo *repository.UsuarioRepository, dependenciaRepo *repository.DependenciaRepository) *ProgramacionHandler {
	return &ProgramacionHandler{svc: svc, usuarioRepo: usuarioRepo, dependenciaRepo: dependenciaRepo}
}

// GET /api/v1/programaciones?anio=2026&mes=9
// Si no se envían anio/mes, devuelve todo el histórico (comportamiento
// anterior) — el frontend siempre debería enviarlos desde que existe la
// vista de calendario, para no cargar cientos de registros de una vez.
func (h *ProgramacionHandler) Listar(c *gin.Context) {
	orgID := middleware.OrganizationID(c)
	anio, _ := strconv.Atoi(c.Query("anio"))
	mes, _ := strconv.Atoi(c.Query("mes"))
	lista, err := h.svc.Listar(c.Request.Context(), orgID, anio, mes)
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
	Fecha            string `json:"fecha" binding:"required"`             // "YYYY-MM-DD"
	HoraSolicitada   string `json:"hora_solicitada" binding:"required"`   // "HH:MM"
	HoraFinalizacion string `json:"hora_finalizacion" binding:"required"` // "HH:MM"
	PuntoEncuentro   string `json:"punto_encuentro" binding:"required"`
	Destino          string `json:"destino" binding:"required"`
	Actividad        string `json:"actividad" binding:"required"`
	Motivo           string `json:"motivo" binding:"required"`
	Dependencia      string `json:"dependencia"`
	// TipoVehiculoID es el tipo de vehículo que el solicitante necesita
	// (ej. "Camioneta") — no un vehículo concreto, ya que en este punto
	// todavía no se ha asignado ninguno. Opcional para no romper
	// integraciones existentes, pero se recomienda siempre enviarlo desde
	// el formulario.
	TipoVehiculoID *int `json:"tipo_vehiculo_id"`
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

	// El nombre del solicitante y su dependencia se toman del registro de
	// usuario (no vienen en el JWT). La dependencia se resuelve
	// automáticamente desde el perfil del usuario que hizo la solicitud —
	// así la fila de la programación queda asociada a la dependencia real
	// del solicitante en vez de un valor genérico, sin que el formulario
	// tenga que volver a preguntarlo. Si el usuario no tiene dependencia
	// asignada (o no se pudo resolver), se usa "DISPONIBLE PATIO" como
	// respaldo para no bloquear la solicitud.
	nombreUsuario := email
	dependenciaUsuario := ""
	if u, err := h.usuarioRepo.GetByID(c.Request.Context(), orgID, userID); err == nil {
		nombreUsuario = u.Nombre
		if u.DependenciaID != nil {
			if dep, err := h.dependenciaRepo.GetByID(c.Request.Context(), orgID, *u.DependenciaID); err == nil {
				dependenciaUsuario = dep.Nombre
			}
		}
	}

	var body solicitudVehiculoRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Error(c, http.StatusBadRequest, "todos los campos son requeridos: fecha, hora_solicitada, hora_finalizacion, punto_encuentro, destino, actividad, motivo")
		return
	}

	// La fecha del servicio no puede ser anterior a hoy — se valida en el
	// backend además del frontend, ya que el frontend es fácilmente evadible.
	hoy := time.Now().Format("2006-01-02")
	if body.Fecha < hoy {
		response.Error(c, http.StatusBadRequest, "la fecha del servicio no puede ser anterior a hoy")
		return
	}

	dependencia := dependenciaUsuario
	if dependencia == "" {
		dependencia = body.Dependencia
	}
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
		Conductor:                "DISPONIBLE PATIO",
		VehiculoID:               nil,
		Dependencia:              dependencia,
		Destino:                  body.Destino,
		HoraSalidaPunto:          horaSalidaPunto,
		HoraFinalizacion:         body.HoraFinalizacion,
		Actividad:                body.Actividad,
		EsVacaciones:             false,
		Programado:               false,
		Motivo:                   &motivo,
		Origen:                   "solicitud",
		SolicitanteNombre:        &nombreSolicitante,
		SolicitanteEmail:         &emailSolicitante,
		HoraSolicitada:           &horaSolicitada,
		PuntoEncuentro:           &puntoEncuentro,
		TipoVehiculoSolicitadoID: body.TipoVehiculoID,
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

// GET /api/v1/solicitudes-vehiculo/mias?estado=&anio=&mes=&page=&page_size=
// Lista las solicitudes hechas por el usuario autenticado, para que pueda
// ver el estado (pendiente de asignar / ya confirmada con conductor y
// vehículo). Soporta filtro por estado, por mes/año y paginación — un
// solicitante frecuente puede acumular decenas de registros.
func (h *ProgramacionHandler) MisSolicitudes(c *gin.Context) {
	orgID := middleware.OrganizationID(c)
	email := middleware.Email(c)

	anio, _ := strconv.Atoi(c.Query("anio"))
	mes, _ := strconv.Atoi(c.Query("mes"))
	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("page_size"))

	filtros := repository.FiltrosMisSolicitudes{
		Estado:   c.Query("estado"),
		Anio:     anio,
		Mes:      mes,
		Page:     page,
		PageSize: pageSize,
	}

	items, total, err := h.svc.MisSolicitudes(c.Request.Context(), orgID, email, filtros)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	pageSizeFinal := filtros.PageSize
	if pageSizeFinal < 1 {
		pageSizeFinal = 20
	}
	pageFinal := filtros.Page
	if pageFinal < 1 {
		pageFinal = 1
	}
	totalPages := (total + pageSizeFinal - 1) / pageSizeFinal
	response.SuccessWithMeta(c, http.StatusOK, items, response.Meta{
		Page:       pageFinal,
		PageSize:   pageSizeFinal,
		TotalItems: int64(total),
		TotalPages: totalPages,
	})
}

// PUT /api/v1/programaciones/items/:itemId/aprobar
// Marca una fila de solicitud como aprobada. El envío del correo
// consolidado ocurre cuando el director guarda la programación completa
// (botón "Guardar cambios"), no en este momento — así se agrupan todas las
// decisiones tomadas en la misma sesión de edición en un solo correo.
func (h *ProgramacionHandler) AprobarSolicitud(c *gin.Context) {
	itemID, err := strconv.Atoi(c.Param("itemId"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "id de item inválido")
		return
	}
	if err := h.svc.AprobarItem(c.Request.Context(), itemID); err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			response.Error(c, http.StatusNotFound, "item no encontrado")
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, http.StatusOK, gin.H{"aprobado": true})
}

type rechazarSolicitudRequest struct {
	Motivo string `json:"motivo" binding:"required"`
}

// PUT /api/v1/programaciones/items/:itemId/rechazar
// Marca una fila de solicitud como rechazada con un motivo obligatorio,
// que se le mostrará al solicitante en el correo consolidado y en su
// timeline de "Mis solicitudes".
func (h *ProgramacionHandler) RechazarSolicitud(c *gin.Context) {
	itemID, err := strconv.Atoi(c.Param("itemId"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "id de item inválido")
		return
	}
	var body rechazarSolicitudRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Error(c, http.StatusBadRequest, "el motivo de rechazo es obligatorio")
		return
	}
	if err := h.svc.RechazarItem(c.Request.Context(), itemID, body.Motivo); err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			response.Error(c, http.StatusNotFound, "item no encontrado")
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, http.StatusOK, gin.H{"rechazado": true})
}

// PUT /api/v1/programaciones/items/:itemId/desbloquear
// Permite al director corregir una fila ya notificada (ej. el vehículo se
// dañó y hay que reasignar otro). Es una acción explícita — se le informa
// al director que esto puede requerir volver a notificar al solicitante
// tras el próximo guardado si cambia la decisión de esa fila.
func (h *ProgramacionHandler) DesbloquearSolicitud(c *gin.Context) {
	itemID, err := strconv.Atoi(c.Param("itemId"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "id de item inválido")
		return
	}
	if err := h.svc.DesbloquearItem(c.Request.Context(), itemID); err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			response.Error(c, http.StatusNotFound, "item no encontrado")
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, http.StatusOK, gin.H{"desbloqueado": true})
}

// GET /api/v1/plantillas-correo/solicitud-vehiculo
// Devuelve la plantilla configurable de notificación a solicitantes, para
// que el Administrador la edite desde el Centro de Programación.
func (h *ProgramacionHandler) ObtenerPlantillaSolicitud(c *gin.Context) {
	orgID := middleware.OrganizationID(c)
	asunto, cuerpo := h.svc.ObtenerPlantillaSolicitud(c.Request.Context(), orgID)
	response.Success(c, http.StatusOK, gin.H{"asunto": asunto, "cuerpo_html": cuerpo})
}

type plantillaSolicitudRequest struct {
	Asunto     string `json:"asunto" binding:"required"`
	CuerpoHTML string `json:"cuerpo_html" binding:"required"`
}

// PUT /api/v1/plantillas-correo/solicitud-vehiculo
func (h *ProgramacionHandler) GuardarPlantillaSolicitud(c *gin.Context) {
	orgID := middleware.OrganizationID(c)
	var body plantillaSolicitudRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Error(c, http.StatusBadRequest, "asunto y cuerpo son requeridos")
		return
	}
	if err := h.svc.GuardarPlantillaSolicitud(c.Request.Context(), orgID, body.Asunto, body.CuerpoHTML); err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, http.StatusOK, gin.H{"guardado": true})
}
