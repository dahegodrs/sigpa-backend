package service

import (
	"context"

	"github.com/alcaldia/sigpa-backend/internal/models"
	"github.com/alcaldia/sigpa-backend/internal/repository"
)

type TemaService struct {
	orgRepo *repository.OrganizacionRepository
}

func NewTemaService(orgRepo *repository.OrganizacionRepository) *TemaService {
	return &TemaService{orgRepo: orgRepo}
}

// ObtenerPorDominio permite tematizar la pantalla de login antes de
// autenticar, a partir del dominio institucional visible en la URL o
// ingresado por el usuario (ej: alcaldiadefunza.gov.co).
func (s *TemaService) ObtenerPorDominio(ctx context.Context, dominio string) (*models.OrganizacionTema, error) {
	return s.orgRepo.GetTemaPorDominio(ctx, dominio)
}

// ObtenerPorOrganizacion se usa después del login, con el organization_id
// resuelto desde el JWT.
func (s *TemaService) ObtenerPorOrganizacion(ctx context.Context, organizationID int) (*models.OrganizacionTema, error) {
	return s.orgRepo.GetTemaPorOrganizationID(ctx, organizationID)
}
