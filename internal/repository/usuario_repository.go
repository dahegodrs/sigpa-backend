package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/alcaldia/sigpa-backend/internal/models"
	"github.com/alcaldia/sigpa-backend/pkg/apperrors"
)

type UsuarioRepository struct {
	db *sql.DB
}

func NewUsuarioRepository(db *sql.DB) *UsuarioRepository {
	return &UsuarioRepository{db: db}
}

const usuarioSelectBase = `
	SELECT u.id, u.organization_id, u.google_id, u.email, u.nombre, u.rol_id, r.nombre AS rol_nombre,
		u.dependencia_id, u.activo, u.ultimo_login, u.fecha_creacion
	FROM usuarios u
	INNER JOIN roles r ON r.id = u.rol_id
`

func scanUsuario(row interface {
	Scan(dest ...interface{}) error
}) (*models.Usuario, error) {
	var u models.Usuario
	err := row.Scan(
		&u.ID, &u.OrganizationID, &u.GoogleID, &u.Email, &u.Nombre, &u.RolID, &u.RolNombre,
		&u.DependenciaID, &u.Activo, &u.UltimoLogin, &u.FechaCreacion,
	)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UsuarioRepository) GetByGoogleID(ctx context.Context, googleID string) (*models.Usuario, error) {
	query := usuarioSelectBase + " WHERE u.google_id = $1"
	row := r.db.QueryRowContext(ctx, query, googleID)
	u, err := scanUsuario(row)
	if err == sql.ErrNoRows {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("error al consultar usuario por google_id: %w", err)
	}
	return u, nil
}

func (r *UsuarioRepository) GetByEmail(ctx context.Context, email string) (*models.Usuario, error) {
	query := usuarioSelectBase + " WHERE u.email = $1"
	row := r.db.QueryRowContext(ctx, query, email)
	u, err := scanUsuario(row)
	if err == sql.ErrNoRows {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("error al consultar usuario por email: %w", err)
	}
	return u, nil
}

func (r *UsuarioRepository) GetByID(ctx context.Context, organizationID, id int) (*models.Usuario, error) {
	query := usuarioSelectBase + " WHERE u.id = $1 AND u.organization_id = $2"
	row := r.db.QueryRowContext(ctx, query, id, organizationID)
	u, err := scanUsuario(row)
	if err == sql.ErrNoRows {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("error al consultar usuario: %w", err)
	}
	return u, nil
}

func (r *UsuarioRepository) List(ctx context.Context, organizationID int, rolID *int, dependenciaID *int) ([]models.Usuario, error) {
	query := usuarioSelectBase + " WHERE u.organization_id = $1"
	args := []interface{}{organizationID}
	argPos := 2

	if rolID != nil {
		query += fmt.Sprintf(" AND u.rol_id = $%d", argPos)
		args = append(args, *rolID)
		argPos++
	}
	if dependenciaID != nil {
		query += fmt.Sprintf(" AND u.dependencia_id = $%d", argPos)
		args = append(args, *dependenciaID)
		argPos++
	}
	query += " ORDER BY u.nombre"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("error al listar usuarios: %w", err)
	}
	defer rows.Close()

	var usuarios []models.Usuario
	for rows.Next() {
		u, err := scanUsuario(rows)
		if err != nil {
			return nil, err
		}
		usuarios = append(usuarios, *u)
	}
	return usuarios, nil
}

// Create inserta un nuevo usuario, típicamente en el primer login con Google.
func (r *UsuarioRepository) Create(ctx context.Context, u *models.Usuario) (int, error) {
	query := `
		INSERT INTO usuarios (organization_id, google_id, email, nombre, rol_id, dependencia_id, activo, ultimo_login)
		VALUES ($1, $2, $3, $4, $5, $6, TRUE, NOW())
		RETURNING id
	`
	var newID int
	err := r.db.QueryRowContext(ctx, query,
		u.OrganizationID, u.GoogleID, u.Email, u.Nombre, u.RolID, u.DependenciaID,
	).Scan(&newID)
	if err != nil {
		return 0, fmt.Errorf("error al crear usuario: %w", err)
	}
	return newID, nil
}

