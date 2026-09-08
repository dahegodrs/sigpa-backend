package service

import (
	"context"
	"fmt"
	"net/smtp"
)

// SMTPEmailService implementa la interfaz NotificadorEmail usando SMTP con
// una cuenta de Gmail institucional normal + contraseña de aplicación.
// Es la alternativa a GmailService (que requiere "domain-wide delegation" y
// por lo tanto acceso a admin.google.com) mientras esa vía no esté disponible.
//
// Para generar la contraseña de aplicación: la cuenta de Gmail debe tener
// verificación en 2 pasos activada; luego en myaccount.google.com/apppasswords
// se genera una contraseña específica para "Correo" — esa es la que va en
// SMTP_APP_PASSWORD, NUNCA la contraseña normal de la cuenta.
type SMTPEmailService struct {
	host       string
	port       string
	usuario    string
	contrasena string
}

func NewSMTPEmailService(host, port, usuario, contrasena string) *SMTPEmailService {
	return &SMTPEmailService{host: host, port: port, usuario: usuario, contrasena: contrasena}
}

func (s *SMTPEmailService) EnviarCorreo(ctx context.Context, destinatario, asunto, cuerpoHTML string) error {
	mensaje := construirMensajeMIME(s.usuario, destinatario, asunto, cuerpoHTML)
	auth := smtp.PlainAuth("", s.usuario, s.contrasena, s.host)
	addr := fmt.Sprintf("%s:%s", s.host, s.port)

	// smtp.SendMail negocia STARTTLS automáticamente si el servidor lo
	// soporta (smtp.gmail.com:587 sí lo hace), así que no hace falta
	// manejar TLS manualmente aquí.
	err := smtp.SendMail(addr, auth, s.usuario, []string{destinatario}, []byte(mensaje))
	if err != nil {
		return fmt.Errorf("error al enviar correo vía SMTP a %s: %w", destinatario, err)
	}
	return nil
}
