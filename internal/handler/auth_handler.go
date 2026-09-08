package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/api/idtoken"

	"github.com/alcaldia/sigpa-backend/internal/config"
	"github.com/alcaldia/sigpa-backend/internal/middleware"
	"github.com/alcaldia/sigpa-backend/internal/repository"
	"github.com/alcaldia/sigpa-backend/internal/service"
	"github.com/alcaldia/sigpa-backend/pkg/apperrors"
	"github.com/alcaldia/sigpa-backend/pkg/response"
)

// AuthHandler gestiona el login con Google OAuth y con credenciales locales
// (usuario + contraseña) como alternativa mientras no esté configurado Google OAuth.
type AuthHandler struct {
	cfg         *config.Config
	usuarioSvc  *service.UsuarioService
	orgRepo     *repository.OrganizacionRepository
	usuarioRepo *repository.UsuarioRepository
}

func NewAuthHandler(cfg *config.Config, usuarioSvc *service.UsuarioService, orgRepo *repository.OrganizacionRepository, usuarioRepo *repository.UsuarioRepository) *AuthHandler {
	return &AuthHandler{cfg: cfg, usuarioSvc: usuarioSvc, orgRepo: orgRepo, usuarioRepo: usuarioRepo}
}

type googleLoginRequest struct {
	IDToken string `json:"id_token" binding:"required"` // token recibido del botón "Sign in with Google" en el frontend
}

type googleLoginResponse struct {
	Token   string      `json:"token"`
	Usuario interface{} `json:"usuario"`
}

// POST /api/v1/auth/google
func (h *AuthHandler) GoogleLogin(c *gin.Context) {
	var req googleLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "id_token requerido")
		return
	}

	ctx := c.Request.Context()

	payload, err := idtoken.Validate(ctx, req.IDToken, h.cfg.GoogleClientID)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "token de Google inválido: "+err.Error())
		return
	}

	email, _ := payload.Claims["email"].(string)
	emailVerificado, _ := payload.Claims["email_verified"].(bool)
	nombre, _ := payload.Claims["name"].(string)
	googleID := payload.Subject

	if email == "" || !emailVerificado {
		response.Error(c, http.StatusUnauthorized, "el correo de Google no está verificado")
		return
	}

	partes := strings.SplitN(strings.ToLower(email), "@", 2)
	if len(partes) != 2 {
		response.Error(c, http.StatusUnauthorized, "correo inválido")
		return
	}
	dominioCorreo := partes[1]

	// La organización se resuelve en dos pasos:
	//  1. Si el correo YA fue invitado por un Administrador (o ya existe
	//     como usuario), se usa la organización a la que pertenece esa
	//     invitación — sin importar si el correo es del dominio institucional
	//     o una cuenta personal de Google. Esto permite operar antes de tener
	//     el dominio corporativo configurado en Google Workspace.
	//  2. Si el correo nunca fue invitado, solo se permite el primer ingreso
	//     si su dominio corresponde a una organización registrada — la
	//     tabla `organizaciones` funciona como lista blanca de dominios.
	var organizationID int
	usuarioExistente, errExistente := h.usuarioSvc.BuscarPorEmail(ctx, email)
	switch {
	case errExistente == nil:
		organizationID = usuarioExistente.OrganizationID
	case errors.Is(errExistente, apperrors.ErrNotFound):
		organizacion, err := h.orgRepo.GetByDominio(ctx, dominioCorreo)
		if err != nil {
			response.Error(c, http.StatusForbidden,
				"este correo no pertenece a ninguna entidad registrada en SIGPA, y no fue invitado por un administrador")
			return
		}
		organizationID = organizacion.ID
	default:
		response.Error(c, http.StatusInternalServerError, "error al verificar el usuario: "+errExistente.Error())
		return
	}

	usuario, err := h.usuarioSvc.ResolverOCrearPorGoogle(ctx, organizationID, googleID, email, nombre)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "error al resolver el usuario: "+err.Error())
		return
	}
	if !usuario.Activo {
		response.Error(c, http.StatusForbidden, "la cuenta de usuario está inactiva; contacte al administrador")
		return
	}

	token, err := h.emitirJWT(ctx, usuario.ID, usuario.OrganizationID, usuario.Email, usuario.RolNombre, usuario.DependenciaID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "error al generar el token de sesión")
		return
	}

	response.Success(c, http.StatusOK, googleLoginResponse{Token: token, Usuario: usuario})
}

// localLoginRequest es el cuerpo del POST /auth/local
type localLoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=4"`
}

// POST /api/v1/auth/local — login con usuario y contraseña propios
func (h *AuthHandler) LocalLogin(c *gin.Context) {
	var req localLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "email y contraseña son requeridos")
		return
	}

	ctx := c.Request.Context()

	// Buscar usuario con hash (no usar GetByEmail que no incluye el hash)
	usuario, err := h.usuarioRepo.GetByEmailConHash(ctx, strings.ToLower(req.Email))
	if errors.Is(err, apperrors.ErrNotFound) {
		response.Error(c, http.StatusUnauthorized, "credenciales incorrectas")
		return
	}
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "error al verificar el usuario")
		return
	}

	if !usuario.Activo {
		response.Error(c, http.StatusForbidden, "la cuenta está inactiva; contacte al administrador")
		return
	}

	if usuario.PasswordHash == nil || *usuario.PasswordHash == "" {
		response.Error(c, http.StatusUnauthorized, "este usuario no tiene contraseña local configurada; usa Google para ingresar")
		return
	}

	// Verificar contraseña con bcrypt
	if err := bcrypt.CompareHashAndPassword([]byte(*usuario.PasswordHash), []byte(req.Password)); err != nil {
		response.Error(c, http.StatusUnauthorized, "credenciales incorrectas")
		return
	}

	// Registrar último login
	_ = h.usuarioRepo.RegistrarLogin(ctx, usuario.ID)

	token, err := h.emitirJWT(ctx, usuario.ID, usuario.OrganizationID, usuario.Email, usuario.RolNombre, usuario.DependenciaID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "error al generar el token de sesión")
		return
	}

	response.Success(c, http.StatusOK, googleLoginResponse{Token: token, Usuario: usuario})
}

// setPasswordRequest es el cuerpo del POST /auth/local/set-password
type setPasswordRequest struct {
	UserID   int    `json:"user_id" binding:"required"`
	Password string `json:"password" binding:"required,min=8"`
}

// POST /api/v1/auth/local/set-password — asignar/cambiar contraseña local (solo Administrador)
func (h *AuthHandler) SetPassword(c *gin.Context) {
	orgID := middleware.OrganizationID(c)

	var req setPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "user_id y password son requeridos (mínimo 8 caracteres)")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "error al procesar la contraseña")
		return
	}

	if err := h.usuarioRepo.ActualizarPasswordHash(c.Request.Context(), orgID, req.UserID, string(hash)); err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			response.Error(c, http.StatusNotFound, "usuario no encontrado")
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, http.StatusOK, gin.H{"actualizado": true})
}

func (h *AuthHandler) emitirJWT(_ context.Context, userID, organizationID int, email, rol string, dependenciaID *int) (string, error) {
	claims := middleware.Claims{
		UserID:         userID,
		OrganizationID: organizationID,
		Email:          email,
		RolNombre:      rol,
		DependenciaID:  dependenciaID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(12 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.cfg.JWTSecret))
}
