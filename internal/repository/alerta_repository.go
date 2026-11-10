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
	SELECT a.id, a.organization_id, a.vehiculo_id, a.documento_id, a.tipo_alerta, a.canal,
		a.fecha_programada, a.fecha_envio, a.destinatario, a.estado_envio, a.detalle_error, a.leida, a.fecha_creacion
	FROM alertas a
`

// alertaSelectEnriquecido agrega los datos de vehículo y documento vía JOIN,
// para que el frontend pueda mostrar mensajes claros ("SOAT vencido hace 5
// días") sin tener que hacer una consulta adicional por cada alerta.
// Incluye también los campos de descarte para que el frontend pueda
// mostrar quién y cuándo descartó cada alerta.
const alertaSelectEnriquecido = `
	SELECT a.id, a.organization_id, a.vehiculo_id, a.documento_id, a.tipo_alerta, a.canal,
		a.fecha_programada, a.fecha_envio, a.destinatario, a.estado_envio, a.detalle_error, a.leida, a.fecha_creacion,
		a.descartada, a.descartada_por, a.fecha_descarte,
		COALESCE(v.placa, '') AS vehiculo_placa,
		COALESCE(td.nombre, '') AS tipo_documento_nombre,
		d.fecha_vencimiento AS documento_fecha_vencimiento
	FROM alertas a
	LEFT JOIN vehiculos v ON v.id = a.vehiculo_id
	INNER JOIN documentos d ON d.id = a.documento_id AND d.eliminado = FALSE
	LEFT JOIN tipos_documento td ON td.id = d.tipo_documento_id
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

