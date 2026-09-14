package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/alcaldia/sigpa-backend/internal/models"
	"github.com/alcaldia/sigpa-backend/internal/repository"
	"github.com/alcaldia/sigpa-backend/pkg/apperrors"
)

// plantillaPorDefectoSolicitud es el CUERPO EDITABLE (texto plano, sin
// HTML) que el Administrador ve y edita desde el diálogo de "Plantilla de
// notificación". El backend lo envuelve siempre con el wrapper HTML fijo
// (envolverEnPlantillaHTML) antes de enviarlo — así el usuario nunca ve ni
// tiene que entender etiquetas <div>/<style>, igual que la plantilla de
// renovación de documentos.
const plantillaPorDefectoSolicitud = `Hola {{nombre_solicitante}},

Te informamos el resultado de tu solicitud de vehículo para el día {{fecha_servicio}}:

{{detalle_aprobadas}}
{{detalle_rechazadas}}

Cualquier duda, comunícate con el área de Patio y Parque Automotor.

Atentamente,
Patio y Parque Automotor
Alcaldía de Funza — Cundinamarca`

const asuntoPorDefectoSolicitud = "Respuesta a tu solicitud de vehículo — {{fecha_servicio}}"

// envolverEnPlantillaHTML aplica el diseño institucional fijo (header rojo
// con esquinas superiores redondeadas, cuerpo blanco) alrededor del texto
// plano que escribió el Administrador. Los saltos de línea del texto se
// convierten a <br> porque el cuerpo del correo es HTML aunque el editor
// sea de texto plano.
func envolverEnPlantillaHTML(textoPlano string) string {
	cuerpoHTML := strings.ReplaceAll(textoPlano, "\n", "<br>")
	return fmt.Sprintf(`<div style="font-family: Segoe UI, Arial, sans-serif; max-width: 600px; margin: 0 auto; color: #222; border-radius: 12px; overflow: hidden; box-shadow: 0 2px 8px rgba(0,0,0,0.08);">
<div style="background-color: #DA151C; padding: 24px; text-align: center; border-radius: 12px 12px 0 0;">
<h2 style="color: #fff; margin: 0; font-size: 18px;">Alcaldía de Funza</h2>
</div>
<div style="padding: 24px; background-color: #ffffff; line-height: 1.6;">
%s
</div>
</div>`, cuerpoHTML)
}

type ProgramacionService struct {
	repo          *repository.ProgramacionRepository
	plantillaRepo *repository.PlantillaCorreoRepository
	notificador   NotificadorEmail
}

func NewProgramacionService(repo *repository.ProgramacionRepository, plantillaRepo *repository.PlantillaCorreoRepository, notificador NotificadorEmail) *ProgramacionService {
	return &ProgramacionService{repo: repo, plantillaRepo: plantillaRepo, notificador: notificador}
}

func (s *ProgramacionService) Listar(ctx context.Context, organizationID, anio, mes int) ([]models.Programacion, error) {
	return s.repo.Listar(ctx, organizationID, anio, mes)
}

func (s *ProgramacionService) Obtener(ctx context.Context, organizationID, id int) (*models.Programacion, error) {
	return s.repo.Obtener(ctx, organizationID, id)
}

// ObtenerUltima expone la programación más reciente para "Copiar programación anterior".
func (s *ProgramacionService) ObtenerUltima(ctx context.Context, organizationID int) (*models.Programacion, error) {
	return s.repo.ObtenerUltima(ctx, organizationID)
}

func (s *ProgramacionService) Crear(ctx context.Context, p *models.Programacion) (int, error) {
	return s.repo.Crear(ctx, p)
}

