package handler

import (
	"html"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/alcaldia/sigpa-backend/internal/middleware"
	"github.com/alcaldia/sigpa-backend/internal/models"
	"github.com/alcaldia/sigpa-backend/internal/repository"
	"github.com/alcaldia/sigpa-backend/internal/service"
	"github.com/alcaldia/sigpa-backend/pkg/apperrors"
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
// corporativa configurada (Brevo o SMTP según config).
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

	var body struct {
		TipoDocumento     string `json:"tipo_documento"`
		FechaVencimiento  string `json:"fecha_vencimiento"`
		DestinatarioExtra string `json:"destinatario_extra"`
		Asunto            string `json:"asunto"`
		Cuerpo            string `json:"cuerpo"`
	}
	_ = c.ShouldBindJSON(&body)

	var asunto, cuerpoHTML string

	if body.Asunto != "" || body.Cuerpo != "" {
		asunto = body.Asunto
		cuerpoEscapado := html.EscapeString(body.Cuerpo)
		cuerpoConSaltos := strings.ReplaceAll(cuerpoEscapado, "\n", "<br>")
		contenidoInterno := "<div style=\"white-space:pre-wrap\">" + cuerpoConSaltos + "</div>"
		cuerpoHTML = envolverPlantillaInstitucional("Renovación de documento requerida", contenidoInterno)
	} else {
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
		asunto = "Solicitud de renovación: " + body.TipoDocumento + " — Vehículo " + vehiculo.Placa
		if body.TipoDocumento == "" {
			asunto = "Solicitud de renovación de documento — Vehículo " + vehiculo.Placa
		}
		contenidoInterno := construirTablaDatos(vehiculo.Placa, body.TipoDocumento, vencimiento, dependencia, responsable)
		cuerpoHTML = envolverPlantillaInstitucional("Renovación de documento requerida", contenidoInterno)
	}

	ctx := c.Request.Context()

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

// DELETE /api/v1/vehiculos/:id/documentos/:docId
// Elimina lógicamente un documento (no borra nada de Google Drive ni de la
// base de datos). Solo Administrador puede ejecutar esta acción — se valida
// en el router con middleware.RequireRoles.
func (h *DocumentoHandler) Eliminar(c *gin.Context) {
	orgID := middleware.OrganizationID(c)
	userID := middleware.UserID(c)

	docID, err := strconv.Atoi(c.Param("docId"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "id de documento inválido")
		return
	}

	if err := h.service.EliminarLogico(c.Request.Context(), orgID, docID, userID); err != nil {
		if err == apperrors.ErrNotFound {
			response.Error(c, http.StatusNotFound, "documento no encontrado")
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, http.StatusOK, gin.H{"eliminado": true})
}

// ── Plantilla visual institucional del correo ──────────────────────────────
//
// iconoAlertaSVG: triángulo de alerta vectorial en vez de emojis (⚠️🔔), que
// se renderizan de forma inconsistente entre clientes de correo y suelen
// percibirse como "genéricos". El SVG mantiene siempre el mismo trazo limpio
// y los colores institucionales de Funza.
const iconoAlertaSVG = "<svg width=\"20\" height=\"20\" viewBox=\"0 0 24 24\" xmlns=\"http://www.w3.org/2000/svg\">" +
	"<path d=\"M12 3.5L2.5 20h19L12 3.5Z\" fill=\"#ffffff\"/>" +
	"<rect x=\"11.1\" y=\"9.3\" width=\"1.8\" height=\"5.6\" rx=\"0.9\" fill=\"#DA151C\"/>" +
	"<circle cx=\"12\" cy=\"17\" r=\"1.1\" fill=\"#DA151C\"/>" +
	"</svg>"

// envolverPlantillaInstitucional aplica el diseño visual compartido de SIGPA
// (header rojo con ícono + card blanca con sombra sutil) alrededor de un
// contenido HTML ya preparado, para que todos los correos —editados
// manualmente por el usuario o generados automáticamente— luzcan
// consistentes con la identidad de la Alcaldía de Funza.
func envolverPlantillaInstitucional(tituloHeader, contenidoHTML string) string {
	return "<div style=\"font-family:'Segoe UI',Arial,sans-serif;max-width:600px;margin:0 auto;background:#f4f4f5;padding:24px 0\">" +
		"<div style=\"max-width:560px;margin:0 auto;background:#ffffff;border-radius:12px;overflow:hidden;box-shadow:0 2px 10px rgba(0,0,0,0.06)\">" +
		"<div style=\"background:#DA151C;padding:18px 24px;display:flex;align-items:center;gap:10px\">" +
		"<table role=\"presentation\" style=\"border-collapse:collapse\"><tr>" +
		"<td style=\"vertical-align:middle;padding-right:10px\">" + iconoAlertaSVG + "</td>" +
		"<td style=\"vertical-align:middle\"><span style=\"color:#ffffff;font-size:16px;font-weight:700;letter-spacing:0.01em\">" + tituloHeader + "</span></td>" +
		"</tr></table>" +
		"</div>" +
		"<div style=\"padding:26px 28px;color:#333333;font-size:14px;line-height:1.65\">" +
		contenidoHTML +
		"</div>" +
		"<div style=\"padding:14px 28px;background:#fafafa;border-top:1px solid #eeeeee\">" +
		"<span style=\"font-size:11.5px;color:#9a9a9a\">Este correo fue generado por SIGPA — Sistema Integral de Gestión del Parque Automotor · Alcaldía de Funza</span>" +
		"</div>" +
		"</div></div>"
}

// construirTablaDatos genera la tabla de datos del documento (usada solo en
// el fallback, cuando nadie envió un mensaje personalizado desde el diálogo).
func construirTablaDatos(placa, tipoDocumento, vencimiento, dependencia, responsable string) string {
	fila := func(etiqueta, valor string, colorValor string) string {
		if colorValor == "" {
			colorValor = "#333333"
		}
		return "<tr>" +
			"<td style=\"padding:9px 0;color:#767676;width:38%;font-size:13.5px\">" + etiqueta + "</td>" +
			"<td style=\"padding:9px 0;color:" + colorValor + ";font-weight:600;font-size:13.5px\">" + valor + "</td>" +
			"</tr>"
	}
	return "<table role=\"presentation\" style=\"width:100%;border-collapse:collapse;border-top:1px solid #f0f0f0\">" +
		fila("Vehículo", placa, "#1a1a1a") +
		fila("Documento", tipoDocumento, "") +
		fila("Vencimiento", vencimiento, "#DA151C") +
		fila("Dependencia", dependencia, "") +
		fila("Responsable", responsable, "") +
		"</table>"
}
