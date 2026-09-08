package models

import "time"

// ListaConfiguracion representa un item de una lista configurable
// (conductores, actividades, destinos, etc.).
type ListaConfiguracion struct {
	ID             int       `json:"id" db:"id"`
	OrganizationID int       `json:"organization_id" db:"organization_id"`
	Tipo           string    `json:"tipo" db:"tipo"` // 'conductor', 'actividad', etc.
	Nombre         string    `json:"nombre" db:"nombre"`
	Activo         bool      `json:"activo" db:"activo"`
	Orden          int       `json:"orden" db:"orden"`
	FechaCreacion  time.Time `json:"fecha_creacion" db:"fecha_creacion"`
}

// Programacion es la cabecera de una programación diaria de vehículos (15-FR-36).
type Programacion struct {
	ID                 int       `json:"id" db:"id"`
	OrganizationID     int       `json:"organization_id" db:"organization_id"`
	Fecha              string    `json:"fecha" db:"fecha"` // DATE → string "YYYY-MM-DD"
	Observaciones      *string   `json:"observaciones,omitempty" db:"observaciones"`
	CreadoPor          *int      `json:"creado_por,omitempty" db:"creado_por"`
	CreadoPorNombre    string    `json:"creado_por_nombre,omitempty" db:"creado_por_nombre"`
	FechaCreacion      time.Time `json:"fecha_creacion" db:"fecha_creacion"`
	FechaActualizacion time.Time `json:"fecha_actualizacion" db:"fecha_actualizacion"`
	// Items se rellena solo en el GET de detalle, no en el listado.
	Items []ProgramacionItem `json:"items,omitempty"`
}

// ProgramacionItem es una fila de la programación (un conductor asignado).
type ProgramacionItem struct {
	ID              int    `json:"id" db:"id"`
	ProgramacionID  int    `json:"programacion_id" db:"programacion_id"`
	VehiculoID      *int   `json:"vehiculo_id,omitempty" db:"vehiculo_id"`
	VehiculoPlaca   string `json:"vehiculo_placa,omitempty" db:"vehiculo_placa"`
	Conductor       string `json:"conductor" db:"conductor"`
	Dependencia     string `json:"dependencia" db:"dependencia"`
	Destino         string `json:"destino" db:"destino"`
	HoraSalidaPunto string `json:"hora_salida_punto" db:"hora_salida_punto"`
	Actividad       string `json:"actividad" db:"actividad"`
	EsVacaciones    bool   `json:"es_vacaciones" db:"es_vacaciones"`
	Orden           int    `json:"orden" db:"orden"`
}
