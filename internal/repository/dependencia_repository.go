package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/alcaldia/sigpa-backend/internal/models"
	"github.com/alcaldia/sigpa-backend/pkg/apperrors"
)

type DependenciaRepository struct {
	db *sql.DB
}

func NewDependenciaRepository(db *sql.DB) *DependenciaRepository {
	return &DependenciaRepository{db: db}
}

const dependenciaSelectBase = `
	SELECT d.id, d.organization_id, d.nombre, d.descripcion, d.responsable_id,
		COALESCE(u.nombre, '') AS responsable_nombre, d.activo
	FROM dependencias d
	LEFT JOIN usuarios u ON u.id = d.responsable_id
`

func scanDependencia(row interface {
	Scan(dest ...interface{}) error
}) (*models.Dependencia, error) {
	var d models.Dependencia
	err := row.Scan(&d.ID, &d.OrganizationID, &d.Nombre, &d.Descripcion, &d.ResponsableID, &d.ResponsableNombre, &d.Activo)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *DependenciaRepository) List(ctx context.Context, organizationID int) ([]models.Dependencia, error) {
	query := dependenciaSelectBase + " WHERE d.organization_id = $1 AND d.activo = TRUE ORDER BY d.nombre"
	rows, err := r.db.QueryContext(ctx, query, organizationID)
	if err != nil {
		return nil, fmt.Errorf("error al listar dependencias: %w", err)
	}
	defer rows.Close()

	var dependencias []models.Dependencia
	for rows.Next() {
		d, err := scanDependencia(rows)
		if err != nil {
			return nil, err
		}
		dependencias = append(dependencias, *d)
	}
	return dependencias, nil
}

func (r *DependenciaRepository) GetByID(ctx context.Context, organizationID, id int) (*models.Dependencia, error) {
	query := dependenciaSelectBase + " WHERE d.id = $1 AND d.organization_id = $2"
	row := r.db.QueryRowContext(ctx, query, id, organizationID)
	d, err := scanDependencia(row)
	if err == sql.ErrNoRows {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("error al consultar dependencia: %w", err)
	}
	return d, nil
}

func (r *DependenciaRepository) Create(ctx context.Context, d *models.Dependencia) (int, error) {
	query := `
		INSERT INTO dependencias (organization_id, nombre, descripcion, responsable_id, activo)
		VALUES ($1, $2, $3, $4, TRUE)
		RETURNING id
	`
	var newID int
	err := r.db.QueryRowContext(ctx, query, d.OrganizationID, d.Nombre, d.Descripcion, d.ResponsableID).Scan(&newID)
	if err != nil {
		return 0, fmt.Errorf("error al crear dependencia: %w", err)
	}
	return newID, nil
}

func (r *DependenciaRepository) Update(ctx context.Context, d *models.Dependencia) error {
	query := `
		UPDATE dependencias SET nombre = $1, descripcion = $2, responsable_id = $3
		WHERE id = $4 AND organization_id = $5
	`
	result, err := r.db.ExecContext(ctx, query, d.Nombre, d.Descripcion, d.ResponsableID, d.ID, d.OrganizationID)
	if err != nil {
		return fmt.Errorf("error al actualizar dependencia: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *DependenciaRepository) SoftDelete(ctx context.Context, organizationID, id int) error {
	query := "UPDATE dependencias SET activo = FALSE WHERE id = $1 AND organization_id = $2"
	result, err := r.db.ExecContext(ctx, query, id, organizationID)
	if err != nil {
		return fmt.Errorf("error al eliminar dependencia: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}
