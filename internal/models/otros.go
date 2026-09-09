package models

import "time"

// Alerta representa una notificación programada o enviada.
type Alerta struct {
	ID              int        `json:"id" db:"id"`
	OrganizationID  int        `json:"organization_id" db:"organization_id"`
	VehiculoID      int        `json:"vehiculo_id" db:"vehiculo_id"`
	DocumentoID     int        `json:"documento_id" db:"documento_id"`
	TipoAlerta      string     `json:"tipo_alerta" db:"tipo_alerta"` // Preventiva, Importante, Prioritaria, Urgente, Critica
	Canal           string     `json:"canal" db:"canal"`             // Email, Google Chat, WhatsApp
	FechaProgramada time.Time  `json:"fecha_programada" db:"fecha_programada"`
	FechaEnvio      *time.Time `json:"fecha_envio,omitempty" db:"fecha_envio"`
	Destinatario    string     `json:"destinatario" db:"destinatario"`
	EstadoEnvio     string     `json:"estado_envio" db:"estado_envio"` // Pendiente, Enviada, Error
	DetalleError    *string    `json:"detalle_error,omitempty" db:"detalle_error"`
	Leida           bool       `json:"leida" db:"leida"`
	FechaCreacion   time.Time  `json:"fecha_creacion" db:"fecha_creacion"`

	// Campos enriquecidos vía JOIN (no se persisten en la tabla `alertas`,
	// solo se completan al listar) — permiten mostrar mensajes claros como
	// "SOAT vencido hace 5 días" en vez del genérico "Documento vencido",
	// sin que el frontend tenga que hacer una consulta adicional por cada
	// alerta para averiguar a qué documento/vehículo corresponde.
	VehiculoPlaca       string     `json:"vehiculo_placa,omitempty" db:"vehiculo_placa"`
	TipoDocumentoNombre string     `json:"tipo_documento_nombre,omitempty" db:"tipo_documento_nombre"`
	DocumentoFechaVenc  *time.Time `json:"documento_fecha_vencimiento,omitempty" db:"documento_fecha_vencimiento"`
}

// ConfigAlerta define los umbrales (días antes del vencimiento) por organización.
type ConfigAlerta struct {
	ID             int    `json:"id" db:"id"`
	OrganizationID int    `json:"organization_id" db:"organization_id"`
	DiasAntes      int    `json:"dias_antes" db:"dias_antes"`
	Nivel          string `json:"nivel" db:"nivel"`
	Activo         bool   `json:"activo" db:"activo"`
}

// HistorialCambio registra la trazabilidad de cualquier entidad del sistema.
type HistorialCambio struct {
	ID              int64     `json:"id" db:"id"`
	OrganizationID  int       `json:"organization_id" db:"organization_id"`
	Entidad         string    `json:"entidad" db:"entidad"`
	EntidadID       int       `json:"entidad_id" db:"entidad_id"`
	UsuarioID       *int      `json:"usuario_id,omitempty" db:"usuario_id"`
	UsuarioNombre   string    `json:"usuario_nombre,omitempty" db:"usuario_nombre"`
	Accion          string    `json:"accion" db:"accion"` // CREACION, ACTUALIZACION, ELIMINACION, CAMBIO_ESTADO
	CampoModificado *string   `json:"campo_modificado,omitempty" db:"campo_modificado"`
	ValorAnterior   *string   `json:"valor_anterior,omitempty" db:"valor_anterior"`
	ValorNuevo      *string   `json:"valor_nuevo,omitempty" db:"valor_nuevo"`
	Motivo          *string   `json:"motivo,omitempty" db:"motivo"`
	Fecha           time.Time `json:"fecha" db:"fecha"`
	// Referencia es un dato legible (ej: placa del vehículo) resuelto según
	// el tipo de entidad, usado por el feed global de "Actividad reciente".
	Referencia *string `json:"referencia,omitempty" db:"referencia"`
}

// Rol catálogo de roles del sistema.
type Rol struct {
	ID          int     `json:"id" db:"id"`
	Nombre      string  `json:"nombre" db:"nombre"`
	Descripcion *string `json:"descripcion,omitempty" db:"descripcion"`
}

// Usuario representa una cuenta autenticada vía Google Workspace o credenciales locales.
type Usuario struct {
	ID             int        `json:"id" db:"id"`
	OrganizationID int        `json:"organization_id" db:"organization_id"`
	GoogleID       string     `json:"google_id" db:"google_id"`
	Email          string     `json:"email" db:"email"`
	Nombre         string     `json:"nombre" db:"nombre"`
	RolID          int        `json:"rol_id" db:"rol_id"`
	RolNombre      string     `json:"rol_nombre,omitempty" db:"rol_nombre"`
	DependenciaID  *int       `json:"dependencia_id,omitempty" db:"dependencia_id"`
	Activo         bool       `json:"activo" db:"activo"`
	UltimoLogin    *time.Time `json:"ultimo_login,omitempty" db:"ultimo_login"`
	FechaCreacion  time.Time  `json:"fecha_creacion" db:"fecha_creacion"`
	// PasswordHash nunca se serializa al cliente — solo se usa internamente
	// para verificar credenciales locales.
	PasswordHash *string `json:"-" db:"password_hash"`
}

// Organizacion representa cada Alcaldía (tenant).
type Organizacion struct {
	ID            int    `json:"id" db:"id"`
	Nombre        string `json:"nombre" db:"nombre"`
	DominioGoogle string `json:"dominio_google" db:"dominio_google"`
	Activo        bool   `json:"activo" db:"activo"`
}

// OrganizacionTema define el branding parametrizable de cada Alcaldía
// (logo, colores, tipografía), para que el frontend pueda tematizarse
// dinámicamente en vez de tener los colores hardcodeados.
type OrganizacionTema struct {
	ID                   int     `json:"id" db:"id"`
	OrganizationID       int     `json:"organization_id" db:"organization_id"`
	LogoURL              *string `json:"logo_url,omitempty" db:"logo_url"`
	EscudoURL            *string `json:"escudo_url,omitempty" db:"escudo_url"`
	ColorPrimario        string  `json:"color_primario" db:"color_primario"`
	ColorSecundario      string  `json:"color_secundario" db:"color_secundario"`
	ColorFondoSidebar    string  `json:"color_fondo_sidebar" db:"color_fondo_sidebar"`
	ColorHoverSidebar    string  `json:"color_hover_sidebar" db:"color_hover_sidebar"`
	ColorFondoTopbar     string  `json:"color_fondo_topbar" db:"color_fondo_topbar"`
	ColorBotonPrimario   string  `json:"color_boton_primario" db:"color_boton_primario"`
	ColorBotonSecundario string  `json:"color_boton_secundario" db:"color_boton_secundario"`
	ColorTexto           string  `json:"color_texto" db:"color_texto"`
	Tipografia           string  `json:"tipografia" db:"tipografia"`
	PaletaExtendidaJSON  *string `json:"paleta_extendida_json,omitempty" db:"paleta_extendida_json"`
}

// Dependencia representa una secretaría/dependencia dentro de la Alcaldía.
type Dependencia struct {
	ID                int     `json:"id" db:"id"`
	OrganizationID    int     `json:"organization_id" db:"organization_id"`
	Nombre            string  `json:"nombre" db:"nombre"`
	Descripcion       *string `json:"descripcion,omitempty" db:"descripcion"`
	ResponsableID     *int    `json:"responsable_id,omitempty" db:"responsable_id"`
	ResponsableNombre string  `json:"responsable_nombre,omitempty" db:"responsable_nombre"`
	Activo            bool    `json:"activo" db:"activo"`
}
