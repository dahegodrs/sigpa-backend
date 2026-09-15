package repository

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"

	"github.com/alcaldia/sigpa-backend/internal/models"
	"github.com/alcaldia/sigpa-backend/pkg/apperrors"
)

// horaRegex extrae la hora "HH:MM" al inicio de campos de texto libre como
// "08:00 - Parque Principal" (hora_salida_punto) o "10:00" / "DISPONIBLE
// PATIO" (hora_finalizacion). Se usa para poder comparar horarios aunque
// estos campos no sean columnas TIME puras en la base de datos.
var horaRegex = regexp.MustCompile(`^(\d{1,2}):(\d{2})`)

// extraerMinutos convierte "08:30 - algo" en minutos desde medianoche
// (510), para poder comparar rangos horarios numéricamente. Devuelve -1 si
// el texto no tiene un formato de hora reconocible (ej. "DISPONIBLE
// PATIO"), en cuyo caso el traslape nunca se considera aplicable para esa
// fila.
func extraerMinutos(texto string) int {
	m := horaRegex.FindStringSubmatch(texto)
	if m == nil {
		return -1
	}
	var horas, minutos int
	fmt.Sscanf(m[1], "%d", &horas)
	fmt.Sscanf(m[2], "%d", &minutos)
	return horas*60 + minutos
}

// rangosSeSolapan implementa la regla estándar de traslape de intervalos:
// dos rangos [inicioA, finA) y [inicioB, finB) se solapan si
// inicioA < finB Y inicioB < finA.
func rangosSeSolapan(inicioA, finA, inicioB, finB int) bool {
	if inicioA < 0 || finA < 0 || inicioB < 0 || finB < 0 {
		return false
	}
	return inicioA < finB && inicioB < finA
}

type ProgramacionRepository struct {
	db *sql.DB
}

func NewProgramacionRepository(db *sql.DB) *ProgramacionRepository {
	return &ProgramacionRepository{db: db}
}

// Listar devuelve las cabeceras de programación de una organización (sin
// items), ordenadas por fecha descendente. Si anio y mes son > 0, filtra
// solo las programaciones de ese mes — usado por la vista de calendario
// del frontend para no tener que traer todo el histórico de una vez.
// TienePendientes indica si el día tiene al menos una fila de solicitud
// sin resolver (origen='solicitud' y estado_solicitud='pendiente'), lo
// que el frontend usa para resaltar en rojo los días que requieren
// atención del Administrador.
func (r *ProgramacionRepository) Listar(ctx context.Context, organizationID, anio, mes int) ([]models.Programacion, error) {
	query := `
		SELECT p.id, p.organization_id, TO_CHAR(p.fecha, 'YYYY-MM-DD') AS fecha,
		       p.observaciones, p.creado_por,
		       COALESCE(u.nombre, '') AS creado_por_nombre,
		       p.fecha_creacion, p.fecha_actualizacion,
		       COALESCE(pi.total_items, 0) AS total_items,
		       COALESCE(pi.total_programados, 0) AS total_programados,
		       COALESCE(pi.tiene_pendientes, FALSE) AS tiene_pendientes
		FROM programaciones_vehiculos p
		LEFT JOIN usuarios u ON u.id = p.creado_por
		LEFT JOIN (
		  SELECT programacion_id,
		         COUNT(*) AS total_items,
		         COUNT(*) FILTER (WHERE programado = TRUE) AS total_programados,
		         BOOL_OR(origen = 'solicitud' AND estado_solicitud = 'pendiente') AS tiene_pendientes
		  FROM programacion_items
		  GROUP BY programacion_id
		) pi ON pi.programacion_id = p.id
		WHERE p.organization_id = $1
	`
	args := []interface{}{organizationID}
	if anio > 0 && mes > 0 {
		query += " AND EXTRACT(YEAR FROM p.fecha) = $2 AND EXTRACT(MONTH FROM p.fecha) = $3"
		args = append(args, anio, mes)
	}
	query += " ORDER BY p.fecha DESC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("error al listar programaciones: %w", err)
	}
	defer rows.Close()

	var lista []models.Programacion
	for rows.Next() {
		var p models.Programacion
		if err := rows.Scan(&p.ID, &p.OrganizationID, &p.Fecha, &p.Observaciones,
			&p.CreadoPor, &p.CreadoPorNombre, &p.FechaCreacion, &p.FechaActualizacion,
			&p.TotalItems, &p.TotalProgramados, &p.TienePendientes); err != nil {
			return nil, err
		}
		lista = append(lista, p)
	}
	return lista, nil
}

