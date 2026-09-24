package service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/alcaldia/sigpa-backend/internal/models"
	"github.com/alcaldia/sigpa-backend/internal/repository"
)

// NotificadorEmail abstrae el envío real de correos (implementado en la capa
// de integración con Gmail API). Se define como interfaz aquí para que el
// servicio de alertas sea testeable sin depender de Google.
type NotificadorEmail interface {
	EnviarCorreo(ctx context.Context, destinatario, asunto, cuerpoHTML string) error
}

const asuntoPorDefectoAlertaDocumental = "SIGPA — {{tipo_documento}} próximo a vencer ({{placa}})"
const plantillaPorDefectoAlertaDocumental = `Cordial saludo,

Por medio del presente correo, desde el área de Patio y Parque Automotor de la Alcaldía de Funza se hace el recordatorio formal de que el documento {{tipo_documento}} del vehículo con placa {{placa}}, asignado a {{dependencia}}, se encuentra {{estado_alerta_texto}} con fecha de vencimiento el {{fecha_vencimiento}}.

El vencimiento está previsto {{dias_para_vencer_texto}}.

Se solicita gestionar con carácter urgente la renovación de este documento para garantizar la legalidad y correcta operación del vehículo.

Por favor confirmar la recepción de este mensaje y las acciones a tomar.

Atentamente,

Patio y Parque Automotor
Alcaldía de Funza — Cundinamarca
{{correo_contacto}}`

type AlertaService struct {
	alertaRepo      *repository.AlertaRepository
	documentoRepo   *repository.DocumentoRepository
	documentoSvc    *DocumentoService
	vehiculoRepo    *repository.VehiculoRepository
	usuarioRepo     *repository.UsuarioRepository
	plantillaRepo   *repository.PlantillaCorreoRepository
	dependenciaRepo *repository.DependenciaRepository
	notificador     NotificadorEmail
}

func NewAlertaService(
	alertaRepo *repository.AlertaRepository,
	documentoRepo *repository.DocumentoRepository,
	documentoSvc *DocumentoService,
	vehiculoRepo *repository.VehiculoRepository,
	usuarioRepo *repository.UsuarioRepository,
	plantillaRepo *repository.PlantillaCorreoRepository,
	dependenciaRepo *repository.DependenciaRepository,
	notificador NotificadorEmail,
) *AlertaService {
	return &AlertaService{
		alertaRepo:      alertaRepo,
		documentoRepo:   documentoRepo,
		documentoSvc:    documentoSvc,
		vehiculoRepo:    vehiculoRepo,
		usuarioRepo:     usuarioRepo,
		plantillaRepo:   plantillaRepo,
		dependenciaRepo: dependenciaRepo,
		notificador:     notificador,
	}
}

// EjecutarRevisionDiaria es el corazón del módulo de alertas. Se ejecuta una
// vez al día (ver cron en cmd/api/main.go) y hace lo siguiente:
//  1. Recalcula el estado documental de todos los documentos vigentes.
//  2. Por cada documento, calcula días restantes para vencer.
//  3. Si los días restantes coinciden con un umbral configurado (45/30/15/7/1/0),
//     y no se ha generado ya una alerta de ese nivel hoy, crea la alerta.
//  4. Envía las alertas pendientes por el canal configurado (Email por defecto).
func (s *AlertaService) EjecutarRevisionDiaria(ctx context.Context) error {
	actualizados, err := s.documentoSvc.RecalcularEstadosDiario(ctx)
	if err != nil {
		return err
	}
	log.Printf("Revisión diaria: %d documentos con estado actualizado", actualizados)

	documentos, err := s.documentoRepo.ListarParaRevisionDiaria(ctx)
	if err != nil {
		return err
	}

	ahora := time.Now()
	generadas := 0

	for _, d := range documentos {
		if d.FechaVencimiento == nil {
			continue
		}
		diasRestantes := models.DiasHasta(*d.FechaVencimiento, ahora)

		config, err := s.alertaRepo.ObtenerConfigActiva(ctx, d.OrganizationID)
		if err != nil {
			log.Printf("error al obtener config de alertas org=%d: %v", d.OrganizationID, err)
			continue
		}

		for _, c := range config {
			// Vencido (dias_antes = 0): alerta crítica diaria mientras siga vencido.
			coincide := (c.DiasAntes == 0 && diasRestantes < 0) || (diasRestantes == c.DiasAntes)
			if !coincide {
				continue
			}

			yaExiste, err := s.alertaRepo.YaExisteAlertaHoy(ctx, d.ID, c.Nivel)
			if err != nil || yaExiste {
				continue
			}

			destinatarios := s.resolverDestinatarios(ctx, d, c.Nivel)
			for _, destino := range destinatarios {
				alertaID, err := s.alertaRepo.Crear(ctx, &models.Alerta{
					OrganizationID: d.OrganizationID,
					VehiculoID:     d.VehiculoID,
					DocumentoID:    d.ID,
					TipoAlerta:     c.Nivel,
					Canal:          "Email",
					Destinatario:   destino,
				})
				if err != nil {
					log.Printf("error al crear alerta documento=%d: %v", d.ID, err)
					continue
				}
				generadas++
				_ = alertaID
			}
		}
	}

	log.Printf("Revisión diaria: %d alertas generadas", generadas)
	return s.EnviarPendientes(ctx)
}

