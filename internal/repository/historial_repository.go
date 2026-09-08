package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/alcaldia/sigpa-backend/internal/models"
)

type HistorialRepository struct {
	db *sql.DB
}

func NewHistorialRepository(db *sql.DB) *HistorialRepository {
	return &HistorialRepository{db: db}
}

// Registrar inserta una entrada de auditoría. Se usa desde los services de
// cada módulo (vehículos, documentos, usuarios, etc.) tras cada mutación.
func (r *HistorialRepository) Registrar(ctx context.Context, h *models.HistorialCambio) error {
	query := `
		INSERT INTO historial_cambios (
			organization_id, entidad, entidad_id, usuario_id, accion,
			campo_modificado, valor_anterior, valor_nuevo, motivo
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := r.db.ExecContext(ctx, query,
		h.OrganizationID, h.Entidad, h.EntidadID, h.UsuarioID, h.Accion,
		h.CampoModificado, h.ValorAnterior, h.ValorNuevo, h.Motivo,
	)
	if err != nil {
		return fmt.Errorf("error al registrar historial: %w", err)
	}
	return nil
}

// ListarRecientePorOrganizacion retorna los últimos movimientos de CUALQUIER
// entidad de la organización (vehículos, documentos, usuarios, etc.), para
// el panel de "Actividad reciente" del dashboard. Cuando la entidad es un
// vehículo o un documento, se resuelve la placa del vehículo como referencia
// legible (ej: "Actualizó el vehículo ABC-123").
func (r *HistorialRepository) ListarRecientePorOrganizacion(ctx context.Context, organizationID, limite int) ([]models.HistorialCambio, error) {
	if limite <= 0 || limite > 100 {
		limite = 10
	}
	query := `
		SELECT
			h.id, h.organization_id, h.entidad, h.entidad_id, h.usuario_id, COALESCE(u.nombre, 'Sistema') AS usuario_nombre,
			h.accion, h.campo_modificado, h.valor_anterior, h.valor_nuevo, h.motivo, h.fecha,
			CASE
				WHEN h.entidad = 'vehiculo' THEN (SELECT placa FROM vehiculos WHERE id = h.entidad_id)
				WHEN h.entidad = 'documento' THEN (
					SELECT v2.placa FROM documentos d2
					INNER JOIN vehiculos v2 ON v2.id = d2.vehiculo_id
					WHERE d2.id = h.entidad_id
				)
				ELSE NULL
			END AS referencia
		FROM historial_cambios h
		LEFT JOIN usuarios u ON u.id = h.usuario_id
		WHERE h.organization_id = $1
		ORDER BY h.fecha DESC
		LIMIT $2
	`
	rows, err := r.db.QueryContext(ctx, query, organizationID, limite)
	if err != nil {
		return nil, fmt.Errorf("error al listar actividad reciente: %w", err)
	}
	defer rows.Close()

	var historial []models.HistorialCambio
	for rows.Next() {
		var h models.HistorialCambio
		if err := rows.Scan(
			&h.ID, &h.OrganizationID, &h.Entidad, &h.EntidadID, &h.UsuarioID, &h.UsuarioNombre,
			&h.Accion, &h.CampoModificado, &h.ValorAnterior, &h.ValorNuevo, &h.Motivo, &h.Fecha, &h.Referencia,
		); err != nil {
			return nil, fmt.Errorf("error al leer actividad reciente: %w", err)
		}
		historial = append(historial, h)
	}
	return historial, nil
}

// ListarPorEntidad devuelve el historial de una entidad específica (ej: vehículo id=10).
func (r *HistorialRepository) ListarPorEntidad(ctx context.Context, organizationID int, entidad string, entidadID int) ([]models.HistorialCambio, error) {
	query := `
		SELECT h.id, h.organization_id, h.entidad, h.entidad_id, h.usuario_id, COALESCE(u.nombre, 'Sistema') AS usuario_nombre,
			h.accion, h.campo_modificado, h.valor_anterior, h.valor_nuevo, h.motivo, h.fecha
		FROM historial_cambios h
		LEFT JOIN usuarios u ON u.id = h.usuario_id
		WHERE h.organization_id = $1 AND h.entidad = $2 AND h.entidad_id = $3
		ORDER BY h.fecha DESC
	`
	rows, err := r.db.QueryContext(ctx, query, organizationID, entidad, entidadID)
	if err != nil {
		return nil, fmt.Errorf("error al listar historial: %w", err)
	}
	defer rows.Close()

	var historial []models.HistorialCambio
	for rows.Next() {
		var h models.HistorialCambio
		if err := rows.Scan(
			&h.ID, &h.OrganizationID, &h.Entidad, &h.EntidadID, &h.UsuarioID, &h.UsuarioNombre,
			&h.Accion, &h.CampoModificado, &h.ValorAnterior, &h.ValorNuevo, &h.Motivo, &h.Fecha,
		); err != nil {
			return nil, fmt.Errorf("error al leer historial: %w", err)
		}
		historial = append(historial, h)
	}
	return historial, nil
}
