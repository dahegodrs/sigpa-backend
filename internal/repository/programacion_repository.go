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
		       p.fecha_creacion, p.fecha_actualizacion
		FROM programaciones_vehiculos p
		LEFT JOIN usuarios u ON u.id = p.creado_por
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
			&p.CreadoPor, &p.CreadoPorNombre, &p.FechaCreacion, &p.FechaActualizacion); err != nil {
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
		       pi.es_vacaciones, pi.programado, pi.orden
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
		if err := rows.Scan(
			&item.ID, &item.ProgramacionID, &item.VehiculoID, &item.VehiculoPlaca,
			&item.Conductor, &item.Dependencia, &item.Destino,
			&item.HoraSalidaPunto, &item.HoraFinalizacion, &item.Actividad,
			&item.EsVacaciones, &item.Programado, &item.Orden,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func insertarItems(ctx context.Context, tx *sql.Tx, programacionID int, items []models.ProgramacionItem) error {
	for i, item := range items {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO programacion_items
			  (programacion_id, vehiculo_id, conductor, dependencia, destino,
			   hora_salida_punto, hora_finalizacion, actividad, es_vacaciones, programado, orden)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		`, programacionID, item.VehiculoID, item.Conductor, item.Dependencia,
			item.Destino, item.HoraSalidaPunto, item.HoraFinalizacion, item.Actividad,
			item.EsVacaciones, item.Programado, i)
		if err != nil {
			return fmt.Errorf("error al insertar item %d: %w", i, err)
		}
	}
	return nil
}
