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
	GoogleServiceAccountFile   string
	GoogleDriveImpersonateEmail string
	GoogleDriveRootFolderID    string
	GmailImpersonateEmail      string

	// SMTP: alternativa al envío vía Gmail API cuando no hay acceso a
	// admin.google.com para activar domain-wide delegation. Usa una cuenta
	// de Gmail institucional normal + contraseña de aplicación.
	SMTPHost       string
	SMTPPort       string
	SMTPUsuario    string
	SMTPContrasena string

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

		GoogleServiceAccountFile:    getEnv("GOOGLE_SERVICE_ACCOUNT_FILE", ""),
		GoogleDriveImpersonateEmail: getEnv("GOOGLE_DRIVE_IMPERSONATE_EMAIL", ""),
		GoogleDriveRootFolderID:     getEnv("GOOGLE_DRIVE_ROOT_FOLDER_ID", ""),
		GmailImpersonateEmail:       getEnv("GMAIL_IMPERSONATE_EMAIL", ""),

		SMTPHost:       getEnv("SMTP_HOST", "smtp.gmail.com"),
		SMTPPort:       getEnv("SMTP_PORT", "587"),
		SMTPUsuario:    getEnv("SMTP_USER", ""),
		SMTPContrasena: getEnv("SMTP_APP_PASSWORD", ""),

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
