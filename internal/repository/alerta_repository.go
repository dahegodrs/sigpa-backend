package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/alcaldia/sigpa-backend/internal/models"
	"github.com/alcaldia/sigpa-backend/pkg/apperrors"
)

type AlertaRepository struct {
	db *sql.DB
}

func NewAlertaRepository(db *sql.DB) *AlertaRepository {
	return &AlertaRepository{db: db}
}

// --- Configuración de reglas de alerta (config_alertas) ---

// ObtenerConfigActiva retorna solo los umbrales activos (usado por el motor de revisión diaria).
func (r *AlertaRepository) ObtenerConfigActiva(ctx context.Context, organizationID int) ([]models.ConfigAlerta, error) {
	return r.listarConfig(ctx, organizationID, true)
}

// ListarConfig retorna todas las reglas (activas e inactivas), para el panel de administración.
func (r *AlertaRepository) ListarConfig(ctx context.Context, organizationID int) ([]models.ConfigAlerta, error) {
	return r.listarConfig(ctx, organizationID, false)
}

func (r *AlertaRepository) listarConfig(ctx context.Context, organizationID int, soloActivas bool) ([]models.ConfigAlerta, error) {
	query := `
		SELECT id, organization_id, dias_antes, nivel, activo
		FROM config_alertas
		WHERE organization_id = $1
	`
	if soloActivas {
		query += " AND activo = TRUE"
	}
	query += " ORDER BY dias_antes DESC"

	rows, err := r.db.QueryContext(ctx, query, organizationID)
	if err != nil {
		return nil, fmt.Errorf("error al obtener configuración de alertas: %w", err)
	}
	defer rows.Close()

	var configs []models.ConfigAlerta
	for rows.Next() {
		var c models.ConfigAlerta
		if err := rows.Scan(&c.ID, &c.OrganizationID, &c.DiasAntes, &c.Nivel, &c.Activo); err != nil {
			return nil, err
		}
		configs = append(configs, c)
	}
	return configs, nil
}

func (r *AlertaRepository) CrearConfig(ctx context.Context, c *models.ConfigAlerta) (int, error) {
	query := `
		INSERT INTO config_alertas (organization_id, dias_antes, nivel, activo)
		VALUES ($1, $2, $3, TRUE)
		RETURNING id
	`
	var newID int
	err := r.db.QueryRowContext(ctx, query, c.OrganizationID, c.DiasAntes, c.Nivel).Scan(&newID)
	if err != nil {
		return 0, fmt.Errorf("error al crear regla de alerta: %w", err)
	}
	return newID, nil
}

