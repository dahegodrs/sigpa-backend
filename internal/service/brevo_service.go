package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// BrevoEmailService implementa la interfaz NotificadorEmail usando la API
// HTTP de Brevo (https://api.brevo.com/v3/smtp/email), en vez de SMTP.
//
// Se eligió Brevo sobre SMTP porque plataformas cloud gratuitas como Render
// suelen bloquear las conexiones salientes al puerto 587 (SMTP) por defecto
// anti-spam — la petición se queda esperando ~2 minutos y termina en timeout.
// La API de Brevo usa HTTPS (puerto 443), que nunca está bloqueado, por lo
// que resuelve el problema sin necesitar un plan de pago en Render.
type BrevoEmailService struct {
	apiKey        string
	remitenteMail string
	remitenteNom  string
	httpClient    *http.Client
}

func NewBrevoEmailService(apiKey, remitenteMail, remitenteNombre string) *BrevoEmailService {
	return &BrevoEmailService{
		apiKey:        apiKey,
		remitenteMail: remitenteMail,
		remitenteNom:  remitenteNombre,
		httpClient:    &http.Client{Timeout: 15 * time.Second},
	}
}

type brevoSender struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type brevoRecipient struct {
	Email string `json:"email"`
}

type brevoSendRequest struct {
	Sender      brevoSender      `json:"sender"`
	To          []brevoRecipient `json:"to"`
	Subject     string           `json:"subject"`
	HTMLContent string           `json:"htmlContent"`
}

// EnviarCorreo satisface la interfaz service.NotificadorEmail.
func (b *BrevoEmailService) EnviarCorreo(ctx context.Context, destinatario, asunto, cuerpoHTML string) error {
	payload := brevoSendRequest{
		Sender:      brevoSender{Name: b.remitenteNom, Email: b.remitenteMail},
		To:          []brevoRecipient{{Email: destinatario}},
		Subject:     asunto,
		HTMLContent: cuerpoHTML,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("error al construir el payload de Brevo: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.brevo.com/v3/smtp/email", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("error al crear la petición a Brevo: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("api-key", b.apiKey)

	res, err := b.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error al conectar con la API de Brevo: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode >= 300 {
		respBody, _ := io.ReadAll(res.Body)
		return fmt.Errorf("Brevo respondió con estado %d: %s", res.StatusCode, string(respBody))
	}

	return nil
}
