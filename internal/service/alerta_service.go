package service

import (
	"context"
	"log"
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

type AlertaService struct {
	alertaRepo    *repository.AlertaRepository
	documentoRepo *repository.DocumentoRepository
	documentoSvc  *DocumentoService
	vehiculoRepo  *repository.VehiculoRepository
	usuarioRepo   *repository.UsuarioRepository
	notificador   NotificadorEmail
}

func NewAlertaService(
	alertaRepo *repository.AlertaRepository,
	documentoRepo *repository.DocumentoRepository,
	documentoSvc *DocumentoService,
	vehiculoRepo *repository.VehiculoRepository,
	usuarioRepo *repository.UsuarioRepository,
	notificador NotificadorEmail,
) *AlertaService {
	return &AlertaService{
		alertaRepo:    alertaRepo,
		documentoRepo: documentoRepo,
		documentoSvc:  documentoSvc,
		vehiculoRepo:  vehiculoRepo,
		usuarioRepo:   usuarioRepo,
		notificador:   notificador,
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
		asunto := "Alerta SIGPA: documento vehicular " + a.TipoAlerta
		cuerpo := "Se ha generado una alerta de nivel " + a.TipoAlerta + " para el vehículo asociado. Ingrese a SIGPA para más detalles."

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
