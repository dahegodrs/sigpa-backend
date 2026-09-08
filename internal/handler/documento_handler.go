package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/alcaldia/sigpa-backend/internal/middleware"
	"github.com/alcaldia/sigpa-backend/internal/models"
	"github.com/alcaldia/sigpa-backend/internal/repository"
	"github.com/alcaldia/sigpa-backend/internal/service"
	"github.com/alcaldia/sigpa-backend/pkg/response"
)

type DocumentoHandler struct {
	service      *service.DocumentoService
	vehiculoRepo *repository.VehiculoRepository
	catalogoRepo *repository.CatalogoRepository
	drive        service.DriveUploader
	notificador  service.NotificadorEmail
	smtpSender   string // email de la cuenta remitente (SMTP_USER)
}

func NewDocumentoHandler(s *service.DocumentoService, vehiculoRepo *repository.VehiculoRepository, catalogoRepo *repository.CatalogoRepository, drive service.DriveUploader, notificador service.NotificadorEmail, smtpSender string) *DocumentoHandler {
	return &DocumentoHandler{service: s, vehiculoRepo: vehiculoRepo, catalogoRepo: catalogoRepo, drive: drive, notificador: notificador, smtpSender: smtpSender}
}

// GET /api/v1/vehiculos/:id/documentos
func (h *DocumentoHandler) ListarPorVehiculo(c *gin.Context) {
	orgID := middleware.OrganizationID(c)
	vehiculoID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "id de vehículo inválido")
		return
	}

	documentos, err := h.service.ListarPorVehiculo(c.Request.Context(), orgID, vehiculoID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, http.StatusOK, documentos)
}

// GET /api/v1/vehiculos/:id/documentos/:tipoId/historico
func (h *DocumentoHandler) Historico(c *gin.Context) {
	orgID := middleware.OrganizationID(c)
	vehiculoID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "id de vehículo inválido")
		return
	}
	tipoID, err := strconv.Atoi(c.Param("tipoId"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "id de tipo de documento inválido")
		return
	}

	historico, err := h.service.HistoricoPorTipo(c.Request.Context(), orgID, vehiculoID, tipoID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, http.StatusOK, historico)
}

// POST /api/v1/vehiculos/:id/documentos/upload
// Recibe el archivo como multipart/form-data (campo "archivo"), lo sube a la
// carpeta de Drive correspondiente a la placa del vehículo (creándola si no
// existe) y registra la nueva versión del documento con el enlace resultante.
func (h *DocumentoHandler) SubirNuevaVersion(c *gin.Context) {
	orgID := middleware.OrganizationID(c)
	userID := middleware.UserID(c)
	vehiculoID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "id de vehículo inválido")
		return
	}

	tipoDocumentoID, err := strconv.Atoi(c.PostForm("tipo_documento_id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "tipo_documento_id inválido")
		return
	}

	fileHeader, err := c.FormFile("archivo")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "el archivo es obligatorio (campo 'archivo')")
		return
	}
	archivo, err := fileHeader.Open()
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "no se pudo leer el archivo recibido")
		return
	}
	defer archivo.Close()

	vehiculo, err := h.vehiculoRepo.GetByID(c.Request.Context(), orgID, vehiculoID)
	if err != nil {
		response.Error(c, http.StatusNotFound, "vehículo no encontrado")
		return
	}

	// Resolver el nombre del tipo de documento para usarlo como nombre de carpeta en Drive.
	// Si falla (tipo no encontrado o error), se usa la placa sola como fallback.
	tipoNombre, err := h.catalogoRepo.ObtenerNombreTipoDocumento(c.Request.Context(), tipoDocumentoID)
	var carpetaID string
	if err != nil || tipoNombre == "" {
		// Fallback: carpeta plana por placa (comportamiento anterior)
		carpetaID, err = h.drive.ObtenerOCrearCarpetaVehiculo(c.Request.Context(), vehiculo.Placa)
	} else {
		// Estructura por tipo: RootFolder → [tipoDocumento] → [placa]
		carpetaID, err = h.drive.ObtenerOCrearRutaDocumento(c.Request.Context(), tipoNombre, vehiculo.Placa)
	}
	if err != nil {
		response.Error(c, http.StatusBadGateway, "error al preparar la carpeta en Google Drive: "+err.Error())
		return
	}

	mimeType := fileHeader.Header.Get("Content-Type")
	driveFileID, webViewLink, err := h.drive.SubirArchivo(c.Request.Context(), carpetaID, fileHeader.Filename, mimeType, archivo)
	if err != nil {
		response.Error(c, http.StatusBadGateway, "error al subir el archivo a Google Drive: "+err.Error())
		return
	}

	d := models.Documento{
		OrganizationID:  orgID,
		VehiculoID:      vehiculoID,
		TipoDocumentoID: tipoDocumentoID,
		ArchivoURL:      &webViewLink,
		ArchivoDriveID:  &driveFileID,
		NombreArchivo:   &fileHeader.Filename,
		CargadoPor:      &userID,
	}
	tamano := fileHeader.Size
	d.TamanoBytes = &tamano
	if fechaExp := c.PostForm("fecha_expedicion"); fechaExp != "" {
		if parsed, err := parseFechaISO(fechaExp); err == nil {
			d.FechaExpedicion = &parsed
		}
	}
	if fechaVenc := c.PostForm("fecha_vencimiento"); fechaVenc != "" {
		if parsed, err := parseFechaISO(fechaVenc); err == nil {
			d.FechaVencimiento = &parsed
		}
	}
	if obs := c.PostForm("observaciones"); obs != "" {
		d.Observaciones = &obs
	}

	newID, err := h.service.SubirNuevaVersion(c.Request.Context(), &d, userID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, http.StatusCreated, gin.H{"id": newID, "archivo_url": webViewLink})
}