// resolverDestinatarios determina a quién notificar según el nivel de alerta:
//   - Preventiva / Importante / Prioritaria: responsable del vehículo +
//     usuarios de la dependencia asignada.
//   - Urgente / Crítica: lo anterior, más Administrador y Gerencia de la
//     organización, tal como lo pide el documento maestro del proyecto
//     ("Gerencia o jefe de transporte cuando el documento esté vencido").
func (s *AlertaService) resolverDestinatarios(ctx context.Context, d models.Documento, nivel string) []string {
	vistos := map[string]bool{}
	var destinatarios []string
	agregar := func(email string) {
		if email == "" || vistos[email] {
			return
		}
		vistos[email] = true
		destinatarios = append(destinatarios, email)
	}

	dependenciaID, emailResponsable, err := s.vehiculoRepo.ObtenerDependenciaYResponsable(ctx, d.VehiculoID)
	if err != nil {
		log.Printf("no se pudo resolver responsable/dependencia del vehículo %d: %v", d.VehiculoID, err)
	} else {
		if emailResponsable != nil {
			agregar(*emailResponsable)
		}
		if dependenciaID != nil {
			emails, err := s.usuarioRepo.EmailsPorDependencia(ctx, d.OrganizationID, *dependenciaID)
			if err != nil {
				log.Printf("no se pudo resolver emails de dependencia %d: %v", *dependenciaID, err)
			}
			for _, e := range emails {
				agregar(e)
			}
		}
	}

	if nivel == "Urgente" || nivel == "Critica" || nivel == "Crítica" {
		emails, err := s.usuarioRepo.EmailsPorRoles(ctx, d.OrganizationID, []string{"Administrador", "Gerencia"})
		if err != nil {
			log.Printf("no se pudo resolver emails de Administrador/Gerencia org=%d: %v", d.OrganizationID, err)
		}
		for _, e := range emails {
			agregar(e)
		}
	}

	return destinatarios
}

// EnviarPendientes procesa la cola de alertas pendientes y las envía por Gmail API.
func (s *AlertaService) EnviarPendientes(ctx context.Context) error {
	pendientes, err := s.alertaRepo.ListarPendientes(ctx)
	if err != nil {
		return err
	}

	for _, a := range pendientes {
		asunto, cuerpo := s.construirCorreoAlertaDocumental(ctx, a)

		err := s.notificador.EnviarCorreo(ctx, a.Destinatario, asunto, cuerpo)
		var detalleError *string
		if err != nil {
			msg := err.Error()
			detalleError = &msg
		}
		if err := s.alertaRepo.MarcarResultado(ctx, a.ID, err == nil, detalleError); err != nil {
			log.Printf("error al marcar resultado de alerta %d: %v", a.ID, err)
		}
	}
	return nil
}

