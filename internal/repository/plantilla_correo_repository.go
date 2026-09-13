package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/alcaldia/sigpa-backend/internal/models"
	"github.com/alcaldia/sigpa-backend/pkg/apperrors"
)

// PlantillaCorreoRepository gestiona las plantillas de correo configurables
// por organización (ej. la notificación de aprobación/rechazo de
// solicitudes de vehículo). Se guardan en base de datos — en vez de
// localStorage — para que cualquier Administrador vea y edite la misma
// plantilla, sin depender de qué navegador la haya guardado.
type PlantillaCorreoRepository struct {
	db *sql.DB
}

func NewPlantillaCorreoRepository(db *sql.DB) *PlantillaCorreoRepository {
	return &PlantillaCorreoRepository{db: db}
}

// ObtenerPorTipo devuelve la plantilla de un tipo específico para la
// organización. Si no existe (nunca se ha guardado/migrado una para esa
// organización), devuelve apperrors.ErrNotFound — el llamador decide si
// usar un valor por defecto embebido en el código.
func (r *PlantillaCorreoRepository) ObtenerPorTipo(ctx context.Context, organizationID int, tipo string) (*models.PlantillaCorreo, error) {
	var p models.PlantillaCorreo
	err := r.db.QueryRowContext(ctx, `
		SELECT id, organization_id, tipo, asunto, cuerpo_html, fecha_actualizacion
		FROM plantillas_correo
		WHERE organization_id = $1 AND tipo = $2
	`, organizationID, tipo).Scan(
		&p.ID, &p.OrganizationID, &p.Tipo, &p.Asunto, &p.CuerpoHTML, &p.FechaActualizacion,
	)
	if err == sql.ErrNoRows {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("error al obtener plantilla de correo: %w", err)
	}
	return &p, nil
}

// GuardarOActualizar hace un upsert: crea la plantilla si no existe para
// esa organización/tipo, o actualiza el asunto/cuerpo si ya existía.
func (r *PlantillaCorreoRepository) GuardarOActualizar(ctx context.Context, organizationID int, tipo, asunto, cuerpoHTML string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO plantillas_correo (organization_id, tipo, asunto, cuerpo_html, fecha_actualizacion)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (organization_id, tipo)
		DO UPDATE SET asunto = EXCLUDED.asunto, cuerpo_html = EXCLUDED.cuerpo_html, fecha_actualizacion = NOW()
	`, organizationID, tipo, asunto, cuerpoHTML)
	if err != nil {
		return fmt.Errorf("error al guardar plantilla de correo: %w", err)
	}
	return nil
}