// ActualizarRolYDependencia permite a un Administrador asignar rol/dependencia desde el panel de Administración.
func (r *UsuarioRepository) ActualizarRolYDependencia(ctx context.Context, organizationID, id, rolID int, dependenciaID *int) error {
	query := `
		UPDATE usuarios SET rol_id = $1, dependencia_id = $2
		WHERE id = $3 AND organization_id = $4
	`
	result, err := r.db.ExecContext(ctx, query, rolID, dependenciaID, id, organizationID)
	if err != nil {
		return fmt.Errorf("error al actualizar usuario: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *UsuarioRepository) ActualizarActivo(ctx context.Context, organizationID, id int, activo bool) error {
	query := "UPDATE usuarios SET activo = $1 WHERE id = $2 AND organization_id = $3"
	result, err := r.db.ExecContext(ctx, query, activo, id, organizationID)
	if err != nil {
		return fmt.Errorf("error al actualizar estado del usuario: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *UsuarioRepository) RegistrarLogin(ctx context.Context, id int) error {
	_, err := r.db.ExecContext(ctx, "UPDATE usuarios SET ultimo_login = NOW() WHERE id = $1", id)
	return err
}

// CrearPreregistrado da de alta un usuario ANTES de su primer login (lo hace
// un Administrador desde el panel, asignándole rol/dependencia de una vez).
// Como `google_id` es NOT NULL y único, se usa un valor placeholder
// ("pending:"+email) hasta que la persona inicie sesión por primera vez con
// Google; en ese momento ActivarPreregistrado reemplaza el placeholder por
// el google_id real, sin perder el rol/dependencia ya asignados.
func (r *UsuarioRepository) CrearPreregistrado(ctx context.Context, u *models.Usuario) (int, error) {
	query := `
		INSERT INTO usuarios (organization_id, google_id, email, nombre, rol_id, dependencia_id, activo)
		VALUES ($1, $2, $3, $4, $5, $6, TRUE)
		RETURNING id
	`
	googleIDPlaceholder := "pending:" + u.Email
	var newID int
	err := r.db.QueryRowContext(ctx, query,
		u.OrganizationID, googleIDPlaceholder, u.Email, u.Nombre, u.RolID, u.DependenciaID,
	).Scan(&newID)
	if err != nil {
		return 0, fmt.Errorf("error al pre-registrar usuario: %w", err)
	}
	return newID, nil
}

// ActualizarGoogleID reemplaza el placeholder "pending:" por el google_id
// real, en el primer login efectivo de un usuario pre-registrado.
func (r *UsuarioRepository) ActualizarGoogleID(ctx context.Context, id int, googleID string) error {
	_, err := r.db.ExecContext(ctx, "UPDATE usuarios SET google_id = $1 WHERE id = $2", googleID, id)
	if err != nil {
		return fmt.Errorf("error al activar usuario pre-registrado: %w", err)
	}
	return nil
}

// EmailsPorRoles resuelve los correos de todos los usuarios activos de la
// organización que tengan alguno de los roles indicados. Se usa para
// notificar a Administrador/Gerencia en alertas Urgente y Crítica.
func (r *UsuarioRepository) EmailsPorRoles(ctx context.Context, organizationID int, roles []string) ([]string, error) {
	if len(roles) == 0 {
		return nil, nil
	}
	placeholders := ""
	args := []interface{}{organizationID}
	for i, rol := range roles {
		if i > 0 {
			placeholders += ","
		}
		placeholders += fmt.Sprintf("$%d", i+2)
		args = append(args, rol)
	}
	query := fmt.Sprintf(`
		SELECT DISTINCT u.email FROM usuarios u
		INNER JOIN roles r ON r.id = u.rol_id
		WHERE u.organization_id = $1 AND u.activo = TRUE AND r.nombre IN (%s)
	`, placeholders)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("error al resolver emails por rol: %w", err)
	}
	defer rows.Close()

	var emails []string
	for rows.Next() {
		var email string
		if err := rows.Scan(&email); err != nil {
			return nil, err
		}
		emails = append(emails, email)
	}
	return emails, nil
}

// GetByEmailConHash retorna el usuario incluyendo password_hash para verificar credenciales locales.
// No usar para respuestas API — el hash nunca sale del backend.
func (r *UsuarioRepository) GetByEmailConHash(ctx context.Context, email string) (*models.Usuario, error) {
	query := `
		SELECT u.id, u.organization_id, u.google_id, u.email, u.nombre, u.rol_id, r.nombre AS rol_nombre,
			u.dependencia_id, u.activo, u.ultimo_login, u.fecha_creacion, u.password_hash
		FROM usuarios u
		INNER JOIN roles r ON r.id = u.rol_id
		WHERE u.email = $1
	`
	var u models.Usuario
	err := r.db.QueryRowContext(ctx, query, email).Scan(
		&u.ID, &u.OrganizationID, &u.GoogleID, &u.Email, &u.Nombre, &u.RolID, &u.RolNombre,
		&u.DependenciaID, &u.Activo, &u.UltimoLogin, &u.FechaCreacion, &u.PasswordHash,
	)
	if err == sql.ErrNoRows {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("error al consultar usuario con hash: %w", err)
	}
	return &u, nil
}

// ActualizarPasswordHash guarda el hash bcrypt de la contraseña local para un usuario.
// Solo puede ser llamado por un Administrador autenticado.
func (r *UsuarioRepository) ActualizarPasswordHash(ctx context.Context, organizationID, userID int, hash string) error {
	result, err := r.db.ExecContext(ctx,
		"UPDATE usuarios SET password_hash = $1 WHERE id = $2 AND organization_id = $3",
		hash, userID, organizationID,
	)
	if err != nil {
		return fmt.Errorf("error al actualizar password_hash: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

// EmailsPorDependencia retorna los correos de los usuarios activos asignados a una dependencia.
func (r *UsuarioRepository) EmailsPorDependencia(ctx context.Context, organizationID, dependenciaID int) ([]string, error) {
	query := `
		SELECT DISTINCT email FROM usuarios
		WHERE organization_id = $1 AND dependencia_id = $2 AND activo = TRUE
	`
	rows, err := r.db.QueryContext(ctx, query, organizationID, dependenciaID)
	if err != nil {
		return nil, fmt.Errorf("error al resolver emails por dependencia: %w", err)
	}
	defer rows.Close()

	var emails []string
	for rows.Next() {
		var email string
		if err := rows.Scan(&email); err != nil {
			return nil, err
		}
		emails = append(emails, email)
	}
	return emails, nil
}