// Actualizar guarda toda la programación y, si hay decisiones nuevas de
// solicitudes (aprobadas/rechazadas) sin notificar, envía el correo
// consolidado por solicitante después de persistir los cambios. El envío
// no bloquea ni hace fallar el guardado si algo sale mal — se registra en
// el log y el director puede seguir trabajando normalmente.
//
// Antes de guardar, se preservan las filas ya notificadas: una vez que se
// le avisó por correo al solicitante que su vehículo fue aprobado/
// rechazado, esa fila queda "congelada" (todos sus campos) para no perder
// trazabilidad — si el frontend intenta enviar un valor distinto para una
// fila bloqueada, se ignora y se conserva el original tal como estaba en
// la base de datos. La única forma de modificarla es que el director la
// desbloquee explícitamente primero (endpoint Desbloquear).
func (s *ProgramacionService) Actualizar(ctx context.Context, organizationID, id int, p *models.Programacion) error {
	actual, err := s.repo.Obtener(ctx, organizationID, id)
	if err == nil {
		bloqueadas := map[int]models.ProgramacionItem{}
		for _, it := range actual.Items {
			if it.NotificadoEn != nil {
				bloqueadas[it.ID] = it
			}
		}
		if len(bloqueadas) > 0 {
			for i, it := range p.Items {
				if original, existe := bloqueadas[it.ID]; existe {
					// Se conserva la fila original completa — el
					// frontend puede haber enviado cambios, pero al
					// estar notificada no se le permite modificarla.
					p.Items[i] = original
				}
			}
		}
	}

	if err := s.repo.Actualizar(ctx, organizationID, id, p); err != nil {
		return err
	}
	go s.notificarDecisionesPendientes(context.Background(), organizationID, id)
	return nil
}

// DesbloquearItem limpia notificado_en de una fila ya notificada, para que
// el director pueda corregirla (ej. el vehículo se dañó y hay que
// reasignar otro). Es una acción explícita — no ocurre automáticamente —
// para que quede claro que fue una corrección intencional después de ya
// haber avisado al solicitante.
func (s *ProgramacionService) DesbloquearItem(ctx context.Context, itemID int) error {
	return s.repo.Desbloquear(ctx, itemID)
}

// AprobarItem marca una fila de solicitud como aprobada (el director ya
// asignó vehículo/conductor y confirmó el servicio).
func (s *ProgramacionService) AprobarItem(ctx context.Context, itemID int) error {
	return s.repo.ActualizarEstadoItem(ctx, itemID, "aprobada", nil)
}

// RechazarItem marca una fila de solicitud como rechazada, con un motivo
// obligatorio que se le mostrará al solicitante en el correo y en su
// timeline de "Mis solicitudes".
func (s *ProgramacionService) RechazarItem(ctx context.Context, itemID int, motivo string) error {
	return s.repo.ActualizarEstadoItem(ctx, itemID, "rechazada", &motivo)
}

