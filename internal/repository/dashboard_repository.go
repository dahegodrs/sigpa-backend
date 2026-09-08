package repository

import (
	"context"
	"database/sql"
	"fmt"
)

type DashboardRepository struct {
	db *sql.DB
}

func NewDashboardRepository(db *sql.DB) *DashboardRepository {
	return &DashboardRepository{db: db}
}

// ConteoPorCampo es un resultado genérico "etiqueta -> cantidad" usado en varias
// tarjetas y gráficos (por estado, por tipo, por dependencia, etc.)
type ConteoPorCampo struct {
	Etiqueta string `json:"etiqueta"`
	Total    int    `json:"total"`
}

// ResumenKPIs agrupa las tarjetas numéricas principales del dashboard.
type ResumenKPIs struct {
	TotalVehiculos     int     `json:"total_vehiculos"`
	VehiculosActivos   int     `json:"vehiculos_activos"`
	VehiculosReposo    int     `json:"vehiculos_en_reposo"`
	VehiculosMantenim  int     `json:"vehiculos_en_mantenimiento"`
	VehiculosComodato  int     `json:"vehiculos_en_comodato"`
	VehiculosBaja      int     `json:"vehiculos_dados_de_baja"`
	SoatVigentes       int     `json:"soat_vigentes"`
	SoatProximos       int     `json:"soat_proximos_a_vencer"`
	SoatVencidos       int     `json:"soat_vencidos"`
	TecnoVigentes      int     `json:"tecno_vigentes"`
	TecnoProximos      int     `json:"tecno_proximos_a_vencer"`
	TecnoVencidos      int     `json:"tecno_vencidos"`
	PolizasVigentes    int     `json:"polizas_vigentes"`
	PolizasProximos    int     `json:"polizas_proximas_a_vencer"`
	PolizasVencidas    int     `json:"polizas_vencidas"`
	SaludDocumentalPct float64 `json:"salud_documental_pct"`
}

func (r *DashboardRepository) ObtenerKPIs(ctx context.Context, organizationID int) (*ResumenKPIs, error) {
	var k ResumenKPIs

	query := `
		SELECT
			COUNT(*) AS total,
			COALESCE(SUM(CASE WHEN ev.nombre = 'Activo' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN ev.nombre = 'En reposo' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN ev.nombre = 'En mantenimiento' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN ev.nombre = 'En comodato' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN ev.nombre = 'Dado de baja' THEN 1 ELSE 0 END), 0)
		FROM vehiculos v
		INNER JOIN estados_vehiculo ev ON ev.id = v.estado_id
		WHERE v.organization_id = $1 AND v.activo = TRUE
	`
	if err := r.db.QueryRowContext(ctx, query, organizationID).Scan(
		&k.TotalVehiculos, &k.VehiculosActivos, &k.VehiculosReposo, &k.VehiculosMantenim,
		&k.VehiculosComodato, &k.VehiculosBaja,
	); err != nil {
		return nil, fmt.Errorf("error al calcular KPIs de vehículos: %w", err)
	}

	docQuery := `
		SELECT
			td.nombre,
			SUM(CASE WHEN d.estado_documento = 'Vigente' THEN 1 ELSE 0 END) AS vigentes,
			SUM(CASE WHEN d.estado_documento = 'Proximo_a_vencer' THEN 1 ELSE 0 END) AS proximos,
			SUM(CASE WHEN d.estado_documento = 'Vencido' THEN 1 ELSE 0 END) AS vencidos
		FROM documentos d
		INNER JOIN tipos_documento td ON td.id = d.tipo_documento_id
		WHERE d.organization_id = $1 AND d.vigente_actual = TRUE
			AND td.nombre IN ('SOAT', 'Tecnomecánica', 'Póliza de seguros')
		GROUP BY td.nombre
	`
	rows, err := r.db.QueryContext(ctx, docQuery, organizationID)
	if err != nil {
		return nil, fmt.Errorf("error al calcular KPIs documentales: %w", err)
	}
	defer rows.Close()

	var totalDocs, totalVigentes int
	for rows.Next() {
		var nombre string
		var vigentes, proximos, vencidos int
		if err := rows.Scan(&nombre, &vigentes, &proximos, &vencidos); err != nil {
			return nil, err
		}
		switch nombre {
		case "SOAT":
			k.SoatVigentes, k.SoatProximos, k.SoatVencidos = vigentes, proximos, vencidos
		case "Tecnomecánica":
			k.TecnoVigentes, k.TecnoProximos, k.TecnoVencidos = vigentes, proximos, vencidos
		case "Póliza de seguros":
			k.PolizasVigentes, k.PolizasProximos, k.PolizasVencidas = vigentes, proximos, vencidos
		}
		totalDocs += vigentes + proximos + vencidos
		totalVigentes += vigentes
	}

	if totalDocs > 0 {
		k.SaludDocumentalPct = (float64(totalVigentes) / float64(totalDocs)) * 100
	}

	return &k, nil
}

// PorDependencia agrupa vehículos activos por dependencia (gráfico de barras del dashboard).
func (r *DashboardRepository) PorDependencia(ctx context.Context, organizationID int) ([]ConteoPorCampo, error) {
	return r.conteoGenerico(ctx, `
		SELECT COALESCE(d.nombre, 'Sin asignar'), COUNT(*)
		FROM vehiculos v
		LEFT JOIN dependencias d ON d.id = v.dependencia_id
		WHERE v.organization_id = $1 AND v.activo = TRUE
		GROUP BY d.nombre
		ORDER BY COUNT(*) DESC
	`, organizationID)
}

