package apperrors

import "errors"

var (
	ErrNotFound         = errors.New("recurso no encontrado")
	ErrDuplicatePlaca   = errors.New("ya existe un vehículo con esa placa en la organización")
	ErrDuplicado        = errors.New("el recurso ya existe")
	ErrValidation       = errors.New("error de validación")
	ErrUnauthorized     = errors.New("no autorizado")
	ErrForbidden        = errors.New("acceso denegado para el rol actual")
	ErrInvalidTenant    = errors.New("organización no válida o inactiva")
)