func (r *AlertaRepository) ActualizarConfig(ctx context.Context, organizationID, id, diasAntes int, nivel string, activo bool) error {
	query := `
		UPDATE config_alertas SET dias_antes = $1, nivel = $2, activo = $3
		WHERE id = $4 AND organization_id = $5
	`
	result, err := r.db.ExecContext(ctx, query, diasAntes, nivel, activo, id, organizationID)
	if err != nil {
		return fmt.Errorf("error al actualizar regla de alerta: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *AlertaRepository) EliminarConfig(ctx context.Context, organizationID, id int) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM config_alertas WHERE id = $1 AND organization_id = $2", id, organizationID)
	if err != nil {
		return fmt.Errorf("error al eliminar regla de alerta: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

// --- Alertas generadas/enviadas ---

const alertaSelectBase = `
	SELECT id, organization_id, vehiculo_id, documento_id, tipo_alerta, canal,
		fecha_programada, fecha_envio, destinatario, estado_envio, detalle_error, leida, fecha_creacion
	FROM alertas
`

func scanAlerta(row interface {
	Scan(dest ...interface{}) error
}) (*models.Alerta, error) {
	var a models.Alerta
	err := row.Scan(
		&a.ID, &a.OrganizationID, &a.VehiculoID, &a.DocumentoID, &a.TipoAlerta, &a.Canal,
		&a.FechaProgramada, &a.FechaEnvio, &a.Destinatario, &a.EstadoEnvio, &a.DetalleError, &a.Leida, &a.FechaCreacion,
	)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// YaExisteAlertaHoy evita duplicar el envío del mismo nivel de alerta el mismo día.
func (r *AlertaRepository) YaExisteAlertaHoy(ctx context.Context, documentoID int, nivel string) (bool, error) {
	query := `
		SELECT COUNT(*) FROM alertas
		WHERE documento_id = $1 AND tipo_alerta = $2 AND fecha_programada = CURRENT_DATE
	`
	var count int
	if err := r.db.QueryRowContext(ctx, query, documentoID, nivel).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

// Crear inserta una nueva alerta pendiente de envío.
func (r *AlertaRepository) Crear(ctx context.Context, a *models.Alerta) (int, error) {
	query := `
		INSERT INTO alertas (
			organization_id, vehiculo_id, documento_id, tipo_alerta, canal,
			fecha_programada, destinatario, estado_envio, leida
		)
		VALUES ($1, $2, $3, $4, $5, CURRENT_DATE, $6, 'Pendiente', FALSE)
		RETURNING id
	`
	var newID int
	err := r.db.QueryRowContext(ctx, query,
		a.OrganizationID, a.VehiculoID, a.DocumentoID, a.TipoAlerta, a.Canal, a.Destinatario,
	).Scan(&newID)
	if err != nil {
		return 0, fmt.Errorf("error al crear alerta: %w", err)
	}
	return newID, nil
}

// ListarPendientes retorna las alertas que aún no han sido enviadas (para el job de envío).
func (r *AlertaRepository) ListarPendientes(ctx context.Context) ([]models.Alerta, error) {
	query := alertaSelectBase + " WHERE estado_envio = 'Pendiente' ORDER BY fecha_programada"
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("error al listar alertas pendientes: %w", err)
	}
	defer rows.Close()

	var alertas []models.Alerta
	for rows.Next() {
		a, err := scanAlerta(rows)
		if err != nil {
			return nil, err
		}
		alertas = append(alertas, *a)
	}
	return alertas, nil
}

// MarcarResultado actualiza el resultado del envío (éxito o error).
func (r *AlertaRepository) MarcarResultado(ctx context.Context, id int, exito bool, detalleError *string) error {
	estado := "Enviada"
	if !exito {
		estado = "Error"
	}
	query := `
		UPDATE alertas SET estado_envio = $1, fecha_envio = NOW(), detalle_error = $2
		WHERE id = $3
	`
	_, err := r.db.ExecContext(ctx, query, estado, detalleError, id)
	if err != nil {
		return fmt.Errorf("error al actualizar resultado de alerta: %w", err)
	}
	return nil
}

// ListarPorOrganizacion retorna alertas recientes para el panel de alertas del frontend.
func (r *AlertaRepository) ListarPorOrganizacion(ctx context.Context, organizationID int, soloPendientes bool) ([]models.Alerta, error) {
	query := alertaSelectBase + " WHERE organization_id = $1"
	if soloPendientes {
		query += " AND estado_envio = 'Pendiente'"
	}
	query += " ORDER BY fecha_programada DESC"

	rows, err := r.db.QueryContext(ctx, query, organizationID)
	if err != nil {
		return nil, fmt.Errorf("error al listar alertas: %w", err)
	}
	defer rows.Close()

	var alertas []models.Alerta
	for rows.Next() {
		a, err := scanAlerta(rows)
		if err != nil {
			return nil, err
		}
		alertas = append(alertas, *a)
	}
	return alertas, nil
}

// MarcarLeida marca una alerta puntual como revisada por el usuario.
func (r *AlertaRepository) MarcarLeida(ctx context.Context, organizationID, id int) error {
	result, err := r.db.ExecContext(ctx, "UPDATE alertas SET leida = TRUE WHERE id = $1 AND organization_id = $2", id, organizationID)
	if err != nil {
		return fmt.Errorf("error al marcar alerta como leída: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

// MarcarTodasLeidas marca como leídas todas las alertas pendientes de lectura de la organización.
func (r *AlertaRepository) MarcarTodasLeidas(ctx context.Context, organizationID int) error {
	_, err := r.db.ExecContext(ctx, "UPDATE alertas SET leida = TRUE WHERE organization_id = $1 AND leida = FALSE", organizationID)
	if err != nil {
		return fmt.Errorf("error al marcar todas las alertas como leídas: %w", err)
	}
	return nil
}
