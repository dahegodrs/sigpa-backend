package service

import (
	"context"

	"github.com/alcaldia/sigpa-backend/internal/models"
	"github.com/alcaldia/sigpa-backend/internal/repository"
)

type DependenciaService struct {
	repo          *repository.DependenciaRepository
	historialRepo *repository.HistorialRepository
}

func NewDependenciaService(repo *repository.DependenciaRepository, historialRepo *repository.HistorialRepository) *DependenciaService {
	return &DependenciaService{repo: repo, historialRepo: historialRepo}
}

func (s *DependenciaService) List(ctx context.Context, organizationID int) ([]models.Dependencia, error) {
	return s.repo.List(ctx, organizationID)
}

func (s *DependenciaService) Create(ctx context.Context, d *models.Dependencia, usuarioID int) (int, error) {
	newID, err := s.repo.Create(ctx, d)
	if err != nil {
		return 0, err
	}
	_ = s.historialRepo.Registrar(ctx, &models.HistorialCambio{
		OrganizationID: d.OrganizationID,
		Entidad:        "dependencia",
		EntidadID:      newID,
		UsuarioID:      &usuarioID,
		Accion:         "CREACION",
	})
	return newID, nil
}

func (s *DependenciaService) Update(ctx context.Context, d *models.Dependencia, usuarioID int) error {
	if err := s.repo.Update(ctx, d); err != nil {
		return err
	}
	_ = s.historialRepo.Registrar(ctx, &models.HistorialCambio{
		OrganizationID: d.OrganizationID,
		Entidad:        "dependencia",
		EntidadID:      d.ID,
		UsuarioID:      &usuarioID,
		Accion:         "ACTUALIZACION",
	})
	return nil
}

func (s *DependenciaService) Delete(ctx context.Context, organizationID, id, usuarioID int) error {
	if err := s.repo.SoftDelete(ctx, organizationID, id); err != nil {
		return err
	}
	_ = s.historialRepo.Registrar(ctx, &models.HistorialCambio{
		OrganizationID: organizationID,
		Entidad:        "dependencia",
		EntidadID:      id,
		UsuarioID:      &usuarioID,
		Accion:         "ELIMINACION",
	})
	return nil
}
