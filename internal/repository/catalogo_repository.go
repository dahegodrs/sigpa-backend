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

func (r *CatalogoRepository) ListTiposVehiculo(ctx context.Context) ([]models.TipoVehiculo, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, nombre FROM tipos_vehiculo ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("error al listar tipos de vehículo: %w", err)
	}
	defer rows.Close()

	var tipos []models.TipoVehiculo
	for rows.Next() {
		var t models.TipoVehiculo
		if err := rows.Scan(&t.ID, &t.Nombre); err != nil {
			return nil, err
		}
		tipos = append(tipos, t)
	}
	return tipos, nil
}

func (r *CatalogoRepository) ListTiposDocumento(ctx context.Context) ([]models.TipoDocumento, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, nombre, obligatorio, dias_alerta_default FROM tipos_documento ORDER BY id")
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
