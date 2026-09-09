package config

import (
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// Config centraliza toda la configuración de la aplicación.
type Config struct {
	AppPort string

	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string

	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string
	// AllowedDomain queda en desuso para la validación de login (ahora la
	// organización se resuelve dinámicamente por el dominio del correo contra
	// la tabla `organizaciones`, permitiendo varias Alcaldías en un mismo
	// despliegue). Se conserva solo por compatibilidad con scripts o
	// despliegues de un solo tenant que quieran seguir usándolo como
	// referencia informativa.
	AllowedDomain string

	// Service account con domain-wide delegation, usado tanto para Drive como
	// para Gmail (impersonando la cuenta institucional correspondiente).
	GoogleServiceAccountFile       string
	GoogleServiceAccountJSONBase64 string
	GoogleDriveImpersonateEmail    string
	GoogleDriveRootFolderID        string
	GmailImpersonateEmail          string

	// SMTP: alternativa al envío vía Gmail API cuando no hay acceso a
	// admin.google.com para activar domain-wide delegation. Usa una cuenta
	// de Gmail institucional normal + contraseña de aplicación.
	SMTPHost       string
	SMTPPort       string
	SMTPUsuario    string
	SMTPContrasena string

	// Brevo (antes Sendinblue): API HTTP para envío de correos transaccionales.
	// Se prefiere sobre SMTP en despliegues cloud (Render, etc.) porque el
	// puerto 587 suele estar bloqueado, mientras que HTTPS nunca lo está.
	BrevoAPIKey          string
	BrevoRemitenteEmail  string
	BrevoRemitenteNombre string

	JWTSecret string

	GmailSenderEmail string

	AlertasCronSpec string // expresión cron para el job diario de vencimientos

	CORSAllowedOrigins []string
}

// Load lee el archivo .env (si existe) y las variables de entorno del sistema.
func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("No se encontró archivo .env, se usarán variables de entorno del sistema")
	}

	return &Config{
		AppPort: getEnv("APP_PORT", "8080"),

		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "1433"),
		DBUser:     getEnv("DB_USER", "sa"),
		DBPassword: getEnv("DB_PASSWORD", ""),
		DBName:     getEnv("DB_NAME", "SIGPA"),

		GoogleClientID:     getEnv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
		GoogleRedirectURL:  getEnv("GOOGLE_REDIRECT_URL", ""),
		AllowedDomain:      getEnv("ALLOWED_DOMAIN", ""),

		GoogleServiceAccountFile:       getEnv("GOOGLE_SERVICE_ACCOUNT_FILE", ""),
		GoogleServiceAccountJSONBase64: getEnv("GOOGLE_SERVICE_ACCOUNT_JSON_BASE64", ""),
		GoogleDriveImpersonateEmail:    getEnv("GOOGLE_DRIVE_IMPERSONATE_EMAIL", ""),
		GoogleDriveRootFolderID:        getEnv("GOOGLE_DRIVE_ROOT_FOLDER_ID", ""),
		GmailImpersonateEmail:          getEnv("GMAIL_IMPERSONATE_EMAIL", ""),

		SMTPHost:       getEnv("SMTP_HOST", "smtp.gmail.com"),
		SMTPPort:       getEnv("SMTP_PORT", "587"),
		SMTPUsuario:    getEnv("SMTP_USER", ""),
		SMTPContrasena: getEnv("SMTP_APP_PASSWORD", ""),

		BrevoAPIKey:          getEnv("BREVO_API_KEY", ""),
		BrevoRemitenteEmail:  getEnv("BREVO_REMITENTE_EMAIL", "Patio@funza-cundinamarca.gov.co"),
		BrevoRemitenteNombre: getEnv("BREVO_REMITENTE_NOMBRE", "SIGPA - Patio Alcaldía de Funza"),

		JWTSecret: getEnv("JWT_SECRET", ""),

		GmailSenderEmail: getEnv("GMAIL_SENDER_EMAIL", ""),

		AlertasCronSpec: getEnv("ALERTAS_CRON_SPEC", "0 6 * * *"), // 6:00 AM todos los días

		CORSAllowedOrigins: strings.Split(getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000"), ","),
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}
