package service

import "os"

// readCredentialsFile lee el archivo JSON de la cuenta de servicio de Google
// (descargado desde la consola de Google Cloud). Se usa tanto para Drive como
// para Gmail, ya que ambas integraciones comparten el mismo mecanismo de
// domain-wide delegation.
func readCredentialsFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}