// ObtenerUltima devuelve la programación más reciente (por fecha) de la
// organización, incluyendo sus items — usada para "Copiar programación
// anterior" en el frontend, evitando que el usuario deba re-crear cada fila
// manualmente cuando la mayoría de vehículos repiten conductor/actividad.
func (r *ProgramacionRepository) ObtenerUltima(ctx context.Context, organizationID int) (*models.Programacion, error) {
	var id int
	err := r.db.QueryRowContext(ctx, `
		SELECT id FROM programaciones_vehiculos
		WHERE organization_id = $1
		ORDER BY fecha DESC, id DESC
		LIMIT 1
	`, organizationID).Scan(&id)
	if err == sql.ErrNoRows {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("error al buscar la última programación: %w", err)
	}
	return r.Obtener(ctx, organizationID, id)
}

// Obtener devuelve la cabecera + items de una programación.
func (r *ProgramacionRepository) Obtener(ctx context.Context, organizationID, id int) (*models.Programacion, error) {
	var p models.Programacion
	err := r.db.QueryRowContext(ctx, `
		SELECT p.id, p.organization_id, TO_CHAR(p.fecha, 'YYYY-MM-DD'),
		       p.observaciones, p.creado_por,
		       COALESCE(u.nombre, '') AS creado_por_nombre,
		       p.fecha_creacion, p.fecha_actualizacion
		FROM programaciones_vehiculos p
		LEFT JOIN usuarios u ON u.id = p.creado_por
		WHERE p.id = $1 AND p.organization_id = $2
	`, id, organizationID).Scan(
		&p.ID, &p.OrganizationID, &p.Fecha, &p.Observaciones,
		&p.CreadoPor, &p.CreadoPorNombre, &p.FechaCreacion, &p.FechaActualizacion,
	)
	if err == sql.ErrNoRows {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("error al obtener programación: %w", err)
	}

	items, err := r.listarItems(ctx, id)
	if err != nil {
		return nil, err
	}
	p.Items = items
	return &p, nil
}

// Crear inserta la cabecera y los items en una transacción.
func (r *ProgramacionRepository) Crear(ctx context.Context, p *models.Programacion) (int, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var newID int
	err = tx.QueryRowContext(ctx, `
		INSERT INTO programaciones_vehiculos (organization_id, fecha, observaciones, creado_por)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`, p.OrganizationID, p.Fecha, p.Observaciones, p.CreadoPor).Scan(&newID)
	if err != nil {
		return 0, fmt.Errorf("error al crear programación: %w", err)
	}

	if err := insertarItems(ctx, tx, newID, p.Items); err != nil {
		return 0, err
	}

	return newID, tx.Commit()
}

