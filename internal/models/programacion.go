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
	// TienePendientes indica si el día tiene al menos una fila de
	// solicitud (origen='solicitud') sin resolver todavía — el frontend
	// lo usa para resaltar en rojo los días que requieren atención del
	// Administrador en la vista de calendario/listado.
	TienePendientes bool `json:"tiene_pendientes" db:"tiene_pendientes"`
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

	// ── Estado explícito de la solicitud (timeline tipo pasarela) ──────────
	// EstadoSolicitud es independiente de Programado: permite que el
	// director rechace una solicitud sin necesidad de asignarle vehículo
	// primero, y le da al solicitante un timeline claro en "Mis solicitudes"
	// (Solicitado → En espera de aprobación → Aprobada/Rechazada).
	EstadoSolicitud string  `json:"estado_solicitud" db:"estado_solicitud"`
	MotivoRechazo   *string `json:"motivo_rechazo,omitempty" db:"motivo_rechazo"`
	// NotificadoEn marca si ya se le envió correo al solicitante sobre esta
	// fila — evita reenviar la misma decisión si el director guarda de
	// nuevo, y permite agrupar en un solo correo todas las filas de un
	// mismo solicitante que aún no se le han notificado.
	NotificadoEn *time.Time `json:"notificado_en,omitempty" db:"notificado_en"`
}

// PlantillaCorreo es una plantilla de correo configurable por organización
// (ej. la notificación de aprobación/rechazo de solicitud de vehículo),
// guardada en base de datos para que la vean/editen todos los
// Administradores en vez de quedar en el localStorage de un solo navegador.
type PlantillaCorreo struct {
	ID                 int       `json:"id" db:"id"`
	OrganizationID     int       `json:"organization_id" db:"organization_id"`
	Tipo               string    `json:"tipo" db:"tipo"`
	Asunto             string    `json:"asunto" db:"asunto"`
	CuerpoHTML         string    `json:"cuerpo_html" db:"cuerpo_html"`
	FechaActualizacion time.Time `json:"fecha_actualizacion" db:"fecha_actualizacion"`
}