func parseFechaISO(valor string) (time.Time, error) {
	return time.Parse("2006-01-02", valor)
}

// GET /api/v1/documentos
// Gestión Documental global: todos los documentos de todos los vehículos de
// la organización, con filtros por tipo, estado, placa y dependencia.
func (h *DocumentoHandler) ListarGlobal(c *gin.Context) {
	orgID := middleware.OrganizationID(c)

	f := repository.DocumentoFiltroGlobal{OrganizationID: orgID, Placa: c.Query("placa")}
	if v := c.Query("tipo_documento_id"); v != "" {
		if id, err := strconv.Atoi(v); err == nil {
			f.TipoDocumentoID = &id
		}
	}
	if v := c.Query("estado_documento"); v != "" {
		f.EstadoDocumento = &v
	}
	if v := c.Query("dependencia_id"); v != "" {
		if id, err := strconv.Atoi(v); err == nil {
			f.DependenciaID = &id
		}
	}
	f.Page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	f.PageSize, _ = strconv.Atoi(c.DefaultQuery("page_size", "24"))

	documentos, total, err := h.service.ListarGlobal(c.Request.Context(), f)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	totalPages := int((total + int64(f.PageSize) - 1) / int64(f.PageSize))
	response.SuccessWithMeta(c, http.StatusOK, documentos, response.Meta{
		Page: f.Page, PageSize: f.PageSize, TotalItems: total, TotalPages: totalPages,
	})
}