// notificarDecisionesPendientes agrupa por solicitante_email todas las
// filas de la programación que tienen una decisión tomada y aún no se les
// ha notificado, y envía UN SOLO correo por solicitante con el detalle de
// todo lo aprobado/rechazado — así si alguien pidió 3 vehículos el mismo
// día y el director resolvió 2 en esta sesión, sale un único correo con
// ambas decisiones (y una tercera solicitud pendiente simplemente no se
// menciona todavía, porque sigue en estado 'pendiente').
func (s *ProgramacionService) notificarDecisionesPendientes(ctx context.Context, organizationID, programacionID int) {
	items, err := s.repo.ListarDecisionesSinNotificar(ctx, programacionID)
	if err != nil {
		log.Printf("error al listar decisiones sin notificar (programacion=%d): %v", programacionID, err)
		return
	}
	if len(items) == 0 {
		return
	}

	// Agrupar por email del solicitante.
	grupos := map[string][]models.ProgramacionItem{}
	for _, it := range items {
		if it.SolicitanteEmail == nil || *it.SolicitanteEmail == "" {
			continue
		}
		grupos[*it.SolicitanteEmail] = append(grupos[*it.SolicitanteEmail], it)
	}

	plantilla, err := s.plantillaRepo.ObtenerPorTipo(ctx, organizationID, "solicitud_vehiculo")
	asunto := asuntoPorDefectoSolicitud
	cuerpo := plantillaPorDefectoSolicitud
	if err == nil {
		asunto = plantilla.Asunto
		cuerpo = plantilla.CuerpoHTML
	}

	var todosLosIDs []int
	for email, filas := range grupos {
		nombreSolicitante := email
		if filas[0].SolicitanteNombre != nil && *filas[0].SolicitanteNombre != "" {
			nombreSolicitante = *filas[0].SolicitanteNombre
		}
		fechaServicio := filas[0].FechaProgramacion

		var htmlAprobadas, htmlRechazadas strings.Builder
		var aprobadasCount, rechazadasCount int
		for _, f := range filas {
			todosLosIDs = append(todosLosIDs, f.ID)
			if f.EstadoSolicitud == "aprobada" {
				aprobadasCount++
				htmlAprobadas.WriteString(fmt.Sprintf(
					`<div style="background:#F0FDF4;border-left:4px solid #16A34A;padding:12px 16px;margin:8px 0;border-radius:4px;">
						<p style="margin:0;font-weight:bold;color:#16A34A;">✓ Aprobada — %s</p>
						<p style="margin:4px 0 0;font-size:14px;">Vehículo: <strong>%s</strong> | Conductor: <strong>%s</strong></p>
						<p style="margin:4px 0 0;font-size:14px;">Horario: %s a %s | Destino: %s</p>
					</div>`,
					f.Actividad, valorOTexto(f.VehiculoPlaca, "por confirmar"), f.Conductor,
					f.HoraSalidaPunto, f.HoraFinalizacion, f.Destino,
				))
			} else if f.EstadoSolicitud == "rechazada" {
				rechazadasCount++
				motivo := "No especificado"
				if f.MotivoRechazo != nil && *f.MotivoRechazo != "" {
					motivo = *f.MotivoRechazo
				}
				htmlRechazadas.WriteString(fmt.Sprintf(
					`<div style="background:#FEF2F2;border-left:4px solid #DA151C;padding:12px 16px;margin:8px 0;border-radius:4px;">
						<p style="margin:0;font-weight:bold;color:#DA151C;">✗ No fue posible atender — %s</p>
						<p style="margin:4px 0 0;font-size:14px;">Motivo: %s</p>
					</div>`,
					f.Actividad, motivo,
				))
			}
		}

		bloqueAprobadas := ""
		if aprobadasCount > 0 {
			bloqueAprobadas = fmt.Sprintf(`<h4 style="color:#16A34A;margin-top:20px;">Solicitudes aprobadas (%d)</h4>%s`, aprobadasCount, htmlAprobadas.String())
		}
		bloqueRechazadas := ""
		if rechazadasCount > 0 {
			bloqueRechazadas = fmt.Sprintf(`<h4 style="color:#DA151C;margin-top:20px;">Solicitudes no atendidas (%d)</h4>%s`, rechazadasCount, htmlRechazadas.String())
		}

		asuntoFinal := reemplazarVariables(asunto, nombreSolicitante, fechaServicio, "", "")
		cuerpoTextoFinal := reemplazarVariables(cuerpo, nombreSolicitante, fechaServicio, bloqueAprobadas, bloqueRechazadas)
		// El Administrador edita texto plano — el wrapper HTML (header rojo
		// con esquinas redondeadas, etc.) se aplica siempre aquí antes de
		// enviar, para que nunca tenga que ver ni tocar HTML.
		cuerpoFinal := envolverEnPlantillaHTML(cuerpoTextoFinal)

		if err := s.notificador.EnviarCorreo(ctx, email, asuntoFinal, cuerpoFinal); err != nil {
			log.Printf("error al enviar correo de solicitud a %s: %v", email, err)
			continue
		}
	}

	if err := s.repo.MarcarNotificados(ctx, todosLosIDs); err != nil {
		log.Printf("error al marcar items como notificados: %v", err)
	}
}