func (s *AlertaService) construirCorreoAlertaDocumental(ctx context.Context, a models.Alerta) (string, string) {
	asunto := asuntoPorDefectoAlertaDocumental
	cuerpo := plantillaPorDefectoAlertaDocumental

	if s.plantillaRepo != nil {
		if p, err := s.plantillaRepo.ObtenerPorTipo(ctx, a.OrganizationID, "alerta_documental"); err == nil {
			asunto = p.Asunto
			cuerpo = p.CuerpoHTML
		}
	}

	fechaVenc := ""
	if a.DocumentoFechaVenc != nil {
		fechaVenc = a.DocumentoFechaVenc.In(time.FixedZone("COT", -5*60*60)).Format("02 de January de 2006")
	}
	dias := ""
	if a.DocumentoFechaVenc != nil {
		d := models.DiasHasta(*a.DocumentoFechaVenc, time.Now())
		if d < 0 {
			dias = fmt.Sprintf("hace %d días", -d)
		} else if d == 0 {
			dias = "hoy"
		} else {
			dias = fmt.Sprintf("en %d días", d)
		}
	}

	estadoTexto := "próximo a vencer"
	if strings.EqualFold(a.TipoAlerta, "Critica") || strings.EqualFold(a.TipoAlerta, "Crítica") {
		estadoTexto = "vencido"
	}

	dependencia := "Dependencia no asignada"
	if depID, _, err := s.vehiculoRepo.ObtenerDependenciaYResponsable(ctx, a.VehiculoID); err == nil && depID != nil {
		if dep, err := s.obtenerNombreDependencia(ctx, a.OrganizationID, *depID); err == nil && dep != "" {
			dependencia = dep
		}
	}

	replacer := strings.NewReplacer(
		"{{placa}}", a.VehiculoPlaca,
		"{{tipo_documento}}", a.TipoDocumentoNombre,
		"{{dependencia}}", dependencia,
		"{{fecha_vencimiento}}", fechaVenc,
		"{{dias_para_vencer}}", dias,
		"{{dias_para_vencer_texto}}", dias,
		"{{estado_alerta}}", a.TipoAlerta,
		"{{estado_alerta_texto}}", estadoTexto,
		"{{correo_contacto}}", "Patio@funza-cundinamarca.gov.co",
	)
	return replacer.Replace(asunto), replacer.Replace(cuerpo)
}

func (s *AlertaService) obtenerNombreDependencia(ctx context.Context, organizationID, dependenciaID int) (string, error) {
	if s.dependenciaRepo == nil {
		return "", fmt.Errorf("dependenciaRepo no configurado")
	}
	dep, err := s.dependenciaRepo.GetByID(ctx, organizationID, dependenciaID)
	if err != nil {
		return "", err
	}
	return dep.Nombre, nil
}

func (s *AlertaService) ListarPorOrganizacion(ctx context.Context, organizationID int, soloPendientes bool) ([]models.Alerta, error) {
	return s.alertaRepo.ListarPorOrganizacion(ctx, organizationID, soloPendientes)
}

func (s *AlertaService) MarcarLeida(ctx context.Context, organizationID, id int) error {
	return s.alertaRepo.MarcarLeida(ctx, organizationID, id)
}

func (s *AlertaService) MarcarTodasLeidas(ctx context.Context, organizationID int) error {
	return s.alertaRepo.MarcarTodasLeidas(ctx, organizationID)
}

// --- Reglas de notificación (config_alertas) ---

func (s *AlertaService) ListarConfig(ctx context.Context, organizationID int) ([]models.ConfigAlerta, error) {
	return s.alertaRepo.ListarConfig(ctx, organizationID)
}

func (s *AlertaService) CrearConfig(ctx context.Context, c *models.ConfigAlerta) (int, error) {
	return s.alertaRepo.CrearConfig(ctx, c)
}

func (s *AlertaService) ActualizarConfig(ctx context.Context, organizationID, id, diasAntes int, nivel string, activo bool) error {
	return s.alertaRepo.ActualizarConfig(ctx, organizationID, id, diasAntes, nivel, activo)
}

func (s *AlertaService) EliminarConfig(ctx context.Context, organizationID, id int) error {
	return s.alertaRepo.EliminarConfig(ctx, organizationID, id)
}