func scanAlertaEnriquecida(row interface {
	Scan(dest ...interface{}) error
}) (*models.Alerta, error) {
	var a models.Alerta
	err := row.Scan(
		&a.ID, &a.OrganizationID, &a.VehiculoID, &a.DocumentoID, &a.TipoAlerta, &a.Canal,
		&a.FechaProgramada, &a.FechaEnvio, &a.Destinatario, &a.EstadoEnvio, &a.DetalleError, &a.Leida, &a.FechaCreacion,
		&a.Descartada, &a.DescartadaPor, &a.FechaDescarte,
		&a.VehiculoPlaca, &a.TipoDocumentoNombre, &a.DocumentoFechaVenc,
	)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// YaExisteAlertaActiva verifica si ya existe una alerta NO descartada para
// ese documento y nivel — impide duplicar cuando el motor corre en días
// consecutivos dentro del mismo rango de umbral.
// Nueva lógica "por rango": una alerta se genera si diasRestantes <= umbral
// Y no existe ya una alerta activa (no descartada) del mismo nivel.
func (r *AlertaRepository) YaExisteAlertaActiva(ctx context.Context, documentoID int, nivel string) (bool, error) {
	query := `
		SELECT COUNT(*) FROM alertas
		WHERE documento_id = $1 AND tipo_alerta = $2 AND descartada = FALSE
	`
	var count int
	if err := r.db.QueryRowContext(ctx, query, documentoID, nivel).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

// YaExisteAlertaHoy — alias de compatibilidad con la lógica anterior.
// Redirige a YaExisteAlertaActiva para usar la nueva semántica por rango.
func (r *AlertaRepository) YaExisteAlertaHoy(ctx context.Context, documentoID int, nivel string) (bool, error) {
	return r.YaExisteAlertaActiva(ctx, documentoID, nivel)
}

// DescartarAlerta marca como descartadas TODAS las filas del mismo
// documento + nivel para esta organización, no solo la fila representante
// que devuelve DISTINCT ON.
//
// Contexto: por cada alerta se crea una fila por destinatario de correo
// (ej. 6 destinatarios → 6 filas con el mismo documento_id + tipo_alerta).
// Al mostrar en UI se usa DISTINCT ON para agrupar en una sola tarjeta,
// pero al descartar es necesario marcar todas las filas del grupo para que
// no vuelvan a aparecer al recargar la página.
func (r *AlertaRepository) DescartarAlerta(ctx context.Context, organizationID, alertaID, usuarioID int) error {
	// Paso 1: obtener documento_id + tipo_alerta de la alerta indicada.
	var documentoID int
	var tipoAlerta string
	err := r.db.QueryRowContext(ctx,
		"SELECT documento_id, tipo_alerta FROM alertas WHERE id = $1 AND organization_id = $2",
		alertaID, organizationID,
	).Scan(&documentoID, &tipoAlerta)
	if err == sql.ErrNoRows {
		return apperrors.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("error al obtener datos de la alerta: %w", err)
	}

	// Paso 2: descartar todo el grupo (mismo documento + nivel).
	result, err := r.db.ExecContext(ctx, `
		UPDATE alertas
		SET descartada = TRUE, descartada_por = $1, fecha_descarte = NOW(), leida = TRUE
		WHERE organization_id = $2 AND documento_id = $3 AND tipo_alerta = $4 AND descartada = FALSE
	`, usuarioID, organizationID, documentoID, tipoAlerta)
	if err != nil {
		return fmt.Errorf("error al descartar alertas del grupo: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

// Crear inserta una nueva alerta pendiente de envío.
func (r *AlertaRepository) Crear(ctx context.Context, a *models.Alerta) (int, error) {
	query := `
		INSERT INTO alertas (
			organization_id, vehiculo_id, documento_id, tipo_alerta, canal,
			fecha_programada, destinatario, estado_envio, leida, descartada
		)
		VALUES ($1, $2, $3, $4, $5, CURRENT_DATE, $6, 'Pendiente', FALSE, FALSE)
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

// ListarPorOrganizacion retorna alertas activas (no descartadas) para el
// Centro de Alertas del frontend, enriquecidas con placa, tipo de documento
// y fecha de vencimiento.
//
// Agrupación visual: aunque en BD existen N filas por alerta (una por
// destinatario de correo), el Centro de Alertas debe mostrar UNA SOLA
// tarjeta por documento/nivel — el usuario no necesita ver "SOAT vencido"
// repetido 6 veces porque se envió a 6 correos distintos.
// Se usa DISTINCT ON (documento_id, tipo_alerta) para colapsar, eligiendo
// la fila con el id más alto (más reciente) de cada grupo.
func (r *AlertaRepository) ListarPorOrganizacion(ctx context.Context, organizationID int, soloPendientes bool) ([]models.Alerta, error) {
	// Subconsulta con DISTINCT ON para colapsar filas duplicadas por destinatario.
	// El ORDER BY interno (documento_id, tipo_alerta, id DESC) asegura que
	// DISTINCT ON tome siempre la fila más reciente por grupo.
	pendientesClause := ""
	if soloPendientes {
		pendientesClause = " AND a.leida = FALSE"
	}

	query := fmt.Sprintf(`
		SELECT DISTINCT ON (a.documento_id, a.tipo_alerta)
			a.id, a.organization_id, a.vehiculo_id, a.documento_id, a.tipo_alerta, a.canal,
			a.fecha_programada, a.fecha_envio, a.destinatario, a.estado_envio, a.detalle_error, a.leida, a.fecha_creacion,
			a.descartada, a.descartada_por, a.fecha_descarte,
			COALESCE(v.placa, '') AS vehiculo_placa,
			COALESCE(td.nombre, '') AS tipo_documento_nombre,
			d.fecha_vencimiento AS documento_fecha_vencimiento
		FROM alertas a
		LEFT JOIN vehiculos v ON v.id = a.vehiculo_id
		INNER JOIN documentos d ON d.id = a.documento_id AND d.eliminado = FALSE
		LEFT JOIN tipos_documento td ON td.id = d.tipo_documento_id
		WHERE a.organization_id = $1 AND a.descartada = FALSE%s
		ORDER BY a.documento_id, a.tipo_alerta, a.id DESC
	`, pendientesClause)

	rows, err := r.db.QueryContext(ctx, query, organizationID)
	if err != nil {
		return nil, fmt.Errorf("error al listar alertas: %w", err)
	}
	defer rows.Close()

	var alertas []models.Alerta
	for rows.Next() {
		a, err := scanAlertaEnriquecida(rows)
		if err != nil {
			return nil, err
		}
		alertas = append(alertas, *a)
	}
	// Re-ordenar por fecha_programada DESC para mantener orden descendente
	// (DISTINCT ON fuerza un ORDER BY que no es el deseado para presentación).
	for i, j := 0, len(alertas)-1; i < j; i, j = i+1, j-1 {
		alertas[i], alertas[j] = alertas[j], alertas[i]
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
	_, err := r.db.ExecContext(ctx, "UPDATE alertas SET leida = TRUE WHERE organization_id = $1 AND leida = FALSE AND descartada = FALSE", organizationID)
	if err != nil {
		return fmt.Errorf("error al marcar todas las alertas como leídas: %w", err)
	}
	return nil
}

// ObtenerCoberturaHoy resume la cobertura del motor de alertas:
// cuántos documentos próximos a vencer tienen ya una alerta activa hoy.
func (r *AlertaRepository) ObtenerCoberturaHoy(ctx context.Context, organizationID int) (*models.CoberturaAlertas, error) {
	const q = `
WITH reglas AS (
	SELECT dias_antes
	FROM config_alertas
	WHERE organization_id = $1 AND activo = TRUE
),
proximos AS (
	SELECT d.id, d.fecha_vencimiento::date AS fecha_venc
	FROM documentos d
	WHERE d.organization_id = $1
	  AND d.eliminado = FALSE
	  AND d.fecha_vencimiento IS NOT NULL
	  AND d.fecha_vencimiento::date > CURRENT_DATE
),
coinciden_hoy AS (
	SELECT p.id
	FROM proximos p
	JOIN reglas r ON (p.fecha_venc - CURRENT_DATE) <= r.dias_antes
	GROUP BY p.id
),
alertados_hoy AS (
	SELECT DISTINCT a.documento_id
	FROM alertas a
	WHERE a.organization_id = $1
	  AND a.descartada = FALSE
)
SELECT
	(SELECT COUNT(*) FROM proximos) AS proximos_total,
	(SELECT COUNT(*) FROM coinciden_hoy) AS coinciden_umbral_hoy,
	(SELECT COUNT(*) FROM alertados_hoy) AS alertas_generadas_hoy,
	(
		SELECT COUNT(*)
		FROM proximos p
		WHERE NOT EXISTS (
			SELECT 1
			FROM alertados_hoy ah
			WHERE ah.documento_id = p.id
		)
	) AS documentos_proximos_sin_alerta_hoy
`
	var c models.CoberturaAlertas
	if err := r.db.QueryRowContext(ctx, q, organizationID).Scan(
		&c.ProximosTotal,
		&c.CoincidenUmbralHoy,
		&c.AlertasGeneradasHoy,
		&c.DocumentosProximosSinAlertaHoy,
	); err != nil {
		return nil, fmt.Errorf("error al obtener cobertura de alertas: %w", err)
	}
	return &c, nil
}