func reemplazarVariables(texto, nombre, fecha, aprobadas, rechazadas string) string {
	texto = strings.ReplaceAll(texto, "{{nombre_solicitante}}", nombre)
	texto = strings.ReplaceAll(texto, "{{fecha_servicio}}", fecha)
	texto = strings.ReplaceAll(texto, "{{detalle_aprobadas}}", aprobadas)
	texto = strings.ReplaceAll(texto, "{{detalle_rechazadas}}", rechazadas)
	return texto
}

func valorOTexto(valor, textoSiVacio string) string {
	if valor == "" {
		return textoSiVacio
	}
	return valor
}

// ObtenerPlantillaSolicitud devuelve la plantilla configurada para
// notificar solicitudes de vehículo, o la plantilla por defecto si la
// organización aún no ha guardado una propia.
func (s *ProgramacionService) ObtenerPlantillaSolicitud(ctx context.Context, organizationID int) (asunto, cuerpo string) {
	plantilla, err := s.plantillaRepo.ObtenerPorTipo(ctx, organizationID, "solicitud_vehiculo")
	if err != nil {
		return asuntoPorDefectoSolicitud, plantillaPorDefectoSolicitud
	}
	return plantilla.Asunto, plantilla.CuerpoHTML
}

// GuardarPlantillaSolicitud permite a un Administrador editar la plantilla
// de notificación desde el Centro de Programación.
func (s *ProgramacionService) GuardarPlantillaSolicitud(ctx context.Context, organizationID int, asunto, cuerpo string) error {
	return s.plantillaRepo.GuardarOActualizar(ctx, organizationID, "solicitud_vehiculo", asunto, cuerpo)
}

func (s *ProgramacionService) Eliminar(ctx context.Context, organizationID, id int) error {
	return s.repo.Eliminar(ctx, organizationID, id)
}

// SolicitarVehiculo implementa el flujo del formulario de solicitud:
//  1. Busca si ya existe una programación para la fecha solicitada.
//  2. Si no existe, la crea vacía (sin items).
//  3. Agrega el nuevo item a esa programación con conductor/vehículo sin
//     asignar y programado=false — queda pendiente de que el director lo
//     complete y confirme desde "Editar Programación".
//
// Devuelve el ID de la programación (para poder redirigir/consultar) y el
// ID del item recién creado.
func (s *ProgramacionService) SolicitarVehiculo(ctx context.Context, organizationID int, creadoPor int, item *models.ProgramacionItem, fecha string) (programacionID int, itemID int, err error) {
	prog, err := s.repo.BuscarPorFecha(ctx, organizationID, fecha)
	if err != nil {
		if !errors.Is(err, apperrors.ErrNotFound) {
			return 0, 0, err
		}
		// No existe programación para esta fecha todavía — se crea vacía.
		nuevaProg := &models.Programacion{
			OrganizationID: organizationID,
			Fecha:          fecha,
			CreadoPor:      &creadoPor,
			Items:          []models.ProgramacionItem{},
		}
		programacionID, err = s.repo.Crear(ctx, nuevaProg)
		if err != nil {
			return 0, 0, err
		}
	} else {
		programacionID = prog.ID
	}

	itemID, err = s.repo.AgregarItem(ctx, programacionID, item)
	if err != nil {
		return 0, 0, err
	}
	return programacionID, itemID, nil
}

// MisSolicitudes lista las solicitudes hechas por un correo específico —
// alimenta la pantalla "Mis solicitudes" del rol Solicitante, con filtros
// de estado/mes y paginación para no cargar decenas de registros de golpe.
func (s *ProgramacionService) MisSolicitudes(ctx context.Context, organizationID int, email string, f repository.FiltrosMisSolicitudes) ([]models.ProgramacionItem, int, error) {
	return s.repo.ListarSolicitudesPorEmail(ctx, organizationID, email, f)
}
