package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/alcaldia/sigpa-backend/internal/models"
	"github.com/alcaldia/sigpa-backend/pkg/apperrors"
)

type CatalogoRepository struct {
	db *sql.DB
}

func NewCatalogoRepository(db *sql.DB) *CatalogoRepository {
	return &CatalogoRepository{db: db}
}

func (r *CatalogoRepository) ListRoles(ctx context.Context) ([]models.Rol, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, nombre, descripcion FROM roles ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("error al listar roles: %w", err)
	}
	defer rows.Close()

	var roles []models.Rol
	for rows.Next() {
		var rl models.Rol
		if err := rows.Scan(&rl.ID, &rl.Nombre, &rl.Descripcion); err != nil {
			return nil, err
		}
		roles = append(roles, rl)
	}
	return roles, nil
}

func (r *CatalogoRepository) ListEstadosVehiculo(ctx context.Context) ([]models.EstadoVehiculo, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, nombre FROM estados_vehiculo ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("error al listar estados de vehículo: %w", err)
	}
	defer rows.Close()

	var estados []models.EstadoVehiculo
	for rows.Next() {
		var e models.EstadoVehiculo
		if err := rows.Scan(&e.ID, &e.Nombre); err != nil {
			return nil, err
		}
		estados = append(estados, e)
	}
	return estados, nil
}

// ListTiposVehiculo trae SOLO los tipos activos, para el catálogo que
// alimenta el formulario de creación de vehículo y los filtros. Los
// inactivos (desactivados desde Admin) no dejan de existir en la BD —
// siguen siendo referenciados por vehículos ya creados con ese tipo — pero
// no se ofrecen como opción para vehículos nuevos.
func (r *CatalogoRepository) ListTiposVehiculo(ctx context.Context) ([]models.TipoVehiculo, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, nombre, activo FROM tipos_vehiculo WHERE activo = TRUE ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("error al listar tipos de vehículo: %w", err)
	}
	defer rows.Close()

	var tipos []models.TipoVehiculo
	for rows.Next() {
		var t models.TipoVehiculo
		if err := rows.Scan(&t.ID, &t.Nombre, &t.Activo); err != nil {
			return nil, err
		}
		tipos = append(tipos, t)
	}
	return tipos, nil
}

// ListTodosTiposVehiculo trae TODOS los tipos (activos e inactivos), usado
// por el panel de Administración → Listas para poder activar/desactivar.
func (r *CatalogoRepository) ListTodosTiposVehiculo(ctx context.Context) ([]models.TipoVehiculo, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, nombre, activo FROM tipos_vehiculo ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("error al listar todos los tipos de vehículo: %w", err)
	}
	defer rows.Close()

	var tipos []models.TipoVehiculo
	for rows.Next() {
		var t models.TipoVehiculo
		if err := rows.Scan(&t.ID, &t.Nombre, &t.Activo); err != nil {
			return nil, err
		}
		tipos = append(tipos, t)
	}
	return tipos, nil
}

// CrearTipoVehiculo agrega un nuevo tipo al catálogo (ej. "Volqueta",
// "Retroexcavadora"), disponible desde Administración → Listas.
func (r *CatalogoRepository) CrearTipoVehiculo(ctx context.Context, nombre string) (int, error) {
	var id int
	err := r.db.QueryRowContext(ctx,
		"INSERT INTO tipos_vehiculo (nombre, activo) VALUES ($1, TRUE) RETURNING id",
		nombre,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("error al crear tipo de vehículo: %w", err)
	}
	return id, nil
}

// ActualizarTipoVehiculo permite renombrar o activar/desactivar un tipo.
func (r *CatalogoRepository) ActualizarTipoVehiculo(ctx context.Context, id int, nombre string, activo bool) error {
	res, err := r.db.ExecContext(ctx,
		"UPDATE tipos_vehiculo SET nombre = $1, activo = $2 WHERE id = $3",
		nombre, activo, id,
	)
	if err != nil {
		return fmt.Errorf("error al actualizar tipo de vehículo: %w", err)
	}
	filas, _ := res.RowsAffected()
	if filas == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

// ListTiposDocumento trae SOLO los tipos activos — filtra "Póliza de
// seguros" (desactivada por decisión del cliente) sin borrar documentos
// históricos ya registrados con ese tipo.
func (r *CatalogoRepository) ListTiposDocumento(ctx context.Context) ([]models.TipoDocumento, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, nombre, obligatorio, dias_alerta_default FROM tipos_documento WHERE activo = TRUE ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("error al listar tipos de documento: %w", err)
	}
	defer rows.Close()

	var tipos []models.TipoDocumento
	for rows.Next() {
		var t models.TipoDocumento
		if err := rows.Scan(&t.ID, &t.Nombre, &t.Obligatorio, &t.DiasAlertaDefault); err != nil {
			return nil, err
		}
		tipos = append(tipos, t)
	}
	return tipos, nil
}

// ObtenerNombreTipoDocumento devuelve el nombre de un tipo de documento por su ID.
// Se usa en el handler de documentos para construir la ruta de carpetas en Drive.
func (r *CatalogoRepository) ObtenerNombreTipoDocumento(ctx context.Context, id int) (string, error) {
	var nombre string
	err := r.db.QueryRowContext(ctx, "SELECT nombre FROM tipos_documento WHERE id = $1", id).Scan(&nombre)
	if err == sql.ErrNoRows {
		return "", apperrors.ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("error al obtener nombre de tipo de documento id=%d: %w", id, err)
	}
	return nombre, nil
}
