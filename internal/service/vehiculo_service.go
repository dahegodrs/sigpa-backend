package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/alcaldia/sigpa-backend/internal/models"
	"github.com/alcaldia/sigpa-backend/internal/repository"
	"github.com/alcaldia/sigpa-backend/pkg/apperrors"
)

// placaRegex valida el formato colombiano típico: 3 letras + 3 números (autos)
// o 3 letras + 2 números + 1 letra (motos). Se deja flexible para maquinaria.
var placaRegex = regexp.MustCompile(`^[A-Z]{3}[0-9]{2,3}[A-Z]?$`)

type VehiculoService struct {
	repo           *repository.VehiculoRepository
	historialRepo  *repository.HistorialRepository
}

func NewVehiculoService(repo *repository.VehiculoRepository, historialRepo *repository.HistorialRepository) *VehiculoService {
	return &VehiculoService{repo: repo, historialRepo: historialRepo}
}

func (s *VehiculoService) Get(ctx context.Context, organizationID, id int) (*models.Vehiculo, error) {
	return s.repo.GetByID(ctx, organizationID, id)
}

func (s *VehiculoService) List(ctx context.Context, f models.VehiculoFiltro) ([]models.Vehiculo, int64, error) {
	return s.repo.List(ctx, f)
}

func (s *VehiculoService) Create(ctx context.Context, v *models.Vehiculo, usuarioID int) (int, error) {
	v.Placa = strings.ToUpper(strings.TrimSpace(v.Placa))

	if !placaRegex.MatchString(v.Placa) {
		return 0, fmt.Errorf("%w: formato de placa inválido (%s)", apperrors.ErrValidation, v.Placa)
	}

	existe, err := s.repo.ExistePlaca(ctx, v.OrganizationID, v.Placa, nil)
	if err != nil {
		return 0, err
	}
	if existe {
		return 0, apperrors.ErrDuplicatePlaca
	}

	newID, err := s.repo.Create(ctx, v)
	if err != nil {
		return 0, err
	}

	_ = s.historialRepo.Registrar(ctx, &models.HistorialCambio{
		OrganizationID: v.OrganizationID,
		Entidad:        "vehiculo",
		EntidadID:      newID,
		UsuarioID:      &usuarioID,
		Accion:         "CREACION",
	})

	return newID, nil
}

func (s *VehiculoService) Update(ctx context.Context, v *models.Vehiculo, usuarioID int, motivo string) error {
	anterior, err := s.repo.GetByID(ctx, v.OrganizationID, v.ID)
	if err != nil {
		return err
	}

	// El PUT es un "parche": el cliente puede mandar solo los campos que
	// quiere cambiar. Cualquier campo opcional que no venga en el request
	// (queda nil / cero) conserva el valor anterior, para no borrar datos
	// del vehículo sin querer (ej: cambiar solo el estado no debe borrar la
	// dependencia, el responsable, la marca, etc.).
	mergeConValoresAnteriores(v, anterior)

	if err := s.repo.Update(ctx, v); err != nil {
		return err
	}

	// Registrar en historial cada campo relevante que haya cambiado de estado,
	// que es el caso más importante de auditar (movimientos del vehículo).
	// Se vuelve a consultar el vehículo ya actualizado porque el request solo
	// trae estado_id (no el nombre), y el nombre del nuevo estado sale del
	// catálogo vía el JOIN de GetByID.
	if anterior.EstadoID != v.EstadoID {
		actualizado, errLookup := s.repo.GetByID(ctx, v.OrganizationID, v.ID)
		nuevoNombre := ""
		if errLookup == nil {
			nuevoNombre = actualizado.EstadoNombre
		}
		_ = s.historialRepo.Registrar(ctx, &models.HistorialCambio{
			OrganizationID:  v.OrganizationID,
			Entidad:         "vehiculo",
			EntidadID:       v.ID,
			UsuarioID:       &usuarioID,
			Accion:          "CAMBIO_ESTADO",
			CampoModificado: strPtr("estado"),
			ValorAnterior:   strPtr(anterior.EstadoNombre),
			ValorNuevo:      strPtr(nuevoNombre),
			Motivo:          strPtr(motivo),
		})
	} else {
		_ = s.historialRepo.Registrar(ctx, &models.HistorialCambio{
			OrganizationID: v.OrganizationID,
			Entidad:        "vehiculo",
			EntidadID:      v.ID,
			UsuarioID:      &usuarioID,
			Accion:         "ACTUALIZACION",
			Motivo:         strPtr(motivo),
		})
	}

	return nil
}

func (s *VehiculoService) Delete(ctx context.Context, organizationID, id, usuarioID int) error {
	if err := s.repo.SoftDelete(ctx, organizationID, id); err != nil {
		return err
	}
	_ = s.historialRepo.Registrar(ctx, &models.HistorialCambio{
		OrganizationID: organizationID,
		Entidad:        "vehiculo",
		EntidadID:      id,
		UsuarioID:      &usuarioID,
		Accion:         "ELIMINACION",
	})
	return nil
}

func (s *VehiculoService) Historial(ctx context.Context, organizationID, id int) ([]models.HistorialCambio, error) {
	return s.historialRepo.ListarPorEntidad(ctx, organizationID, "vehiculo", id)
}

func strPtr(s string) *string { return &s }

// mergeConValoresAnteriores completa con los valores previos cualquier campo
// opcional que el cliente no haya enviado en el PUT, para que una actualización
// parcial (ej: solo el estado) no borre el resto de la información del vehículo.
func mergeConValoresAnteriores(nuevo, anterior *models.Vehiculo) {
	if nuevo.Marca == nil {
		nuevo.Marca = anterior.Marca
	}
	if nuevo.Linea == nil {
		nuevo.Linea = anterior.Linea
	}
	if nuevo.Modelo == nil {
		nuevo.Modelo = anterior.Modelo
	}
	if nuevo.Color == nil {
		nuevo.Color = anterior.Color
	}
	if nuevo.Motor == nil {
		nuevo.Motor = anterior.Motor
	}
	if nuevo.Chasis == nil {
		nuevo.Chasis = anterior.Chasis
	}
	if nuevo.VIN == nil {
		nuevo.VIN = anterior.VIN
	}
	if nuevo.Capacidad == nil {
		nuevo.Capacidad = anterior.Capacidad
	}
	if nuevo.Combustible == nil {
		nuevo.Combustible = anterior.Combustible
	}
	if nuevo.DependenciaID == nil {
		nuevo.DependenciaID = anterior.DependenciaID
	}
	if nuevo.ResponsableID == nil {
		nuevo.ResponsableID = anterior.ResponsableID
	}
	if nuevo.Ubicacion == nil {
		nuevo.Ubicacion = anterior.Ubicacion
	}
	if nuevo.Observaciones == nil {
		nuevo.Observaciones = anterior.Observaciones
	}
	// EstadoID y TipoVehiculoID son obligatorios (NOT NULL) en la base de
	// datos, así que no son punteros; 0 nunca es un id real (empiezan en 1),
	// así que se usa como señal de "no vino en el request".
	if nuevo.EstadoID == 0 {
		nuevo.EstadoID = anterior.EstadoID
	}
	if nuevo.TipoVehiculoID == 0 {
		nuevo.TipoVehiculoID = anterior.TipoVehiculoID
	}
}
