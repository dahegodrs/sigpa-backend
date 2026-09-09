package service

import (
	"context"

	"github.com/alcaldia/sigpa-backend/internal/models"
	"github.com/alcaldia/sigpa-backend/internal/repository"
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
