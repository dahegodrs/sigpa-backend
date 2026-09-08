package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/alcaldia/sigpa-backend/internal/models"
	"github.com/alcaldia/sigpa-backend/pkg/apperrors"
)

type ListaRepository struct {
	db *sql.DB
}

func NewListaRepository(db *sql.DB) *ListaRepository {
	return &ListaRepository{db: db}
}

// ListarPorTipo devuelve todos los items activos de un tipo para una organización,
// ordenados por `orden` ASC, luego por `nombre` ASC.
func (r *ListaRepository) ListarPorTipo(ctx context.Context, organizationID int, tipo string) ([]models.ListaConfiguracion, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, organization_id, tipo, nombre, activo, orden, fecha_creacion
		FROM listas_configuracion
		WHERE organization_id = $1 AND tipo = $2 AND activo = TRUE
		ORDER BY orden ASC, nombre ASC
	`, organizationID, tipo)
	if err != nil {
		return nil, fmt.Errorf("error al listar items de tipo '%s': %w", tipo, err)
	}
	defer rows.Close()
	return scanListas(rows)
}

// ListarTodos devuelve todos los items (activos e inactivos) de todos los tipos.
func (r *ListaRepository) ListarTodos(ctx context.Context, organizationID int) ([]models.ListaConfiguracion, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, organization_id, tipo, nombre, activo, orden, fecha_creacion
		FROM listas_configuracion
		WHERE organization_id = $1
		ORDER BY tipo ASC, orden ASC, nombre ASC
	`, organizationID)
	if err != nil {
		return nil, fmt.Errorf("error al listar todas las listas: %w", err)
	}
	defer rows.Close()
	return scanListas(rows)
}

// Crear inserta un nuevo item en la lista.
func (r *ListaRepository) Crear(ctx context.Context, item *models.ListaConfiguracion) (int, error) {
	var newID int
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO listas_configuracion (organization_id, tipo, nombre, activo, orden)
		VALUES ($1, $2, $3, TRUE, $4)
		RETURNING id
	`, item.OrganizationID, item.Tipo, item.Nombre, item.Orden).Scan(&newID)
	if err != nil {
		return 0, fmt.Errorf("error al crear item de lista: %w", err)
	}
	return newID, nil
}

// Actualizar modifica nombre, orden y estado activo de un item.
func (r *ListaRepository) Actualizar(ctx context.Context, organizationID, id int, nombre string, orden int, activo bool) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE listas_configuracion SET nombre = $1, orden = $2, activo = $3
		WHERE id = $4 AND organization_id = $5
	`, nombre, orden, activo, id, organizationID)
	if err != nil {
		return fmt.Errorf("error al actualizar item de lista: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

// Eliminar borra permanentemente un item (solo admin).
func (r *ListaRepository) Eliminar(ctx context.Context, organizationID, id int) error {
	result, err := r.db.ExecContext(ctx,
		"DELETE FROM listas_configuracion WHERE id = $1 AND organization_id = $2",
		id, organizationID)
	if err != nil {
		return fmt.Errorf("error al eliminar item de lista: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func scanListas(rows *sql.Rows) ([]models.ListaConfiguracion, error) {
	var lista []models.ListaConfiguracion
	for rows.Next() {
		var item models.ListaConfiguracion
		if err := rows.Scan(&item.ID, &item.OrganizationID, &item.Tipo, &item.Nombre,
			&item.Activo, &item.Orden, &item.FechaCreacion); err != nil {
			return nil, err
		}
		lista = append(lista, item)
	}
	return lista, nil
}
