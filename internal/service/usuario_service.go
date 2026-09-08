package service

import (
	"context"
	"errors"
	"strings"

	"github.com/alcaldia/sigpa-backend/internal/models"
	"github.com/alcaldia/sigpa-backend/internal/repository"
	"github.com/alcaldia/sigpa-backend/pkg/apperrors"
)

// rolConsultaPorDefecto es el rol asignado automáticamente a cualquier cuenta
// del dominio institucional que inicie sesión por primera vez. Un
// Administrador debe elevar el rol manualmente después (Dependencia/Gerencia).
const rolConsultaPorDefecto = "Consulta"

type UsuarioService struct {
	repo           *repository.UsuarioRepository
	catalogoRepo   *repository.CatalogoRepository
	historialRepo  *repository.HistorialRepository
}

func NewUsuarioService(repo *repository.UsuarioRepository, catalogoRepo *repository.CatalogoRepository, historialRepo *repository.HistorialRepository) *UsuarioService {
	return &UsuarioService{repo: repo, catalogoRepo: catalogoRepo, historialRepo: historialRepo}
}

// ResolverOCrearPorGoogle busca el usuario por google_id; si no existe pero ya
// tiene un registro por email (caso migración), lo vincula; si no existe en
// absoluto, lo crea con el rol "Consulta" por defecto. El llamador (auth
// handler) ya validó que el dominio del correo corresponde a la organización.
func (s *UsuarioService) ResolverOCrearPorGoogle(ctx context.Context, organizationID int, googleID, email, nombre string) (*models.Usuario, error) {
	u, err := s.repo.GetByGoogleID(ctx, googleID)
	if err == nil {
		_ = s.repo.RegistrarLogin(ctx, u.ID)
		return u, nil
	}
	if !errors.Is(err, apperrors.ErrNotFound) {
		return nil, err
	}

	// ¿Existe un usuario pre-registrado (invitado por un Administrador) con
	// este correo, esperando su primer login real? Si es así, se activa
	// conservando el rol/dependencia ya asignados, en vez de crear uno nuevo
	// con el rol "Consulta" por defecto.
	preregistrado, errPre := s.repo.GetByEmail(ctx, email)
	if errPre == nil && strings.HasPrefix(preregistrado.GoogleID, "pending:") {
		if err := s.repo.ActualizarGoogleID(ctx, preregistrado.ID, googleID); err != nil {
			return nil, err
		}
		_ = s.repo.RegistrarLogin(ctx, preregistrado.ID)
		return s.repo.GetByID(ctx, organizationID, preregistrado.ID)
	}

	roles, err := s.catalogoRepo.ListRoles(ctx)
	if err != nil {
		return nil, err
	}
	var rolID int
	for _, r := range roles {
		if r.Nombre == rolConsultaPorDefecto {
			rolID = r.ID
			break
		}
	}

	nuevo := &models.Usuario{
		OrganizationID: organizationID,
		GoogleID:       googleID,
		Email:          email,
		Nombre:         nombre,
		RolID:          rolID,
	}
	newID, err := s.repo.Create(ctx, nuevo)
	if err != nil {
		return nil, err
	}

	_ = s.historialRepo.Registrar(ctx, &models.HistorialCambio{
		OrganizationID: organizationID,
		Entidad:        "usuario",
		EntidadID:      newID,
		Accion:         "CREACION",
		Motivo:         strPtr("primer inicio de sesión con Google"),
	})

	return s.repo.GetByID(ctx, organizationID, newID)
}

// InvitarUsuario permite que un Administrador pre-registre a alguien con un
// rol y dependencia ya definidos, antes de que esa persona inicie sesión.
func (s *UsuarioService) InvitarUsuario(ctx context.Context, organizationID int, email, nombre string, rolID int, dependenciaID *int, invitadoPor int) (int, error) {
	existente, err := s.repo.GetByEmail(ctx, email)
	if err == nil && existente != nil {
		return 0, apperrors.ErrDuplicado
	}
	if err != nil && !errors.Is(err, apperrors.ErrNotFound) {
		return 0, err
	}

	nuevo := &models.Usuario{
		OrganizationID: organizationID,
		Email:          email,
		Nombre:         nombre,
		RolID:          rolID,
		DependenciaID:  dependenciaID,
	}
	newID, err := s.repo.CrearPreregistrado(ctx, nuevo)
	if err != nil {
		return 0, err
	}

	_ = s.historialRepo.Registrar(ctx, &models.HistorialCambio{
		OrganizationID: organizationID,
		Entidad:        "usuario",
		EntidadID:      newID,
		UsuarioID:      &invitadoPor,
		Accion:         "CREACION",
		Motivo:         strPtr("invitación creada por administrador"),
	})

	return newID, nil
}

// BuscarPorEmail se usa desde el login para saber si un correo ya fue
// invitado o ya existe como usuario, sin importar el dominio institucional
// (permite el acceso de cuentas personales de Google previamente invitadas
// por un Administrador, mientras el dominio corporativo aún no esté listo).
func (s *UsuarioService) BuscarPorEmail(ctx context.Context, email string) (*models.Usuario, error) {
	return s.repo.GetByEmail(ctx, email)
}

func (s *UsuarioService) List(ctx context.Context, organizationID int, rolID, dependenciaID *int) ([]models.Usuario, error) {
	return s.repo.List(ctx, organizationID, rolID, dependenciaID)
}

func (s *UsuarioService) ActualizarRolYDependencia(ctx context.Context, organizationID, id, rolID int, dependenciaID *int, ejecutadoPor int) error {
	if err := s.repo.ActualizarRolYDependencia(ctx, organizationID, id, rolID, dependenciaID); err != nil {
		return err
	}
	_ = s.historialRepo.Registrar(ctx, &models.HistorialCambio{
		OrganizationID:  organizationID,
		Entidad:         "usuario",
		EntidadID:       id,
		UsuarioID:       &ejecutadoPor,
		Accion:          "ACTUALIZACION",
		CampoModificado: strPtr("rol/dependencia"),
	})
	return nil
}

func (s *UsuarioService) CambiarActivo(ctx context.Context, organizationID, id int, activo bool, ejecutadoPor int) error {
	if err := s.repo.ActualizarActivo(ctx, organizationID, id, activo); err != nil {
		return err
	}
	accion := "ACTUALIZACION"
	if !activo {
		accion = "ELIMINACION"
	}
	_ = s.historialRepo.Registrar(ctx, &models.HistorialCambio{
		OrganizationID: organizationID,
		Entidad:        "usuario",
		EntidadID:      id,
		UsuarioID:      &ejecutadoPor,
		Accion:         accion,
	})
	return nil
}
