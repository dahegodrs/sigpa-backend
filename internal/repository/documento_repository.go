package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/alcaldia/sigpa-backend/internal/models"
	"github.com/alcaldia/sigpa-backend/pkg/apperrors"
)

type DocumentoRepository struct {
	db *sql.DB
}

func NewDocumentoRepository(db *sql.DB) *DocumentoRepository {
	return &DocumentoRepository{db: db}
}

const documentoSelectBase = `
	SELECT
		d.id, d.organization_id, d.vehiculo_id, d.tipo_documento_id, td.nombre AS tipo_documento_nombre,
		d.fecha_expedicion, d.fecha_vencimiento, d.estado_documento, d.archivo_url, d.archivo_drive_id,
		d.nombre_archivo, d.tamano_bytes, d.version, d.vigente_actual, d.observaciones, d.fecha_carga, d.cargado_por
	FROM documentos d
	INNER JOIN tipos_documento td ON td.id = d.tipo_documento_id
`

func scanDocumento(row interface {
	Scan(dest ...interface{}) error
}) (*models.Documento, error) {
	var d models.Documento
	err := row.Scan(
		&d.ID, &d.OrganizationID, &d.VehiculoID, &d.TipoDocumentoID, &d.TipoDocumentoNombre,
		&d.FechaExpedicion, &d.FechaVencimiento, &d.EstadoDocumento, &d.ArchivoURL, &d.ArchivoDriveID,
		&d.NombreArchivo, &d.TamanoBytes, &d.Version, &d.VigenteActual, &d.Observaciones, &d.FechaCarga, &d.CargadoPor,
	)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// ListarPorVehiculo retorna únicamente la versión vigente de cada tipo de documento del vehículo.
func (r *DocumentoRepository) ListarPorVehiculo(ctx context.Context, organizationID, vehiculoID int) ([]models.Documento, error) {
	query := documentoSelectBase + `
		WHERE d.organization_id = $1 AND d.vehiculo_id = $2 AND d.vigente_actual = TRUE
		ORDER BY td.nombre
	`
	rows, err := r.db.QueryContext(ctx, query, organizationID, vehiculoID)
	if err != nil {
		return nil, fmt.Errorf("error al listar documentos: %w", err)
	}
	defer rows.Close()

	var documentos []models.Documento
	for rows.Next() {
		d, err := scanDocumento(rows)
		if err != nil {
			return nil, fmt.Errorf("error al leer documento: %w", err)
		}
		documentos = append(documentos, *d)
	}
	return documentos, nil
}

// HistoricoPorTipo retorna todas las versiones anteriores de un tipo de documento de un vehículo.
func (r *DocumentoRepository) HistoricoPorTipo(ctx context.Context, organizationID, vehiculoID, tipoDocumentoID int) ([]models.Documento, error) {
	query := documentoSelectBase + `
		WHERE d.organization_id = $1 AND d.vehiculo_id = $2 AND d.tipo_documento_id = $3
		ORDER BY d.version DESC
	`
	rows, err := r.db.QueryContext(ctx, query, organizationID, vehiculoID, tipoDocumentoID)
	if err != nil {
		return nil, fmt.Errorf("error al consultar histórico de documento: %w", err)
	}
	defer rows.Close()

	var documentos []models.Documento
	for rows.Next() {
		d, err := scanDocumento(rows)
		if err != nil {
			return nil, err
		}
		documentos = append(documentos, *d)
	}
	return documentos, nil
}

// CrearNuevaVersion inserta un nuevo documento marcándolo como vigente y desactiva la versión anterior.
// Se ejecuta dentro de una transacción para mantener consistencia del versionado.
func (r *DocumentoRepository) CrearNuevaVersion(ctx context.Context, d *models.Documento) (int, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("error al iniciar transacción: %w", err)
	}
	defer tx.Rollback()

	// Determinar la siguiente versión y desactivar la anterior, si existe.
	var maxVersion sql.NullInt32
	err = tx.QueryRowContext(ctx, `
		SELECT MAX(version) FROM documentos
		WHERE vehiculo_id = $1 AND tipo_documento_id = $2
	`, d.VehiculoID, d.TipoDocumentoID).Scan(&maxVersion)
	if err != nil {
		return 0, fmt.Errorf("error al calcular versión: %w", err)
	}

	nextVersion := 1
	if maxVersion.Valid {
		nextVersion = int(maxVersion.Int32) + 1
		_, err = tx.ExecContext(ctx, `
			UPDATE documentos SET vigente_actual = FALSE
			WHERE vehiculo_id = $1 AND tipo_documento_id = $2 AND vigente_actual = TRUE
		`, d.VehiculoID, d.TipoDocumentoID)
		if err != nil {
			return 0, fmt.Errorf("error al archivar versión anterior: %w", err)
		}
	}

	var newID int
	insertQuery := `
		INSERT INTO documentos (
			organization_id, vehiculo_id, tipo_documento_id, fecha_expedicion, fecha_vencimiento,
			estado_documento, archivo_url, archivo_drive_id, nombre_archivo, tamano_bytes,
			version, vigente_actual, observaciones, cargado_por
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, TRUE, $12, $13)
		RETURNING id
	`
	err = tx.QueryRowContext(ctx, insertQuery,
		d.OrganizationID, d.VehiculoID, d.TipoDocumentoID, d.FechaExpedicion, d.FechaVencimiento,
		d.EstadoDocumento, d.ArchivoURL, d.ArchivoDriveID, d.NombreArchivo, d.TamanoBytes,
		nextVersion, d.Observaciones, d.CargadoPor,
	).Scan(&newID)
	if err != nil {
		return 0, fmt.Errorf("error al insertar documento: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("error al confirmar transacción: %w", err)
	}

	return newID, nil
}

// GetByID obtiene un documento vigente por ID.
func (r *DocumentoRepository) GetByID(ctx context.Context, organizationID, id int) (*models.Documento, error) {
	query := documentoSelectBase + " WHERE d.id = $1 AND d.organization_id = $2"
	row := r.db.QueryRowContext(ctx, query, id, organizationID)
	d, err := scanDocumento(row)
	if err == sql.ErrNoRows {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("error al consultar documento: %w", err)
	}
	return d, nil
}

// ListarProximosAVencerYVencidos es usado por el job diario de alertas.
// Retorna todos los documentos vigentes de todas las organizaciones cuyo
// estado calculado deba generar una alerta según config_alertas.
func (r *DocumentoRepository) ListarParaRevisionDiaria(ctx context.Context) ([]models.Documento, error) {
	query := documentoSelectBase + `
		WHERE d.vigente_actual = TRUE AND d.fecha_vencimiento IS NOT NULL
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("error al listar documentos para revisión diaria: %w", err)
	}
	defer rows.Close()

	var documentos []models.Documento
	for rows.Next() {
		d, err := scanDocumento(rows)
		if err != nil {
			return nil, err
		}
		documentos = append(documentos, *d)
	}
	return documentos, nil
}

// ActualizarEstado sincroniza el campo estado_documento calculado (job diario / al consultar).
func (r *DocumentoRepository) ActualizarEstado(ctx context.Context, id int, estado models.EstadoDocumento) error {
	_, err := r.db.ExecContext(ctx, "UPDATE documentos SET estado_documento = $1 WHERE id = $2", estado, id)
	if err != nil {
		return fmt.Errorf("error al actualizar estado de documento: %w", err)
	}
	return nil
}

// DocumentoFiltroGlobal agrupa los filtros de la Gestión Documental global
// (todos los vehículos), a diferencia de ListarPorVehiculo que ya viene
// acotado a un solo vehículo.
type DocumentoFiltroGlobal struct {
	OrganizationID  int
	TipoDocumentoID *int
	EstadoDocumento *string
	Placa           string
	DependenciaID   *int
	Page            int
	PageSize        int
}

const documentoGlobalSelectBase = `
	SELECT
		d.id, d.organization_id, d.vehiculo_id, v.placa AS vehiculo_placa,
		d.tipo_documento_id, td.nombre AS tipo_documento_nombre,
		d.fecha_expedicion, d.fecha_vencimiento, d.estado_documento, d.archivo_url, d.archivo_drive_id,
		d.nombre_archivo, d.tamano_bytes, d.version, d.vigente_actual, d.observaciones, d.fecha_carga, d.cargado_por
	FROM documentos d
	INNER JOIN tipos_documento td ON td.id = d.tipo_documento_id
	INNER JOIN vehiculos v ON v.id = d.vehiculo_id
`

func scanDocumentoGlobal(row interface {
	Scan(dest ...interface{}) error
}) (*models.Documento, error) {
	var d models.Documento
	err := row.Scan(
		&d.ID, &d.OrganizationID, &d.VehiculoID, &d.VehiculoPlaca,
		&d.TipoDocumentoID, &d.TipoDocumentoNombre,
		&d.FechaExpedicion, &d.FechaVencimiento, &d.EstadoDocumento, &d.ArchivoURL, &d.ArchivoDriveID,
		&d.NombreArchivo, &d.TamanoBytes, &d.Version, &d.VigenteActual, &d.Observaciones, &d.FechaCarga, &d.CargadoPor,
	)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// ListarGlobal retorna documentos de TODOS los vehículos de la organización,
// para la pantalla de Gestión Documental (a diferencia de ListarPorVehiculo,
// que solo trae los de un vehículo puntual).
func (r *DocumentoRepository) ListarGlobal(ctx context.Context, f DocumentoFiltroGlobal) ([]models.Documento, int64, error) {
	conditions := []string{"d.organization_id = $1", "d.vigente_actual = TRUE"}
	args := []interface{}{f.OrganizationID}
	argPos := 2

	if f.TipoDocumentoID != nil {
		conditions = append(conditions, fmt.Sprintf("d.tipo_documento_id = $%d", argPos))
		args = append(args, *f.TipoDocumentoID)
		argPos++
	}
	if f.EstadoDocumento != nil && *f.EstadoDocumento != "" {
		conditions = append(conditions, fmt.Sprintf("d.estado_documento = $%d", argPos))
		args = append(args, *f.EstadoDocumento)
		argPos++
	}
	if f.Placa != "" {
		conditions = append(conditions, fmt.Sprintf("v.placa ILIKE $%d", argPos))
		args = append(args, "%"+f.Placa+"%")
		argPos++
	}
	if f.DependenciaID != nil {
		conditions = append(conditions, fmt.Sprintf("v.dependencia_id = $%d", argPos))
		args = append(args, *f.DependenciaID)
		argPos++
	}

	whereClause := strings.Join(conditions, " AND ")

	countQuery := `
		SELECT COUNT(*) FROM documentos d INNER JOIN vehiculos v ON v.id = d.vehiculo_id WHERE ` + whereClause
	var total int64
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("error al contar documentos: %w", err)
	}

	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 || f.PageSize > 200 {
		f.PageSize = 24
	}
	offset := (f.Page - 1) * f.PageSize

	dataQuery := documentoGlobalSelectBase + " WHERE " + whereClause +
		fmt.Sprintf(" ORDER BY d.fecha_carga DESC LIMIT $%d OFFSET $%d", argPos, argPos+1)
	args = append(args, f.PageSize, offset)

	rows, err := r.db.QueryContext(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("error al listar documentos: %w", err)
	}
	defer rows.Close()

	var documentos []models.Documento
	for rows.Next() {
		d, err := scanDocumentoGlobal(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("error al leer documento: %w", err)
		}
		documentos = append(documentos, *d)
	}
	return documentos, total, nil
}

// ConteoPorTipoDocumento agrupa cantidad de archivos y tamaño total por tipo
// de documento, para las tarjetas de "carpetas" de la Gestión Documental.
type ConteoPorTipoDocumento struct {
	TipoDocumentoID     int    `json:"tipo_documento_id"`
	TipoDocumentoNombre string `json:"tipo_documento_nombre"`
	Total               int    `json:"total"`
	TamanoTotalBytes    int64  `json:"tamano_total_bytes"`
}

func (r *DocumentoRepository) ConteoPorTipo(ctx context.Context, organizationID int) ([]ConteoPorTipoDocumento, error) {
	query := `
		SELECT td.id, td.nombre, COUNT(d.id), COALESCE(SUM(d.tamano_bytes), 0)
		FROM tipos_documento td
		LEFT JOIN documentos d ON d.tipo_documento_id = td.id AND d.vigente_actual = TRUE AND d.organization_id = $1
		GROUP BY td.id, td.nombre
		ORDER BY td.id
	`
	rows, err := r.db.QueryContext(ctx, query, organizationID)
	if err != nil {
		return nil, fmt.Errorf("error al calcular conteo por tipo de documento: %w", err)
	}
	defer rows.Close()

	var resultados []ConteoPorTipoDocumento
	for rows.Next() {
		var c ConteoPorTipoDocumento
		if err := rows.Scan(&c.TipoDocumentoID, &c.TipoDocumentoNombre, &c.Total, &c.TamanoTotalBytes); err != nil {
			return nil, err
		}
		resultados = append(resultados, c)
	}
	return resultados, nil
}
