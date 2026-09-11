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
	// TotalItems/TotalProgramados se calculan en el listado (GET /programaciones)
	// para mostrar el conteo en la tabla sin tener que traer todos los items.
	// TotalProgramados es el que se muestra al usuario, ya que representa
	// cuántos vehículos quedaron confirmados para la planilla oficial —
	// las filas sin marcar (borradores) no cuentan aunque existan en la BD.
	TotalItems       int `json:"total_items" db:"total_items"`
	TotalProgramados int `json:"total_programados" db:"total_programados"`
	// Items se rellena solo en el GET de detalle, no en el listado.
	Items []ProgramacionItem `json:"items,omitempty"`
}

// ProgramacionItem es una fila de la programación (un conductor asignado).
type ProgramacionItem struct {
	ID               int    `json:"id" db:"id"`
	ProgramacionID   int    `json:"programacion_id" db:"programacion_id"`
	VehiculoID       *int   `json:"vehiculo_id,omitempty" db:"vehiculo_id"`
	VehiculoPlaca    string `json:"vehiculo_placa,omitempty" db:"vehiculo_placa"`
	Conductor        string `json:"conductor" db:"conductor"`
	Dependencia      string `json:"dependencia" db:"dependencia"`
	Destino          string `json:"destino" db:"destino"`
	HoraSalidaPunto  string `json:"hora_salida_punto" db:"hora_salida_punto"`
	HoraFinalizacion string `json:"hora_finalizacion" db:"hora_finalizacion"`
	Actividad        string `json:"actividad" db:"actividad"`
	EsVacaciones     bool   `json:"es_vacaciones" db:"es_vacaciones"`
	// Programado controla si la fila se incluye en el PDF/planilla final:
	// solo las filas con Programado = true aparecen en la vista de
	// impresión, permitiendo dejar borradores/filas en construcción sin
	// que salgan en el documento oficial que se genera al guardar.
	Programado bool `json:"programado" db:"programado"`
	Orden      int  `json:"orden" db:"orden"`

	// ── Campos de solicitud (formulario de solicitud de vehículo) ──────────
	// Motivo es texto libre explicando el porqué del viaje — distinto de
	// Actividad, que es una categoría cerrada (TRASLADO FUNCIONARIOS, etc.).
	Motivo *string `json:"motivo,omitempty" db:"motivo"`
	// Origen distingue si la fila vino de una solicitud por formulario
	// ("solicitud") o fue creada directamente por el director ("manual").
	Origen            string  `json:"origen" db:"origen"`
	SolicitanteNombre *string `json:"solicitante_nombre,omitempty" db:"solicitante_nombre"`
	SolicitanteEmail  *string `json:"solicitante_email,omitempty" db:"solicitante_email"`
	HoraSolicitada    *string `json:"hora_solicitada,omitempty" db:"hora_solicitada"` // "HH:MM" — tipo TIME en BD
	PuntoEncuentro    *string `json:"punto_encuentro,omitempty" db:"punto_encuentro"`
	// FechaProgramacion solo se rellena en ListarSolicitudesPorEmail (join
	// con la cabecera), para que "Mis solicitudes" pueda mostrar en qué
	// fecha quedó registrado el servicio sin tener que hacer otra consulta.
	FechaProgramacion string `json:"fecha_programacion,omitempty" db:"-"`
}