// POST /api/v1/vehiculos/:id/documentos/:docId/notificar
// Envía un correo de notificación de renovación de documento usando la cuenta
// corporativa configurada en SMTP_USER (Patio@funza-cundinamarca.gov.co).
func (h *DocumentoHandler) NotificarDocumento(c *gin.Context) {
	orgID := middleware.OrganizationID(c)
	vehiculoID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "id de vehículo inválido")
		return
	}

	vehiculo, err := h.vehiculoRepo.GetByID(c.Request.Context(), orgID, vehiculoID)
	if err != nil {
		response.Error(c, http.StatusNotFound, "vehículo no encontrado")
		return
	}

	// Datos del documento desde el body (opcionales — pueden venir del frontend)
	var body struct {
		TipoDocumento     string `json:"tipo_documento"`
		FechaVencimiento  string `json:"fecha_vencimiento"`
		DestinatarioExtra string `json:"destinatario_extra"` // correo adicional (opcional)
	}
	_ = c.ShouldBindJSON(&body)

	// Construir asunto y cuerpo del correo
	asunto := "🔔 Renovación requerida: " + body.TipoDocumento + " — Vehículo " + vehiculo.Placa
	if body.TipoDocumento == "" {
		asunto = "🔔 Renovación de documento requerida — Vehículo " + vehiculo.Placa
	}

	vencimiento := body.FechaVencimiento
	if vencimiento == "" {
		vencimiento = "Sin fecha registrada"
	}
	dependencia := vehiculo.DependenciaNombre
	if dependencia == "" {
		dependencia = "Sin asignar"
	}
	responsable := vehiculo.ResponsableNombre
	if responsable == "" {
		responsable = "Sin asignar"
	}

	cuerpoHTML := "<div style='font-family:Arial,sans-serif;max-width:600px;margin:0 auto'>" +
		"<div style='background:#DA151C;padding:20px 24px;border-radius:8px 8px 0 0'>" +
		"<h2 style='color:#fff;margin:0;font-size:18px'>⚠️ Renovación de documento requerida</h2>" +
		"</div>" +
		"<div style='background:#f9f9f9;padding:24px;border:1px solid #e0e0e0;border-radius:0 0 8px 8px'>" +
		"<table style='width:100%;border-collapse:collapse'>" +
		"<tr><td style='padding:8px 0;color:#666;width:40%'><strong>Vehículo:</strong></td><td style='padding:8px 0;font-weight:700;color:#333'>" + vehiculo.Placa + "</td></tr>" +
		"<tr><td style='padding:8px 0;color:#666'><strong>Documento:</strong></td><td style='padding:8px 0;color:#333'>" + body.TipoDocumento + "</td></tr>" +
		"<tr><td style='padding:8px 0;color:#666'><strong>Vencimiento:</strong></td><td style='padding:8px 0;color:#DA151C;font-weight:700'>" + vencimiento + "</td></tr>" +
		"<tr><td style='padding:8px 0;color:#666'><strong>Dependencia:</strong></td><td style='padding:8px 0;color:#333'>" + dependencia + "</td></tr>" +
		"<tr><td style='padding:8px 0;color:#666'><strong>Responsable:</strong></td><td style='padding:8px 0;color:#333'>" + responsable + "</td></tr>" +
		"</table>" +
		"<p style='margin-top:20px;font-size:13px;color:#888'>Este correo fue generado automáticamente por SIGPA — Sistema Integral de Gestión del Parque Automotor · Alcaldía de Funza.</p>" +
		"</div></div>"

	ctx := c.Request.Context()

	// Determinar destinatario principal
	// Si el frontend envió un destinatario explícito, usar ese;
	// si no, enviar a la propia cuenta del patio como registro interno.
	destinatarioPrincipal := body.DestinatarioExtra
	if destinatarioPrincipal == "" {
		destinatarioPrincipal = h.smtpSender
		if destinatarioPrincipal == "" {
			destinatarioPrincipal = "Patio@funza-cundinamarca.gov.co"
		}
	}

	if err := h.notificador.EnviarCorreo(ctx, destinatarioPrincipal, asunto, cuerpoHTML); err != nil {
		response.Error(c, http.StatusInternalServerError, "error al enviar el correo: "+err.Error())
		return
	}

	response.Success(c, http.StatusOK, gin.H{"enviado": true})
}

// GET /api/v1/documentos/conteo-por-tipo
// Alimenta las tarjetas de "carpetas" (SOAT, Tecnomecánica, etc.) con
// cantidad real de archivos y tamaño total ocupado.
func (h *DocumentoHandler) ConteoPorTipo(c *gin.Context) {
	orgID := middleware.OrganizationID(c)
	conteo, err := h.service.ConteoPorTipo(c.Request.Context(), orgID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, http.StatusOK, conteo)
}
