package models

import "time"

// EstadoDocumento representa los estados calculados de un documento.
type EstadoDocumento string

const (
	EstadoVigente        EstadoDocumento = "Vigente"
	EstadoProximoAVencer EstadoDocumento = "Proximo_a_vencer"
	EstadoVencido        EstadoDocumento = "Vencido"
	EstadoPendiente      EstadoDocumento = "Pendiente"
)

// Documento representa un documento asociado a un vehículo (SOAT, Tecnomecánica, etc.)
type Documento struct {
	ID                  int             `json:"id" db:"id"`
	OrganizationID      int             `json:"organization_id" db:"organization_id"`
	VehiculoID          int             `json:"vehiculo_id" db:"vehiculo_id"`
	VehiculoPlaca       string          `json:"vehiculo_placa,omitempty" db:"vehiculo_placa"`
	TipoDocumentoID     int             `json:"tipo_documento_id" db:"tipo_documento_id"`
	TipoDocumentoNombre string          `json:"tipo_documento_nombre,omitempty" db:"tipo_documento_nombre"`
	FechaExpedicion     *time.Time      `json:"fecha_expedicion,omitempty" db:"fecha_expedicion"`
	FechaVencimiento    *time.Time      `json:"fecha_vencimiento,omitempty" db:"fecha_vencimiento"`
	EstadoDocumento     EstadoDocumento `json:"estado_documento" db:"estado_documento"`
	ArchivoURL          *string         `json:"archivo_url,omitempty" db:"archivo_url"`
	ArchivoDriveID      *string         `json:"archivo_drive_id,omitempty" db:"archivo_drive_id"`
	NombreArchivo       *string         `json:"nombre_archivo,omitempty" db:"nombre_archivo"`
	TamanoBytes         *int64          `json:"tamano_bytes,omitempty" db:"tamano_bytes"`
	Version             int             `json:"version" db:"version"`
	VigenteActual       bool            `json:"vigente_actual" db:"vigente_actual"`
	Observaciones       *string         `json:"observaciones,omitempty" db:"observaciones"`
	FechaCarga          time.Time       `json:"fecha_carga" db:"fecha_carga"`
	CargadoPor          *int            `json:"cargado_por,omitempty" db:"cargado_por"`
	Eliminado           bool            `json:"eliminado" db:"eliminado"`
	EliminadoPor        *int            `json:"eliminado_por,omitempty" db:"eliminado_por"`
	FechaEliminacion    *time.Time      `json:"fecha_eliminacion,omitempty" db:"fecha_eliminacion"`
}

// TipoDocumento catálogo de tipos de documento controlados.
type TipoDocumento struct {
	ID                int    `json:"id" db:"id"`
	Nombre            string `json:"nombre" db:"nombre"`
	Obligatorio       bool   `json:"obligatorio" db:"obligatorio"`
	DiasAlertaDefault int    `json:"dias_alerta_default" db:"dias_alerta_default"`
}

// DiasHasta calcula la diferencia en días de calendario entre "ahora" y una
// fecha objetivo, ignorando la hora del día. Ambas fechas se normalizan a UTC
// antes de comparar: las fechas de vencimiento se guardan como medianoche UTC,
// pero "ahora" llega en la zona horaria local del servidor (ej: Colombia,
// UTC-5); si se comparan sin normalizar, el resultado varía varias horas y
// puede redondear al día equivocado (ej: dar 6 días en vez de 7).
func DiasHasta(fecha time.Time, ahora time.Time) int {
	fechaUTC := fecha.UTC()
	ahoraUTC := ahora.UTC()
	fechaSoloDia := time.Date(fechaUTC.Year(), fechaUTC.Month(), fechaUTC.Day(), 0, 0, 0, 0, time.UTC)
	ahoraSoloDia := time.Date(ahoraUTC.Year(), ahoraUTC.Month(), ahoraUTC.Day(), 0, 0, 0, 0, time.UTC)
	return int(fechaSoloDia.Sub(ahoraSoloDia).Hours() / 24)
}

// CalcularEstado determina el estado documental según la fecha de vencimiento
// y la ventana de "vigente" configurada (por defecto 45 días, según las reglas
// del proyecto). Regla: Vigente > diasVentana días para vencer, Proximo_a_vencer
// dentro de la ventana, Vencido si la fecha ya pasó, Pendiente si no hay fecha.
func CalcularEstado(fechaVencimiento *time.Time, diasVentana int, ahora time.Time) EstadoDocumento {
	if fechaVencimiento == nil {
		return EstadoPendiente
	}
	diasRestantes := DiasHasta(*fechaVencimiento, ahora)

	switch {
	case diasRestantes < 0:
		return EstadoVencido
	case diasRestantes <= diasVentana:
		return EstadoProximoAVencer
	default:
		return EstadoVigente
	}
}