// Actualizar reemplaza la cabecera y re-inserta todos los items.
func (r *ProgramacionRepository) Actualizar(ctx context.Context, organizationID, id int, p *models.Programacion) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		UPDATE programaciones_vehiculos
		SET fecha = $1, observaciones = $2, fecha_actualizacion = NOW()
		WHERE id = $3 AND organization_id = $4
	`, p.Fecha, p.Observaciones, id, organizationID)
	if err != nil {
		return fmt.Errorf("error al actualizar programación: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return apperrors.ErrNotFound
	}

	// Borrar items anteriores y re-insertar
	if _, err := tx.ExecContext(ctx,
		"DELETE FROM programacion_items WHERE programacion_id = $1", id); err != nil {
		return fmt.Errorf("error al borrar items anteriores: %w", err)
	}
	if err := insertarItems(ctx, tx, id, p.Items); err != nil {
		return err
	}

	return tx.Commit()
}

// BuscarPorFecha devuelve la cabecera de la programación (sin items) que
// existe para una fecha específica dentro de la organización, o
// apperrors.ErrNotFound si todavía no hay ninguna. Se usa en el flujo de
// solicitud de vehículo: antes de crear una nueva programación para el
// día, se verifica si ya existe una para no duplicarla.
func (r *ProgramacionRepository) BuscarPorFecha(ctx context.Context, organizationID int, fecha string) (*models.Programacion, error) {
	var p models.Programacion
	err := r.db.QueryRowContext(ctx, `
		SELECT id, organization_id, TO_CHAR(fecha, 'YYYY-MM-DD'), observaciones,
		       creado_por, fecha_creacion, fecha_actualizacion
		FROM programaciones_vehiculos
		WHERE organization_id = $1 AND fecha = $2
	`, organizationID, fecha).Scan(
		&p.ID, &p.OrganizationID, &p.Fecha, &p.Observaciones,
		&p.CreadoPor, &p.FechaCreacion, &p.FechaActualizacion,
	)
	if err == sql.ErrNoRows {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("error al buscar programación por fecha: %w", err)
	}
	return &p, nil
}

// AgregarItem inserta UNA fila nueva a una programación ya existente,
// calculando automáticamente el siguiente "orden" disponible — usado por
// el flujo de solicitud de vehículo, que agrega una fila a la vez sin
// reemplazar las que ya existen (a diferencia de Actualizar, que reescribe
// todos los items de una).
func (r *ProgramacionRepository) AgregarItem(ctx context.Context, programacionID int, item *models.ProgramacionItem) (int, error) {
	var siguienteOrden int
	err := r.db.QueryRowContext(ctx,
		"SELECT COALESCE(MAX(orden), -1) + 1 FROM programacion_items WHERE programacion_id = $1",
		programacionID,
	).Scan(&siguienteOrden)
	if err != nil {
		return 0, fmt.Errorf("error al calcular el orden del nuevo item: %w", err)
	}

	origen := item.Origen
	if origen == "" {
		origen = "manual"
	}
	estadoSolicitud := item.EstadoSolicitud
	if estadoSolicitud == "" {
		estadoSolicitud = "pendiente"
	}

	var newID int
	err = r.db.QueryRowContext(ctx, `
		INSERT INTO programacion_items
		  (programacion_id, vehiculo_id, conductor, dependencia, destino,
		   hora_salida_punto, hora_finalizacion, actividad, es_vacaciones, programado, orden,
		   motivo, origen, solicitante_nombre, solicitante_email, hora_solicitada, punto_encuentro,
		   estado_solicitud, tipo_vehiculo_solicitado_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
		RETURNING id
	`, programacionID, item.VehiculoID, item.Conductor, item.Dependencia,
		item.Destino, item.HoraSalidaPunto, item.HoraFinalizacion, item.Actividad,
		item.EsVacaciones, item.Programado, siguienteOrden,
		item.Motivo, origen, item.SolicitanteNombre, item.SolicitanteEmail,
		item.HoraSolicitada, item.PuntoEncuentro, estadoSolicitud, item.TipoVehiculoSolicitadoID).Scan(&newID)
	if err != nil {
		return 0, fmt.Errorf("error al agregar item a la programación: %w", err)
	}
	return newID, nil
}

// ConflictoHorario describe un choque de horario detectado al intentar
// aprobar una fila: qué recurso chocó (vehículo o conductor) y con qué
// otra fila de la misma programación.
type ConflictoHorario struct {
	Recurso            string // "vehiculo" o "conductor"
	ValorRecurso       string // placa o nombre del conductor
	ActividadChocante  string
	HoraInicioChocante string
	HoraFinChocante    string
}

// BuscarConflictoHorario revisa las demás filas de la misma programación
// (mismo día) que ya estén marcadas como programado=TRUE, y detecta si el
// vehículo o el conductor de la fila que se va a aprobar ya están
// ocupados en un horario que se solapa. excluirItemID se usa para no
// comparar la fila contra sí misma. Devuelve nil si no hay conflicto.
func (r *ProgramacionRepository) BuscarConflictoHorario(ctx context.Context, programacionID int, vehiculoID *int, conductor string, horaSalidaPunto, horaFinalizacion string, excluirItemID int) (*ConflictoHorario, error) {
	inicioNueva := extraerMinutos(horaSalidaPunto)
	finNueva := extraerMinutos(horaFinalizacion)
	// Si la fila que se aprueba no tiene un horario reconocible, no hay
	// forma de detectar traslape — se deja pasar (comportamiento anterior).
	if inicioNueva < 0 || finNueva < 0 {
		return nil, nil
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT pi.vehiculo_id, COALESCE(v.placa, ''), pi.conductor, pi.hora_salida_punto, pi.hora_finalizacion, pi.actividad
		FROM programacion_items pi
		LEFT JOIN vehiculos v ON v.id = pi.vehiculo_id
		WHERE pi.programacion_id = $1 AND pi.id != $2 AND pi.programado = TRUE
	`, programacionID, excluirItemID)
	if err != nil {
		return nil, fmt.Errorf("error al buscar conflictos de horario: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var otroVehiculoID sql.NullInt64
		var otraPlaca, otroConductor, otraHoraInicio, otraHoraFin, otraActividad string
		if err := rows.Scan(&otroVehiculoID, &otraPlaca, &otroConductor, &otraHoraInicio, &otraHoraFin, &otraActividad); err != nil {
			return nil, err
		}

		inicioOtra := extraerMinutos(otraHoraInicio)
		finOtra := extraerMinutos(otraHoraFin)
		if !rangosSeSolapan(inicioNueva, finNueva, inicioOtra, finOtra) {
			continue
		}

		// Mismo vehículo en horario solapado.
		if vehiculoID != nil && otroVehiculoID.Valid && int(otroVehiculoID.Int64) == *vehiculoID {
			return &ConflictoHorario{
				Recurso: "vehiculo", ValorRecurso: otraPlaca,
				ActividadChocante: otraActividad, HoraInicioChocante: otraHoraInicio, HoraFinChocante: otraHoraFin,
			}, nil
		}
		// Mismo conductor en horario solapado (comparación case-insensitive
		// simple, ya que el conductor viene de una lista configurable con
		// nombres consistentes).
		if conductor != "" && conductor != "DISPONIBLE PATIO" && otroConductor == conductor {
			return &ConflictoHorario{
				Recurso: "conductor", ValorRecurso: otroConductor,
				ActividadChocante: otraActividad, HoraInicioChocante: otraHoraInicio, HoraFinChocante: otraHoraFin,
			}, nil
		}
	}
	return nil, nil
}

