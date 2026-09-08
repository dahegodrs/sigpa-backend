package service

import (
	"context"
	"fmt"
	"io"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

// DriveUploader abstrae la subida de archivos a Google Drive para que el
// resto de la aplicación (handlers/services de documentos) no dependa
// directamente del SDK de Google.
type DriveUploader interface {
	// ObtenerOCrearCarpetaVehiculo crea/resuelve solo la carpeta de la placa
	// (estructura plana, se conserva por compatibilidad).
	ObtenerOCrearCarpetaVehiculo(ctx context.Context, placa string) (string, error)
	// ObtenerOCrearRutaDocumento crea/resuelve la ruta:
	//   RootFolder → [tipoDocumento] → [placa]
	// y devuelve el ID de la carpeta de la placa dentro del tipo.
	ObtenerOCrearRutaDocumento(ctx context.Context, tipoDocumento, placa string) (string, error)
	SubirArchivo(ctx context.Context, carpetaID, nombreArchivo, mimeType string, contenido io.Reader) (fileID string, webViewLink string, err error)
}

// GoogleDriveService implementa DriveUploader SIN domain-wide delegation:
// en vez de "actuar como" una cuenta institucional (lo cual requiere acceso
// a admin.google.com para autorizarlo), la cuenta de servicio actúa como sí
// misma y opera dentro de una Unidad Compartida (Shared Drive) de la que es
// miembro directo — eso se configura con un "Compartir" normal, sin
// necesitar la consola de administración de Google Workspace.
//
// Requisito: GOOGLE_DRIVE_ROOT_FOLDER_ID debe ser el ID de una Unidad
// Compartida (o una carpeta dentro de ella) compartida explícitamente con el
// client_email de la cuenta de servicio, con rol de Administrador de
// contenido o superior. Las cuentas de servicio no tienen "Mi unidad"
// propia, por eso no sirve una carpeta normal fuera de una Unidad Compartida.
type GoogleDriveService struct {
	svc                  *drive.Service
	rootFolderID         string
	dominioInstitucional string
}

// NewGoogleDriveService construye el cliente de Drive a partir del JSON de la
// cuenta de servicio, sin impersonar a ningún usuario.
func NewGoogleDriveService(ctx context.Context, serviceAccountFile, rootFolderID, dominioInstitucional string) (*GoogleDriveService, error) {
	credsJSON, err := readCredentialsFile(serviceAccountFile)
	if err != nil {
		return nil, fmt.Errorf("error al leer credenciales de service account: %w", err)
	}

	cfg, err := google.JWTConfigFromJSON(credsJSON, drive.DriveScope)
	if err != nil {
		return nil, fmt.Errorf("error al parsear credenciales de service account: %w", err)
	}
	// Sin cfg.Subject: la cuenta de servicio actúa como sí misma, no
	// impersona a nadie — por eso no requiere domain-wide delegation.

	client := cfg.Client(ctx)
	svc, err := drive.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("error al inicializar el cliente de Drive: %w", err)
	}

	return &GoogleDriveService{svc: svc, rootFolderID: rootFolderID, dominioInstitucional: dominioInstitucional}, nil
}

// obtenerOCrearCarpeta es un helper genérico que busca o crea una carpeta
// con el nombre dado dentro del padre indicado.
func (g *GoogleDriveService) obtenerOCrearCarpeta(ctx context.Context, parentID, nombre string) (string, error) {
	query := fmt.Sprintf(
		"'%s' in parents and name = '%s' and mimeType = 'application/vnd.google-apps.folder' and trashed = false",
		parentID, nombre,
	)
	res, err := g.svc.Files.List().
		Q(query).
		Fields("files(id, name)").
		SupportsAllDrives(true).
		IncludeItemsFromAllDrives(true).
		Context(ctx).
		Do()
	if err != nil {
		return "", fmt.Errorf("error al buscar carpeta '%s' en Drive: %w", nombre, err)
	}
	if len(res.Files) > 0 {
		return res.Files[0].Id, nil
	}
	nueva := &drive.File{
		Name:     nombre,
		MimeType: "application/vnd.google-apps.folder",
		Parents:  []string{parentID},
	}
	creada, err := g.svc.Files.Create(nueva).
		Fields("id").
		SupportsAllDrives(true).
		Context(ctx).
		Do()
	if err != nil {
		return "", fmt.Errorf("error al crear carpeta '%s' en Drive: %w", nombre, err)
	}
	return creada.Id, nil
}

// ObtenerOCrearCarpetaVehiculo crea/resuelve solo la carpeta de la placa
// directamente bajo el rootFolder (estructura plana, mantenida por compatibilidad).
func (g *GoogleDriveService) ObtenerOCrearCarpetaVehiculo(ctx context.Context, placa string) (string, error) {
	return g.obtenerOCrearCarpeta(ctx, g.rootFolderID, placa)
}

// ObtenerOCrearRutaDocumento crea/resuelve la ruta:
//
//	RootFolder → [tipoDocumento] → [placa]
//
// y devuelve el ID de la carpeta de la placa dentro del tipo de documento.
// Ejemplo: "SIGPA" → "SOAT" → "ABC123"
func (g *GoogleDriveService) ObtenerOCrearRutaDocumento(ctx context.Context, tipoDocumento, placa string) (string, error) {
	// Nivel 1: carpeta del tipo de documento
	carpetaTipoID, err := g.obtenerOCrearCarpeta(ctx, g.rootFolderID, tipoDocumento)
	if err != nil {
		return "", fmt.Errorf("error al resolver carpeta de tipo '%s': %w", tipoDocumento, err)
	}
	// Nivel 2: carpeta de la placa dentro del tipo
	carpetaPlacaID, err := g.obtenerOCrearCarpeta(ctx, carpetaTipoID, placa)
	if err != nil {
		return "", fmt.Errorf("error al resolver carpeta de placa '%s' bajo '%s': %w", placa, tipoDocumento, err)
	}
	return carpetaPlacaID, nil
}

func (g *GoogleDriveService) SubirArchivo(ctx context.Context, carpetaID, nombreArchivo, mimeType string, contenido io.Reader) (string, string, error) {
	archivo := &drive.File{
		Name:    nombreArchivo,
		Parents: []string{carpetaID},
	}

	creado, err := g.svc.Files.Create(archivo).
		Media(contenido).
		Fields("id, webViewLink").
		SupportsAllDrives(true).
		Context(ctx).
		Do()
	if err != nil {
		return "", "", fmt.Errorf("error al subir archivo a Drive: %w", err)
	}

	// Se asigna permiso "anyone/reader" para que el archivo pueda visualizarse
	// desde el enlace directo sin necesidad de iniciar sesión con una cuenta
	// de Google (por ejemplo, al abrir el PDF desde un celular sin sesión
	// institucional activa). El ID del archivo es aleatorio e impredecible,
	// por lo que solo quien tenga el enlace exacto (compartido desde SIGPA)
	// puede acceder — no queda expuesto por búsqueda ni indexación.
	_, err = g.svc.Permissions.Create(creado.Id, &drive.Permission{
		Type: "anyone",
		Role: "reader",
	}).SupportsAllDrives(true).Context(ctx).Do()
	if err != nil {
		fmt.Printf("advertencia: no se pudo asignar permiso público de lectura al archivo %s: %v\n", creado.Id, err)
	}

	return creado.Id, creado.WebViewLink, nil
}
