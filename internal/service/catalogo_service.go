package service

import (
	"context"

	"github.com/alcaldia/sigpa-backend/internal/models"
	"github.com/alcaldia/sigpa-backend/internal/repository"
)

type CatalogoService struct {
	repo *repository.CatalogoRepository
}

func NewCatalogoService(repo *repository.CatalogoRepository) *CatalogoService {
	return &CatalogoService{repo: repo}
}

// TodosLosCatalogos agrupa lo que el frontend necesita para poblar selects de
// formularios y filtros, en una sola llamada al cargar la aplicación.
type TodosLosCatalogos struct {
	Roles           []models.Rol            `json:"roles"`
	EstadosVehiculo []models.EstadoVehiculo `json:"estados_vehiculo"`
	TiposVehiculo   []models.TipoVehiculo   `json:"tipos_vehiculo"`
	TiposDocumento  []models.TipoDocumento  `json:"tipos_documento"`
}

func (s *CatalogoService) ObtenerTodos(ctx context.Context) (*TodosLosCatalogos, error) {
	roles, err := s.repo.ListRoles(ctx)
	if err != nil {
		return nil, err
	}
	estados, err := s.repo.ListEstadosVehiculo(ctx)
	if err != nil {
		return nil, err
	}
	tiposVehiculo, err := s.repo.ListTiposVehiculo(ctx)
	if err != nil {
		return nil, err
	}
	tiposDocumento, err := s.repo.ListTiposDocumento(ctx)
	if err != nil {
		return nil, err
	}

	return &TodosLosCatalogos{
		Roles:           roles,
		EstadosVehiculo: estados,
		TiposVehiculo:   tiposVehiculo,
		TiposDocumento:  tiposDocumento,
	}, nil
}

// ── Gestión de tipos de vehículo desde Administración → Listas ─────────────

func (s *CatalogoService) ListarTodosTiposVehiculo(ctx context.Context) ([]models.TipoVehiculo, error) {
	return s.repo.ListTodosTiposVehiculo(ctx)
}

func (s *CatalogoService) CrearTipoVehiculo(ctx context.Context, nombre string) (int, error) {
	return s.repo.CrearTipoVehiculo(ctx, nombre)
}

func (s *CatalogoService) ActualizarTipoVehiculo(ctx context.Context, id int, nombre string, activo bool) error {
	return s.repo.ActualizarTipoVehiculo(ctx, id, nombre, activo)
}
