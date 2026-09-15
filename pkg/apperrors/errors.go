package apperrors

import "errors"

var (
	ErrNotFound       = errors.New("recurso no encontrado")
	ErrDuplicatePlaca = errors.New("ya existe un vehículo con esa placa en la organización")
	ErrDuplicado      = errors.New("el recurso ya existe")
	ErrValidation     = errors.New("error de validación")
	ErrUnauthorized   = errors.New("no autorizado")
	ErrForbidden      = errors.New("acceso denegado para el rol actual")
	ErrInvalidTenant  = errors.New("organización no válida o inactiva")
	// ErrConflicto genérico se usa como sentinel para errors.Is() — el
	// mensaje real y descriptivo va en ConflictoError.Message.
	ErrConflicto = errors.New("conflicto de horario")
)

// ConflictoError envuelve ErrConflicto con un mensaje descriptivo dinámico
// (ej. "El vehículo BXL94C ya está programado de 08:00 a 10:00..."),
// permitiendo que el handler devuelva HTTP 409 con detalle específico
// mientras el resto del código puede seguir usando errors.Is(err,
// apperrors.ErrConflicto) para detectar el tipo de error.
type ConflictoError struct {
	Message string
}

func (e *ConflictoError) Error() string {
	return e.Message
}

func (e *ConflictoError) Is(target error) bool {
	return target == ErrConflicto
}

func NewConflictoError(mensaje string) error {
	return &ConflictoError{Message: mensaje}
}
