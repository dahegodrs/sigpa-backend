package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

// GmailService implementa la interfaz NotificadorEmail (definida en
// alerta_service.go) usando la Gmail API con una cuenta de servicio y
// domain-wide delegation, impersonando la cuenta institucional remitente
// (GMAIL_IMPERSONATE_EMAIL, ej: notificaciones@alcaldiademadrid.gov.co).
type GmailService struct {
	svc            *gmail.Service
	emailRemitente string
}

// NewGmailService construye el cliente de Gmail. Requiere que el
// administrador de Google Workspace haya autorizado el scope
// https://www.googleapis.com/auth/gmail.send para el client_id de la cuenta
// de servicio (mismo mecanismo de domain-wide delegation que Drive).
func NewGmailService(ctx context.Context, serviceAccountFile, impersonateEmail string) (*GmailService, error) {
	credsJSON, err := readCredentialsFile(serviceAccountFile)
	if err != nil {
		return nil, fmt.Errorf("error al leer credenciales de service account: %w", err)
	}

	cfg, err := google.JWTConfigFromJSON(credsJSON, gmail.GmailSendScope)
	if err != nil {
		return nil, fmt.Errorf("error al parsear credenciales de service account: %w", err)
	}
	cfg.Subject = impersonateEmail

	client := cfg.Client(ctx)
	svc, err := gmail.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("error al inicializar el cliente de Gmail: %w", err)
	}

	return &GmailService{svc: svc, emailRemitente: impersonateEmail}, nil
}

// EnviarCorreo satisface la interfaz service.NotificadorEmail.
func (g *GmailService) EnviarCorreo(ctx context.Context, destinatario, asunto, cuerpoHTML string) error {
	mensaje := construirMensajeMIME(g.emailRemitente, destinatario, asunto, cuerpoHTML)

	raw := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString([]byte(mensaje))
	gmailMsg := &gmail.Message{Raw: raw}

	_, err := g.svc.Users.Messages.Send("me", gmailMsg).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("error al enviar correo vía Gmail API a %s: %w", destinatario, err)
	}
	return nil
}

func construirMensajeMIME(remitente, destinatario, asunto, cuerpoHTML string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("From: %s\r\n", remitente))
	b.WriteString(fmt.Sprintf("To: %s\r\n", destinatario))
	b.WriteString(fmt.Sprintf("Subject: %s\r\n", asunto))
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n\r\n")
	b.WriteString(cuerpoHTML)
	return b.String()
}