// PorTipo agrupa vehículos por tipo, filtrando solo los que están en estado 'Activo'.
func (r *DashboardRepository) PorTipo(ctx context.Context, organizationID int) ([]ConteoPorCampo, error) {
	return r.conteoGenerico(ctx, `
		SELECT tv.nombre, COUNT(*)
		FROM vehiculos v
		INNER JOIN tipos_vehiculo tv ON tv.id = v.tipo_vehiculo_id
		INNER JOIN estados_vehiculo ev ON ev.id = v.estado_id
		WHERE v.organization_id = $1 AND v.activo = TRUE AND ev.nombre = 'Activo'
		GROUP BY tv.nombre
		ORDER BY COUNT(*) DESC
	`, organizationID)
}

// PorEstado agrupa vehículos por estado.
func (r *DashboardRepository) PorEstado(ctx context.Context, organizationID int) ([]ConteoPorCampo, error) {
	return r.conteoGenerico(ctx, `
		SELECT ev.nombre, COUNT(*)
		FROM vehiculos v
		INNER JOIN estados_vehiculo ev ON ev.id = v.estado_id
		WHERE v.organization_id = $1 AND v.activo = TRUE
		GROUP BY ev.nombre
		ORDER BY COUNT(*) DESC
	`, organizationID)
}

// VencimientosPorMes agrupa los documentos vigentes por mes de vencimiento (próximos 12 meses).
func (r *DashboardRepository) VencimientosPorMes(ctx context.Context, organizationID int) ([]ConteoPorCampo, error) {
	return r.conteoGenerico(ctx, `
		SELECT TO_CHAR(d.fecha_vencimiento, 'YYYY-MM'), COUNT(*)
		FROM documentos d
		WHERE d.organization_id = $1 AND d.vigente_actual = TRUE
			AND d.fecha_vencimiento BETWEEN CURRENT_DATE AND (CURRENT_DATE + INTERVAL '12 months')
		GROUP BY TO_CHAR(d.fecha_vencimiento, 'YYYY-MM')
		ORDER BY TO_CHAR(d.fecha_vencimiento, 'YYYY-MM')
	`, organizationID)
}

// VencimientoMensualPorTipo desglosa los vencimientos de un mes por tipo de
// documento, usado para el gráfico de líneas SOAT/Tecnomecánica/Póliza.
type VencimientoMensualPorTipo struct {
	Mes           string `json:"mes"`
	Soat          int    `json:"soat"`
	Tecnomecanica int    `json:"tecnomecanica"`
	Poliza        int    `json:"poliza"`
}

// VencimientosPorMesPorTipo agrupa los vencimientos de los próximos 12 meses
// por tipo de documento (SOAT, Tecnomecánica, Póliza de seguros).
func (r *DashboardRepository) VencimientosPorMesPorTipo(ctx context.Context, organizationID int) ([]VencimientoMensualPorTipo, error) {
	query := `
		SELECT
			TO_CHAR(d.fecha_vencimiento, 'YYYY-MM') AS mes,
			SUM(CASE WHEN td.nombre = 'SOAT' THEN 1 ELSE 0 END) AS soat,
			SUM(CASE WHEN td.nombre = 'Tecnomecánica' THEN 1 ELSE 0 END) AS tecno,
			SUM(CASE WHEN td.nombre = 'Póliza de seguros' THEN 1 ELSE 0 END) AS poliza
		FROM documentos d
		INNER JOIN tipos_documento td ON td.id = d.tipo_documento_id
		WHERE d.organization_id = $1 AND d.vigente_actual = TRUE
			AND d.fecha_vencimiento BETWEEN CURRENT_DATE AND (CURRENT_DATE + INTERVAL '12 months')
		GROUP BY TO_CHAR(d.fecha_vencimiento, 'YYYY-MM')
		ORDER BY TO_CHAR(d.fecha_vencimiento, 'YYYY-MM')
	`
	rows, err := r.db.QueryContext(ctx, query, organizationID)
	if err != nil {
		return nil, fmt.Errorf("error al calcular vencimientos por mes y tipo: %w", err)
	}
	defer rows.Close()

	var resultados []VencimientoMensualPorTipo
	for rows.Next() {
		var v VencimientoMensualPorTipo
		if err := rows.Scan(&v.Mes, &v.Soat, &v.Tecnomecanica, &v.Poliza); err != nil {
			return nil, err
		}
		resultados = append(resultados, v)
	}
	return resultados, nil
}

func (r *DashboardRepository) conteoGenerico(ctx context.Context, query string, organizationID int) ([]ConteoPorCampo, error) {
	rows, err := r.db.QueryContext(ctx, query, organizationID)
	if err != nil {
		return nil, fmt.Errorf("error al ejecutar conteo del dashboard: %w", err)
	}
	defer rows.Close()

	var resultados []ConteoPorCampo
	for rows.Next() {
		var c ConteoPorCampo
		if err := rows.Scan(&c.Etiqueta, &c.Total); err != nil {
			return nil, err
		}
		resultados = append(resultados, c)
	}
	return resultados, nil
}