// ObtenerItem devuelve una sola fila por su ID — usado para leer los
// datos actuales (vehículo, conductor, horario) antes de validar
// conflictos al aprobar.
func (r *ProgramacionRepository) ObtenerItem(ctx context.Context, itemID int) (*models.ProgramacionItem, error) {
	var item models.ProgramacionItem
	err := r.db.QueryRowContext(ctx, `
		SELECT id, programacion_id, vehiculo_id, conductor, hora_salida_punto, hora_finalizacion
		FROM programacion_items WHERE id = $1
	`, itemID).Scan(&item.ID, &item.ProgramacionID, &item.VehiculoID, &item.Conductor, &item.HoraSalidaPunto, &item.HoraFinalizacion)
	if err == sql.ErrNoRows {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("error al obtener item: %w", err)
	}
	return &item, nil
}

// ActualizarEstadoItem aprueba o rechaza UNA fila de solicitud puntual sin
// tener que reescribir toda la programación — usado por los botones
// rápidos ✅/❌ en "Editar Programación". No marca notificado_en: eso lo
// hace el flujo de envío de correo al guardar, una vez el mensaje sale.
func (r *ProgramacionRepository) ActualizarEstadoItem(ctx context.Context, itemID int, estado string, motivoRechazo *string) error {
	result, err := r.db.ExecContext(ctx,
		"UPDATE programacion_items SET estado_solicitud = $1, motivo_rechazo = $2 WHERE id = $3",
		estado, motivoRechazo, itemID,
	)
	if err != nil {
		return fmt.Errorf("error al actualizar estado de la solicitud: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

// ListarDecisionesSinNotificar devuelve las filas de una programación que
// ya tienen una decisión tomada (aprobada/rechazada) pero todavía no se le
// ha avisado al solicitante — es la base para armar el correo consolidado.
// Incluye la fecha de la programación (join con la cabecera) porque el
// correo de notificación la necesita para el mensaje "resultado de tu
// solicitud para el día X" — antes faltaba este dato.
func (r *ProgramacionRepository) ListarDecisionesSinNotificar(ctx context.Context, programacionID int) ([]models.ProgramacionItem, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT pi.id, pi.programacion_id, pi.vehiculo_id,
		       COALESCE(v.placa, '') AS vehiculo_placa,
		       pi.conductor, pi.dependencia, pi.destino,
		       pi.hora_salida_punto, pi.hora_finalizacion, pi.actividad,
		       pi.es_vacaciones, pi.programado, pi.orden,
		       pi.motivo, pi.origen, pi.solicitante_nombre, pi.solicitante_email,
		       TO_CHAR(pi.hora_solicitada, 'HH24:MI') AS hora_solicitada, pi.punto_encuentro,
		       pi.estado_solicitud, pi.motivo_rechazo,
		       TO_CHAR(p.fecha, 'YYYY-MM-DD') AS fecha_programacion
		FROM programacion_items pi
		JOIN programaciones_vehiculos p ON p.id = pi.programacion_id
		LEFT JOIN vehiculos v ON v.id = pi.vehiculo_id
		WHERE pi.programacion_id = $1
		  AND pi.origen = 'solicitud'
		  AND pi.estado_solicitud != 'pendiente'
		  AND pi.notificado_en IS NULL
	`, programacionID)
	if err != nil {
		return nil, fmt.Errorf("error al listar decisiones sin notificar: %w", err)
	}
	defer rows.Close()

	var items []models.ProgramacionItem
	for rows.Next() {
		var item models.ProgramacionItem
		var horaSolicitada sql.NullString
		var fechaProgramacion string
		if err := rows.Scan(
			&item.ID, &item.ProgramacionID, &item.VehiculoID, &item.VehiculoPlaca,
			&item.Conductor, &item.Dependencia, &item.Destino,
			&item.HoraSalidaPunto, &item.HoraFinalizacion, &item.Actividad,
			&item.EsVacaciones, &item.Programado, &item.Orden,
			&item.Motivo, &item.Origen, &item.SolicitanteNombre, &item.SolicitanteEmail,
			&horaSolicitada, &item.PuntoEncuentro,
			&item.EstadoSolicitud, &item.MotivoRechazo, &fechaProgramacion,
		); err != nil {
			return nil, err
		}
		if horaSolicitada.Valid {
			item.HoraSolicitada = &horaSolicitada.String
		}
		item.FechaProgramacion = fechaProgramacion
		items = append(items, item)
	}
	return items, nil
}

// MarcarNotificados marca un conjunto de items como ya notificados, para
// que no se vuelvan a incluir en un correo futuro.
func (r *ProgramacionRepository) MarcarNotificados(ctx context.Context, itemIDs []int) error {
	if len(itemIDs) == 0 {
		return nil
	}
	placeholders := ""
	args := []interface{}{}
	for i, id := range itemIDs {
		if i > 0 {
			placeholders += ","
		}
		placeholders += fmt.Sprintf("$%d", i+1)
		args = append(args, id)
	}
	query := fmt.Sprintf("UPDATE programacion_items SET notificado_en = NOW() WHERE id IN (%s)", placeholders)
	_, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("error al marcar items como notificados: %w", err)
	}
	return nil
}

// Desbloquear limpia notificado_en de un item específico, permitiendo que
// el director vuelva a editarlo/reaprobar/rechazarlo. Se usa cuando algo
// cambió después de haber notificado al solicitante (ej. el vehículo se
// dañó y hay que reasignar otro) — el director debe usar esto de forma
// explícita, así queda claro que fue una corrección intencional.
func (r *ProgramacionRepository) Desbloquear(ctx context.Context, itemID int) error {
	result, err := r.db.ExecContext(ctx,
		"UPDATE programacion_items SET notificado_en = NULL WHERE id = $1", itemID,
	)
	if err != nil {
		return fmt.Errorf("error al desbloquear item: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

// FiltrosMisSolicitudes agrupa los filtros opcionales de la pantalla "Mis
// solicitudes": por estado (pendiente/aprobada/rechazada), por mes/año, y
// paginación — necesarios porque un solicitante frecuente puede acumular
// decenas de registros y listarlos todos de una vez volvería la pantalla
// lenta e ilegible.
type FiltrosMisSolicitudes struct {
	Estado   string // "" = todas
	Anio     int    // 0 = sin filtro
	Mes      int    // 0 = sin filtro
	Page     int    // 1-indexed
	PageSize int
}

// ListarSolicitudesPorEmail devuelve las filas (de cualquier programación)
// que fueron solicitadas por un correo específico — alimenta la pantalla
// "Mis solicitudes" del rol Solicitante. Soporta filtro por estado, por
// mes/año y paginación; devuelve también el total de registros que
// cumplen el filtro (sin paginar) para que el frontend pueda calcular el
// número de páginas.
func (r *ProgramacionRepository) ListarSolicitudesPorEmail(ctx context.Context, organizationID int, email string, f FiltrosMisSolicitudes) ([]models.ProgramacionItem, int, error) {
	conditions := []string{"p.organization_id = $1", "pi.solicitante_email = $2"}
	args := []interface{}{organizationID, email}
	argPos := 3

	if f.Estado != "" {
		conditions = append(conditions, fmt.Sprintf("pi.estado_solicitud = $%d", argPos))
		args = append(args, f.Estado)
		argPos++
	}
	if f.Anio > 0 && f.Mes > 0 {
		conditions = append(conditions, fmt.Sprintf("EXTRACT(YEAR FROM p.fecha) = $%d AND EXTRACT(MONTH FROM p.fecha) = $%d", argPos, argPos+1))
		args = append(args, f.Anio, f.Mes)
		argPos += 2
	}
	whereClause := "WHERE " + fmt.Sprintf("%s", conditions[0])
	for _, c := range conditions[1:] {
		whereClause += " AND " + c
	}

	var total int
	countQuery := "SELECT COUNT(*) FROM programacion_items pi JOIN programaciones_vehiculos p ON p.id = pi.programacion_id " + whereClause
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("error al contar solicitudes: %w", err)
	}

	page := f.Page
	if page < 1 {
		page = 1
	}
	pageSize := f.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	query := fmt.Sprintf(`
		SELECT pi.id, pi.programacion_id, pi.vehiculo_id,
		       COALESCE(v.placa, '') AS vehiculo_placa,
		       pi.conductor, pi.dependencia, pi.destino,
		       pi.hora_salida_punto, pi.hora_finalizacion, pi.actividad,
		       pi.es_vacaciones, pi.programado, pi.orden,
		       pi.motivo, pi.origen, pi.solicitante_nombre, pi.solicitante_email,
		       TO_CHAR(pi.hora_solicitada, 'HH24:MI') AS hora_solicitada, pi.punto_encuentro,
		       pi.estado_solicitud, pi.motivo_rechazo,
		       TO_CHAR(p.fecha, 'YYYY-MM-DD') AS fecha_programacion
		FROM programacion_items pi
		JOIN programaciones_vehiculos p ON p.id = pi.programacion_id
		LEFT JOIN vehiculos v ON v.id = pi.vehiculo_id
		%s
		ORDER BY p.fecha DESC, pi.id DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argPos, argPos+1)
	args = append(args, pageSize, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("error al listar solicitudes por email: %w", err)
	}
	defer rows.Close()

	var items []models.ProgramacionItem
	for rows.Next() {
		var item models.ProgramacionItem
		var horaSolicitada sql.NullString
		var fechaProgramacion string
		if err := rows.Scan(
			&item.ID, &item.ProgramacionID, &item.VehiculoID, &item.VehiculoPlaca,
			&item.Conductor, &item.Dependencia, &item.Destino,
			&item.HoraSalidaPunto, &item.HoraFinalizacion, &item.Actividad,
			&item.EsVacaciones, &item.Programado, &item.Orden,
			&item.Motivo, &item.Origen, &item.SolicitanteNombre, &item.SolicitanteEmail,
			&horaSolicitada, &item.PuntoEncuentro,
			&item.EstadoSolicitud, &item.MotivoRechazo, &fechaProgramacion,
		); err != nil {
			return nil, 0, err
		}
		if horaSolicitada.Valid {
			item.HoraSolicitada = &horaSolicitada.String
		}
		item.FechaProgramacion = fechaProgramacion
		items = append(items, item)
	}
	return items, total, nil
}

// Eliminar borra la cabecera (los items se borran por CASCADE).
func (r *ProgramacionRepository) Eliminar(ctx context.Context, organizationID, id int) error {
	result, err := r.db.ExecContext(ctx,
		"DELETE FROM programaciones_vehiculos WHERE id = $1 AND organization_id = $2",
		id, organizationID)
	if err != nil {
		return fmt.Errorf("error al eliminar programación: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

// ---- helpers ----------------------------------------------------------------

func (r *ProgramacionRepository) listarItems(ctx context.Context, programacionID int) ([]models.ProgramacionItem, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT pi.id, pi.programacion_id, pi.vehiculo_id,
		       COALESCE(v.placa, '') AS vehiculo_placa,
		       pi.conductor, pi.dependencia, pi.destino,
		       pi.hora_salida_punto, pi.hora_finalizacion, pi.actividad,
		       pi.es_vacaciones, pi.programado, pi.orden,
		       pi.motivo, pi.origen, pi.solicitante_nombre, pi.solicitante_email,
		       TO_CHAR(pi.hora_solicitada, 'HH24:MI') AS hora_solicitada, pi.punto_encuentro,
		       pi.estado_solicitud, pi.motivo_rechazo,
		       pi.tipo_vehiculo_solicitado_id, COALESCE(tv.nombre, '') AS tipo_vehiculo_solicitado_nombre
		FROM programacion_items pi
		LEFT JOIN vehiculos v ON v.id = pi.vehiculo_id
		LEFT JOIN tipos_vehiculo tv ON tv.id = pi.tipo_vehiculo_solicitado_id
		WHERE pi.programacion_id = $1
		ORDER BY pi.orden ASC, pi.id ASC
	`, programacionID)
	if err != nil {
		return nil, fmt.Errorf("error al listar items: %w", err)
	}
	defer rows.Close()

	var items []models.ProgramacionItem
	for rows.Next() {
		var item models.ProgramacionItem
		var horaSolicitada sql.NullString
		if err := rows.Scan(
			&item.ID, &item.ProgramacionID, &item.VehiculoID, &item.VehiculoPlaca,
			&item.Conductor, &item.Dependencia, &item.Destino,
			&item.HoraSalidaPunto, &item.HoraFinalizacion, &item.Actividad,
			&item.EsVacaciones, &item.Programado, &item.Orden,
			&item.Motivo, &item.Origen, &item.SolicitanteNombre, &item.SolicitanteEmail,
			&horaSolicitada, &item.PuntoEncuentro,
			&item.EstadoSolicitud, &item.MotivoRechazo,
			&item.TipoVehiculoSolicitadoID, &item.TipoVehiculoSolicitadoNombre,
		); err != nil {
			return nil, err
		}
		if horaSolicitada.Valid {
			item.HoraSolicitada = &horaSolicitada.String
		}
		items = append(items, item)
	}
	return items, nil
}

func insertarItems(ctx context.Context, tx *sql.Tx, programacionID int, items []models.ProgramacionItem) error {
	for i, item := range items {
		origen := item.Origen
		if origen == "" {
			origen = "manual"
		}
		estadoSolicitud := item.EstadoSolicitud
		if estadoSolicitud == "" {
			estadoSolicitud = "pendiente"
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO programacion_items
			  (programacion_id, vehiculo_id, conductor, dependencia, destino,
			   hora_salida_punto, hora_finalizacion, actividad, es_vacaciones, programado, orden,
			   motivo, origen, solicitante_nombre, solicitante_email, hora_solicitada, punto_encuentro,
			   estado_solicitud, motivo_rechazo, tipo_vehiculo_solicitado_id)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)
		`, programacionID, item.VehiculoID, item.Conductor, item.Dependencia,
			item.Destino, item.HoraSalidaPunto, item.HoraFinalizacion, item.Actividad,
			item.EsVacaciones, item.Programado, i,
			item.Motivo, origen, item.SolicitanteNombre, item.SolicitanteEmail,
			item.HoraSolicitada, item.PuntoEncuentro, estadoSolicitud, item.MotivoRechazo,
			item.TipoVehiculoSolicitadoID)
		if err != nil {
			return fmt.Errorf("error al insertar item %d: %w", i, err)
		}
	}
	return nil
}
