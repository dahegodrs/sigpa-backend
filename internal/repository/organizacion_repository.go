package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/alcaldia/sigpa-backend/internal/models"
	"github.com/alcaldia/sigpa-backend/pkg/apperrors"
)

type OrganizacionRepository struct {
	db *sql.DB
}

func NewOrganizacionRepository(db *sql.DB) *OrganizacionRepository {
	return &OrganizacionRepository{db: db}
}

func (r *OrganizacionRepository) GetByDominio(ctx context.Context, dominio string) (*models.Organizacion, error) {
	query := `
		SELECT id, nombre, dominio_google, activo
		FROM organizaciones WHERE dominio_google = $1 AND activo = TRUE
	`
	var o models.Organizacion
	err := r.db.QueryRowContext(ctx, query, dominio).Scan(&o.ID, &o.Nombre, &o.DominioGoogle, &o.Activo)
	if err == sql.ErrNoRows {
		return nil, apperrors.ErrInvalidTenant
	}
	if err != nil {
		return nil, fmt.Errorf("error al consultar organización: %w", err)
	}
	return &o, nil
}

func (r *OrganizacionRepository) GetByID(ctx context.Context, id int) (*models.Organizacion, error) {
	query := "SELECT id, nombre, dominio_google, activo FROM organizaciones WHERE id = $1"
	var o models.Organizacion
	err := r.db.QueryRowContext(ctx, query, id).Scan(&o.ID, &o.Nombre, &o.DominioGoogle, &o.Activo)
	if err == sql.ErrNoRows {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("error al consultar organización: %w", err)
	}
	return &o, nil
}

func (r *OrganizacionRepository) List(ctx context.Context) ([]models.Organizacion, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, nombre, dominio_google, activo FROM organizaciones WHERE activo = TRUE ORDER BY nombre")
	if err != nil {
		return nil, fmt.Errorf("error al listar organizaciones: %w", err)
	}
	defer rows.Close()

	var orgs []models.Organizacion
	for rows.Next() {
		var o models.Organizacion
		if err := rows.Scan(&o.ID, &o.Nombre, &o.DominioGoogle, &o.Activo); err != nil {
			return nil, err
		}
		orgs = append(orgs, o)
	}
	return orgs, nil
}

const temaSelectBase = `
	SELECT t.id, t.organization_id, t.logo_url, t.escudo_url, t.color_primario, t.color_secundario,
		t.color_fondo_sidebar, t.color_hover_sidebar, t.color_fondo_topbar, t.color_boton_primario,
		t.color_boton_secundario, t.color_texto, t.tipografia, t.paleta_extendida_json
	FROM organizacion_tema t
`

func scanTema(row interface {
	Scan(dest ...interface{}) error
}) (*models.OrganizacionTema, error) {
	var t models.OrganizacionTema
	err := row.Scan(
		&t.ID, &t.OrganizationID, &t.LogoURL, &t.EscudoURL, &t.ColorPrimario, &t.ColorSecundario,
		&t.ColorFondoSidebar, &t.ColorHoverSidebar, &t.ColorFondoTopbar, &t.ColorBotonPrimario,
		&t.ColorBotonSecundario, &t.ColorTexto, &t.Tipografia, &t.PaletaExtendidaJSON,
	)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// GetTemaPorOrganizationID retorna el branding de una organización ya resuelta (post-login).
func (r *OrganizacionRepository) GetTemaPorOrganizationID(ctx context.Context, organizationID int) (*models.OrganizacionTema, error) {
	row := r.db.QueryRowContext(ctx, temaSelectBase+" WHERE t.organization_id = $1", organizationID)
	tema, err := scanTema(row)
	if err == sql.ErrNoRows {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("error al consultar tema de organización: %w", err)
	}
	return tema, nil
}

// GetTemaPorDominio permite que la pantalla de login se tematice ANTES de
// autenticar, a partir del dominio institucional (ej: alcaldiadefunza.gov.co).
func (r *OrganizacionRepository) GetTemaPorDominio(ctx context.Context, dominio string) (*models.OrganizacionTema, error) {
	query := temaSelectBase + `
		INNER JOIN organizaciones o ON o.id = t.organization_id
		WHERE o.dominio_google = $1 AND o.activo = TRUE
	`
	row := r.db.QueryRowContext(ctx, query, dominio)
	tema, err := scanTema(row)
	if err == sql.ErrNoRows {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("error al consultar tema por dominio: %w", err)
	}
	return tema, nil
}
