package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/alcaldia/sigpa-backend/internal/models"
	"github.com/alcaldia/sigpa-backend/pkg/apperrors"
)

type VehiculoRepository struct {
	db *sql.DB
}

func NewVehiculoRepository(db *sql.DB) *VehiculoRepository {
	return &VehiculoRepository{db: db}
}

const vehiculoSelectBase = `
	SELECT
		v.id, v.organization_id, v.placa, v.tipo_vehiculo_id, tv.nombre AS tipo_vehiculo_nombre,
		v.marca, v.linea, v.modelo, v.color, v.motor, v.chasis, v.vin, v.capacidad, v.combustible,
		v.dependencia_id, COALESCE(d.nombre, '') AS dependencia_nombre,
		v.responsable_id, COALESCE(u.nombre, '') AS responsable_nombre,
		v.estado_id, ev.nombre AS estado_nombre,
		v.ubicacion, v.observaciones, v.activo, v.fecha_creacion, v.fecha_actualizacion
	FROM vehiculos v
	INNER JOIN tipos_vehiculo tv ON tv.id = v.tipo_vehiculo_id
	INNER JOIN estados_vehiculo ev ON ev.id = v.estado_id
	LEFT JOIN dependencias d ON d.id = v.dependencia_id
	LEFT JOIN usuarios u ON u.id = v.responsable_id
`

