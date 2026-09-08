package service

import (
	"encoding/base64"
	"fmt"
	"os"
)

// readCredentialsFile obtiene el JSON de la cuenta de servicio de Google.
// Soporta dos mecanismos, en este orden de prioridad:
//
//  1. Variable de entorno GOOGLE_SERVICE_ACCOUNT_JSON_BASE64: el contenido
//     completo del archivo JSON codificado en Base64. Es la opción recomendada
//     para plataformas cloud (Render, Railway, etc.) donde no se puede subir
//     un archivo de credenciales junto al código por git (correcto, ya que son
//     credenciales sensibles que no deben versionarse).
//  2. Ruta de archivo en disco (uso local/desarrollo): se lee directamente
//     desde el filesystem, típicamente ./credentials/sigpa-service-account.json
func readCredentialsFile(path string) ([]byte, error) {
	if b64 := os.Getenv("GOOGLE_SERVICE_ACCOUNT_JSON_BASE64"); b64 != "" {
		decoded, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			return nil, fmt.Errorf("error al decodificar GOOGLE_SERVICE_ACCOUNT_JSON_BASE64: %w", err)
		}
		return decoded, nil
	}
	return os.ReadFile(path)
}
