package service

import (
	"context"
	"errors"

	"github.com/alcaldia/sigpa-backend/internal/models"
	"github.com/alcaldia/sigpa-backend/internal/repository"
	"github.com/alcaldia/sigpa-backend/pkg/apperrors"
)

type ProgramacionService struct {
	repo *repository.ProgramacionRepository
}

func NewProgramacionService(repo *repository.ProgramacionRepository) *ProgramacionService {
	return &ProgramacionService{repo: repo}
}

func (s *ProgramacionService) Listar(ctx context.Context, organizationID int) ([]models.Programacion, error) {
	return s.repo.Listar(ctx, organizationID)
}

func (s *ProgramacionService) Obtener(ctx context.Context, organizationID, id int) (*models.Programacion, error) {
	return s.repo.Obtener(ctx, organizationID, id)
}

// ObtenerUltima expone la programación más reciente para "Copiar programación anterior".
func (s *ProgramacionService) ObtenerUltima(ctx context.Context, organizationID int) (*models.Programacion, error) {
	return s.repo.ObtenerUltima(ctx, organizationID)
}

func (s *ProgramacionService) Crear(ctx context.Context, p *models.Programacion) (int, error) {
	return s.repo.Crear(ctx, p)
}

func (s *ProgramacionService) Actualizar(ctx context.Context, organizationID, id int, p *models.Programacion) error {
	return s.repo.Actualizar(ctx, organizationID, id, p)
}

func (s *ProgramacionService) Eliminar(ctx context.Context, organizationID, id int) error {
	return s.repo.Eliminar(ctx, organizationID, id)
}

// SolicitarVehiculo implementa el flujo del formulario de solicitud:
//  1. Busca si ya existe una programación para la fecha solicitada.
//  2. Si no existe, la crea vacía (sin items).
//  3. Agrega el nuevo item a esa programación con conductor/vehículo sin
//     asignar y programado=false — queda pendiente de que el director lo
//     complete y confirme desde "Editar Programación".
//
// Devuelve el ID de la programación (para poder redirigir/consultar) y el
// ID del item recién creado.
func (s *ProgramacionService) SolicitarVehiculo(ctx context.Context, organizationID int, creadoPor int, item *models.ProgramacionItem, fecha string) (programacionID int, itemID int, err error) {
	prog, err := s.repo.BuscarPorFecha(ctx, organizationID, fecha)
	if err != nil {
		if !errors.Is(err, apperrors.ErrNotFound) {
			return 0, 0, err
		}
		// No existe programación para esta fecha todavía — se crea vacía.
		nuevaProg := &models.Programacion{
			OrganizationID: organizationID,
			Fecha:          fecha,
			CreadoPor:      &creadoPor,
			Items:          []models.ProgramacionItem{},
		}
		programacionID, err = s.repo.Crear(ctx, nuevaProg)
		if err != nil {
			return 0, 0, err
		}
	} else {
		programacionID = prog.ID
	}

	itemID, err = s.repo.AgregarItem(ctx, programacionID, item)
	if err != nil {
		return 0, 0, err
	}
	return programacionID, itemID, nil
}

// MisSolicitudes lista las solicitudes hechas por un correo específico —
// alimenta la pantalla "Mis solicitudes" del rol Solicitante.
func (s *ProgramacionService) MisSolicitudes(ctx context.Context, organizationID int, email string) ([]models.ProgramacionItem, error) {
	return s.repo.ListarSolicitudesPorEmail(ctx, organizationID, email)
}