func scanVehiculo(row interface {
	Scan(dest ...interface{}) error
}) (*models.Vehiculo, error) {
	var v models.Vehiculo
	err := row.Scan(
		&v.ID, &v.OrganizationID, &v.Placa, &v.TipoVehiculoID, &v.TipoVehiculoNombre,
		&v.Marca, &v.Linea, &v.Modelo, &v.Color, &v.Motor, &v.Chasis, &v.VIN, &v.Capacidad, &v.Combustible,
		&v.DependenciaID, &v.DependenciaNombre,
		&v.ResponsableID, &v.ResponsableNombre,
		&v.EstadoID, &v.EstadoNombre,
		&v.Ubicacion, &v.Observaciones, &v.Activo, &v.FechaCreacion, &v.FechaActualizacion,
	)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// GetByID obtiene un vehículo por ID, filtrado siempre por organización (tenant).
func (r *VehiculoRepository) GetByID(ctx context.Context, organizationID, id int) (*models.Vehiculo, error) {
	query := vehiculoSelectBase + " WHERE v.id = $1 AND v.organization_id = $2 AND v.activo = TRUE"
	row := r.db.QueryRowContext(ctx, query, id, organizationID)

	v, err := scanVehiculo(row)
	if err == sql.ErrNoRows {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("error al consultar vehículo: %w", err)
	}
	return v, nil
}

// List devuelve vehículos paginados aplicando los filtros recibidos.
// vehiculoListSelectBase es una variante de vehiculoSelectBase usada SOLO en
// List(), que además trae el estado vigente de SOAT/Tecnomecánica/Póliza
// mediante LEFT JOIN, para que la tabla de vehículos muestre el semáforo
// documental sin tener que entrar a cada ficha.
const vehiculoListSelectBase = `
	SELECT
		v.id, v.organization_id, v.placa, v.tipo_vehiculo_id, tv.nombre AS tipo_vehiculo_nombre,
		v.marca, v.linea, v.modelo, v.color, v.motor, v.chasis, v.vin, v.capacidad, v.combustible,
		v.dependencia_id, COALESCE(d.nombre, '') AS dependencia_nombre,
		v.responsable_id, COALESCE(u.nombre, '') AS responsable_nombre,
		v.estado_id, ev.nombre AS estado_nombre,
		v.ubicacion, v.observaciones, v.activo, v.fecha_creacion, v.fecha_actualizacion,
		soat.estado_documento, tecno.estado_documento, poliza.estado_documento
	FROM vehiculos v
	INNER JOIN tipos_vehiculo tv ON tv.id = v.tipo_vehiculo_id
	INNER JOIN estados_vehiculo ev ON ev.id = v.estado_id
	LEFT JOIN dependencias d ON d.id = v.dependencia_id
	LEFT JOIN usuarios u ON u.id = v.responsable_id
	LEFT JOIN documentos soat ON soat.vehiculo_id = v.id AND soat.vigente_actual = TRUE
		AND soat.tipo_documento_id IN (SELECT id FROM tipos_documento WHERE nombre = 'SOAT')
	LEFT JOIN documentos tecno ON tecno.vehiculo_id = v.id AND tecno.vigente_actual = TRUE
		AND tecno.tipo_documento_id IN (SELECT id FROM tipos_documento WHERE nombre = 'Tecnomecánica')
	LEFT JOIN documentos poliza ON poliza.vehiculo_id = v.id AND poliza.vigente_actual = TRUE
		AND poliza.tipo_documento_id IN (SELECT id FROM tipos_documento WHERE nombre = 'Póliza de seguros')
`

func scanVehiculoConDocumentos(row interface {
	Scan(dest ...interface{}) error
}) (*models.Vehiculo, error) {
	var v models.Vehiculo
	err := row.Scan(
		&v.ID, &v.OrganizationID, &v.Placa, &v.TipoVehiculoID, &v.TipoVehiculoNombre,
		&v.Marca, &v.Linea, &v.Modelo, &v.Color, &v.Motor, &v.Chasis, &v.VIN, &v.Capacidad, &v.Combustible,
		&v.DependenciaID, &v.DependenciaNombre,
		&v.ResponsableID, &v.ResponsableNombre,
		&v.EstadoID, &v.EstadoNombre,
		&v.Ubicacion, &v.Observaciones, &v.Activo, &v.FechaCreacion, &v.FechaActualizacion,
		&v.SoatEstado, &v.TecnoEstado, &v.PolizaEstado,
	)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (r *VehiculoRepository) List(ctx context.Context, f models.VehiculoFiltro) ([]models.Vehiculo, int64, error) {
	conditions := []string{"v.organization_id = $1", "v.activo = TRUE"}
	args := []interface{}{f.OrganizationID}
	argPos := 2

	addCondition := func(cond string, val interface{}) {
		conditions = append(conditions, fmt.Sprintf(cond, argPos))
		args = append(args, val)
		argPos++
	}

	if f.Placa != "" {
		addCondition("v.placa ILIKE $%d", "%"+f.Placa+"%")
	}
	if f.DependenciaID != nil {
		addCondition("v.dependencia_id = $%d", *f.DependenciaID)
	}
	if f.TipoVehiculoID != nil {
		addCondition("v.tipo_vehiculo_id = $%d", *f.TipoVehiculoID)
	}
	if f.EstadoID != nil {
		addCondition("v.estado_id = $%d", *f.EstadoID)
	}
	if f.ResponsableID != nil {
		addCondition("v.responsable_id = $%d", *f.ResponsableID)
	}
	if f.Marca != "" {
		addCondition("v.marca ILIKE $%d", "%"+f.Marca+"%")
	}
	if f.Modelo != nil {
		addCondition("v.modelo = $%d", *f.Modelo)
	}

	whereClause := strings.Join(conditions, " AND ")

	// Total de registros para la paginación
	countQuery := "SELECT COUNT(*) FROM vehiculos v WHERE " + whereClause
	var total int64
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("error al contar vehículos: %w", err)
	}

	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 || f.PageSize > 200 {
		f.PageSize = 20
	}
	offset := (f.Page - 1) * f.PageSize

	dataQuery := vehiculoListSelectBase + " WHERE " + whereClause +
		fmt.Sprintf(" ORDER BY v.placa LIMIT $%d OFFSET $%d", argPos, argPos+1)
	args = append(args, f.PageSize, offset)

	rows, err := r.db.QueryContext(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("error al listar vehículos: %w", err)
	}
	defer rows.Close()

	var vehiculos []models.Vehiculo
	for rows.Next() {
		v, err := scanVehiculoConDocumentos(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("error al leer vehículo: %w", err)
		}
		vehiculos = append(vehiculos, *v)
	}

	return vehiculos, total, nil
}

// ExistePlaca valida unicidad de placa dentro de la organización (para creación/edición).
func (r *VehiculoRepository) ExistePlaca(ctx context.Context, organizationID int, placa string, excluirID *int) (bool, error) {
	query := "SELECT COUNT(*) FROM vehiculos WHERE organization_id = $1 AND placa = $2 AND activo = TRUE"
	args := []interface{}{organizationID, placa}
	if excluirID != nil {
		query += " AND id <> $3"
		args = append(args, *excluirID)
	}
	var count int
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

// Create inserta un nuevo vehículo y retorna su ID.
func (r *VehiculoRepository) Create(ctx context.Context, v *models.Vehiculo) (int, error) {
	query := `
		INSERT INTO vehiculos (
			organization_id, placa, tipo_vehiculo_id, marca, linea, modelo, color, motor, chasis, vin,
			capacidad, combustible, dependencia_id, responsable_id, estado_id, ubicacion, observaciones
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
		RETURNING id
	`
	var newID int
	err := r.db.QueryRowContext(ctx, query,
		v.OrganizationID, v.Placa, v.TipoVehiculoID, v.Marca, v.Linea, v.Modelo, v.Color, v.Motor, v.Chasis, v.VIN,
		v.Capacidad, v.Combustible, v.DependenciaID, v.ResponsableID, v.EstadoID, v.Ubicacion, v.Observaciones,
	).Scan(&newID)
	if err != nil {
		return 0, fmt.Errorf("error al crear vehículo: %w", err)
	}
	return newID, nil
}

// Update actualiza un vehículo existente (siempre acotado a la organización).
func (r *VehiculoRepository) Update(ctx context.Context, v *models.Vehiculo) error {
	query := `
		UPDATE vehiculos SET
			tipo_vehiculo_id = $1, marca = $2, linea = $3, modelo = $4, color = $5, motor = $6, chasis = $7, vin = $8,
			capacidad = $9, combustible = $10, dependencia_id = $11, responsable_id = $12,
			estado_id = $13, ubicacion = $14, observaciones = $15, fecha_actualizacion = NOW()
		WHERE id = $16 AND organization_id = $17
	`
	result, err := r.db.ExecContext(ctx, query,
		v.TipoVehiculoID, v.Marca, v.Linea, v.Modelo, v.Color, v.Motor, v.Chasis, v.VIN,
		v.Capacidad, v.Combustible, v.DependenciaID, v.ResponsableID,
		v.EstadoID, v.Ubicacion, v.Observaciones, v.ID, v.OrganizationID,
	)
	if err != nil {
		return fmt.Errorf("error al actualizar vehículo: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

// ObtenerDependenciaYResponsable retorna el dependencia_id y el email del
// responsable (si tiene) de un vehículo. Se usa para resolver destinatarios
// de alertas sin acoplar el repositorio de alertas al de vehículos/usuarios.
func (r *VehiculoRepository) ObtenerDependenciaYResponsable(ctx context.Context, vehiculoID int) (dependenciaID *int, emailResponsable *string, err error) {
	query := `
		SELECT v.dependencia_id, u.email
		FROM vehiculos v
		LEFT JOIN usuarios u ON u.id = v.responsable_id
		WHERE v.id = $1
	`
	row := r.db.QueryRowContext(ctx, query, vehiculoID)
	if scanErr := row.Scan(&dependenciaID, &emailResponsable); scanErr != nil {
		if scanErr == sql.ErrNoRows {
			return nil, nil, apperrors.ErrNotFound
		}
		return nil, nil, fmt.Errorf("error al resolver responsable del vehículo: %w", scanErr)
	}
	return dependenciaID, emailResponsable, nil
}

// SoftDelete marca el vehículo como inactivo (dado de baja lógica del registro).
func (r *VehiculoRepository) SoftDelete(ctx context.Context, organizationID, id int) error {
	query := "UPDATE vehiculos SET activo = FALSE, fecha_actualizacion = NOW() WHERE id = $1 AND organization_id = $2"
	result, err := r.db.ExecContext(ctx, query, id, organizationID)
	if err != nil {
		return fmt.Errorf("error al eliminar vehículo: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}
