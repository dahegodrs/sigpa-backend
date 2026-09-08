package models

import "time"

// Vehiculo representa un registro del parque automotor.
type Vehiculo struct {
	ID                 int       `json:"id" db:"id"`
	OrganizationID     int       `json:"organization_id" db:"organization_id"`
	Placa              string    `json:"placa" db:"placa"`
	TipoVehiculoID     int       `json:"tipo_vehiculo_id" db:"tipo_vehiculo_id"`
	TipoVehiculoNombre string    `json:"tipo_vehiculo_nombre,omitempty" db:"tipo_vehiculo_nombre"`
	Marca              *string   `json:"marca,omitempty" db:"marca"`
	Linea              *string   `json:"linea,omitempty" db:"linea"`
	Modelo             *int      `json:"modelo,omitempty" db:"modelo"`
	Color              *string   `json:"color,omitempty" db:"color"`
	Motor              *string   `json:"motor,omitempty" db:"motor"`
	Chasis             *string   `json:"chasis,omitempty" db:"chasis"`
	VIN                *string   `json:"vin,omitempty" db:"vin"`
	Capacidad          *string   `json:"capacidad,omitempty" db:"capacidad"`
	Combustible        *string   `json:"combustible,omitempty" db:"combustible"`
	DependenciaID      *int      `json:"dependencia_id,omitempty" db:"dependencia_id"`
	DependenciaNombre  string    `json:"dependencia_nombre,omitempty" db:"dependencia_nombre"`
	ResponsableID      *int      `json:"responsable_id,omitempty" db:"responsable_id"`
	ResponsableNombre  string    `json:"responsable_nombre,omitempty" db:"responsable_nombre"`
	EstadoID           int       `json:"estado_id" db:"estado_id"`
	EstadoNombre       string    `json:"estado_nombre,omitempty" db:"estado_nombre"`
	Ubicacion          *string   `json:"ubicacion,omitempty" db:"ubicacion"`
	Observaciones      *string   `json:"observaciones,omitempty" db:"observaciones"`
	Activo             bool      `json:"activo" db:"activo"`
	FechaCreacion      time.Time `json:"fecha_creacion" db:"fecha_creacion"`
	FechaActualizacion time.Time `json:"fecha_actualizacion" db:"fecha_actualizacion"`

	// Campos calculados para la ficha del vehículo (no persistidos directamente aquí)
	SaludDocumental string `json:"salud_documental,omitempty" db:"-"`

	// Estado documental resumido de los 3 documentos principales, usado SOLO
	// en el listado (GET /vehiculos) para no obligar a entrar a cada ficha
	// a ver si algo está vencido. Nil si el vehículo no tiene ese documento cargado.
	SoatEstado   *string `json:"soat_estado,omitempty" db:"-"`
	TecnoEstado  *string `json:"tecno_estado,omitempty" db:"-"`
	PolizaEstado *string `json:"poliza_estado,omitempty" db:"-"`
}

// VehiculoFiltro agrupa los parámetros de búsqueda / filtros soportados por el listado.
type VehiculoFiltro struct {
	OrganizationID int
	Placa          string
	DependenciaID  *int
	TipoVehiculoID *int
	EstadoID       *int
	ResponsableID  *int
	Marca          string
	Modelo         *int
	Page           int
	PageSize       int
}

// EstadoVehiculo catálogo de estados posibles.
type EstadoVehiculo struct {
	ID     int    `json:"id" db:"id"`
	Nombre string `json:"nombre" db:"nombre"`
}

// TipoVehiculo catálogo de tipos de vehículo.
type TipoVehiculo struct {
	ID     int    `json:"id" db:"id"`
	Nombre string `json:"nombre" db:"nombre"`
}
