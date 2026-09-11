package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/alcaldia/sigpa-backend/internal/models"
	"github.com/alcaldia/sigpa-backend/pkg/apperrors"
)

type ProgramacionRepository struct {
	db *sql.DB
}

func NewProgramacionRepository(db *sql.DB) *ProgramacionRepository {
	return &ProgramacionRepository{db: db}
}

// Listar devuelve las cabeceras de programación de una organización (sin items),
// ordenadas por fecha descendente.
func (r *ProgramacionRepository) Listar(ctx context.Context, organizationID int) ([]models.Programacion, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT p.id, p.organization_id, TO_CHAR(p.fecha, 'YYYY-MM-DD') AS fecha,
		       p.observaciones, p.creado_por,
		       COALESCE(u.nombre, '') AS creado_por_nombre,
		       p.fecha_creacion, p.fecha_actualizacion,
		       COALESCE(pi.total_items, 0) AS total_items,
		       COALESCE(pi.total_programados, 0) AS total_programados
		FROM programaciones_vehiculos p
		LEFT JOIN usuarios u ON u.id = p.creado_por
		LEFT JOIN (
		  SELECT programacion_id,
		         COUNT(*) AS total_items,
		         COUNT(*) FILTER (WHERE programado = TRUE) AS total_programados
		  FROM programacion_items
		  GROUP BY programacion_id
		) pi ON pi.programacion_id = p.id
		WHERE p.organization_id = $1
		ORDER BY p.fecha DESC
	`, organizationID)
	if err != nil {
		return nil, fmt.Errorf("error al listar programaciones: %w", err)
	}
	defer rows.Close()

	var lista []models.Programacion
	for rows.Next() {
		var p models.Programacion
		if err := rows.Scan(&p.ID, &p.OrganizationID, &p.Fecha, &p.Observaciones,
			&p.CreadoPor, &p.CreadoPorNombre, &p.FechaCreacion, &p.FechaActualizacion,
			&p.TotalItems, &p.TotalProgramados); err != nil {
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

	var newID int
	err = r.db.QueryRowContext(ctx, `
		INSERT INTO programacion_items
		  (programacion_id, vehiculo_id, conductor, dependencia, destino,
		   hora_salida_punto, hora_finalizacion, actividad, es_vacaciones, programado, orden,
		   motivo, origen, solicitante_nombre, solicitante_email, hora_solicitada, punto_encuentro)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
		RETURNING id
	`, programacionID, item.VehiculoID, item.Conductor, item.Dependencia,
		item.Destino, item.HoraSalidaPunto, item.HoraFinalizacion, item.Actividad,
		item.EsVacaciones, item.Programado, siguienteOrden,
		item.Motivo, origen, item.SolicitanteNombre, item.SolicitanteEmail,
		item.HoraSolicitada, item.PuntoEncuentro).Scan(&newID)
	if err != nil {
		return 0, fmt.Errorf("error al agregar item a la programación: %w", err)
	}
	return newID, nil
}

// ListarSolicitudesPorEmail devuelve todas las filas (de cualquier
// programación) que fueron solicitadas por un correo específico —
// alimenta la pantalla "Mis solicitudes" del rol Solicitante.
func (r *ProgramacionRepository) ListarSolicitudesPorEmail(ctx context.Context, organizationID int, email string) ([]models.ProgramacionItem, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT pi.id, pi.programacion_id, pi.vehiculo_id,
		       COALESCE(v.placa, '') AS vehiculo_placa,
		       pi.conductor, pi.dependencia, pi.destino,
		       pi.hora_salida_punto, pi.hora_finalizacion, pi.actividad,
		       pi.es_vacaciones, pi.programado, pi.orden,
		       pi.motivo, pi.origen, pi.solicitante_nombre, pi.solicitante_email,
		       TO_CHAR(pi.hora_solicitada, 'HH24:MI') AS hora_solicitada, pi.punto_encuentro,
		       TO_CHAR(p.fecha, 'YYYY-MM-DD') AS fecha_programacion
		FROM programacion_items pi
		JOIN programaciones_vehiculos p ON p.id = pi.programacion_id
		LEFT JOIN vehiculos v ON v.id = pi.vehiculo_id
		WHERE p.organization_id = $1 AND pi.solicitante_email = $2
		ORDER BY p.fecha DESC, pi.id DESC
	`, organizationID, email)
	if err != nil {
		return nil, fmt.Errorf("error al listar solicitudes por email: %w", err)
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
			&horaSolicitada, &item.PuntoEncuentro, &fechaProgramacion,
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
		       TO_CHAR(pi.hora_solicitada, 'HH24:MI') AS hora_solicitada, pi.punto_encuentro
		FROM programacion_items pi
		LEFT JOIN vehiculos v ON v.id = pi.vehiculo_id
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
		_, err := tx.ExecContext(ctx, `
			INSERT INTO programacion_items
			  (programacion_id, vehiculo_id, conductor, dependencia, destino,
			   hora_salida_punto, hora_finalizacion, actividad, es_vacaciones, programado, orden,
			   motivo, origen, solicitante_nombre, solicitante_email, hora_solicitada, punto_encuentro)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
		`, programacionID, item.VehiculoID, item.Conductor, item.Dependencia,
			item.Destino, item.HoraSalidaPunto, item.HoraFinalizacion, item.Actividad,
			item.EsVacaciones, item.Programado, i,
			item.Motivo, origen, item.SolicitanteNombre, item.SolicitanteEmail,
			item.HoraSolicitada, item.PuntoEncuentro)
		if err != nil {
			return fmt.Errorf("error al insertar item %d: %w", i, err)
		}
	}
	return nil
}
