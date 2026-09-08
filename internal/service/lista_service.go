package service

import (
	"context"

	"github.com/alcaldia/sigpa-backend/internal/models"
	"github.com/alcaldia/sigpa-backend/internal/repository"
)

type ListaService struct {
	repo *repository.ListaRepository
}

func NewListaService(repo *repository.ListaRepository) *ListaService {
	return &ListaService{repo: repo}
}

func (s *ListaService) ListarPorTipo(ctx context.Context, organizationID int, tipo string) ([]models.ListaConfiguracion, error) {
	return s.repo.ListarPorTipo(ctx, organizationID, tipo)
}

func (s *ListaService) ListarTodos(ctx context.Context, organizationID int) ([]models.ListaConfiguracion, error) {
	return s.repo.ListarTodos(ctx, organizationID)
}

func (s *ListaService) Crear(ctx context.Context, organizationID int, tipo, nombre string, orden int) (int, error) {
	return s.repo.Crear(ctx, &models.ListaConfiguracion{
		OrganizationID: organizationID,
		Tipo:           tipo,
		Nombre:         nombre,
		Orden:          orden,
	})
}

func (s *ListaService) Actualizar(ctx context.Context, organizationID, id int, nombre string, orden int, activo bool) error {
	return s.repo.Actualizar(ctx, organizationID, id, nombre, orden, activo)
}

func (s *ListaService) Eliminar(ctx context.Context, organizationID, id int) error {
	return s.repo.Eliminar(ctx, organizationID, id)
}
