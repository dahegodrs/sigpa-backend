package main

import (
	"context"
	"io"
	"log"

	"github.com/robfig/cron/v3"

	"github.com/alcaldia/sigpa-backend/internal/config"
	"github.com/alcaldia/sigpa-backend/internal/database"
	"github.com/alcaldia/sigpa-backend/internal/handler"
	"github.com/alcaldia/sigpa-backend/internal/repository"
	"github.com/alcaldia/sigpa-backend/internal/router"
	"github.com/alcaldia/sigpa-backend/internal/service"
)

// notificadorGmailStub se usa como respaldo cuando no hay credenciales de
// Google configuradas (por ejemplo, en desarrollo local), para que el
// servidor arranque igual y solo loguee en consola en vez de fallar.
type notificadorGmailStub struct{}

func (n *notificadorGmailStub) EnviarCorreo(ctx context.Context, destinatario, asunto, cuerpoHTML string) error {
	log.Printf("[STUB Gmail - sin credenciales configuradas] Para: %s | Asunto: %s", destinatario, asunto)
	return nil
}

// driveUploaderStub, mismo propósito que el stub de Gmail, para Drive.
type driveUploaderStub struct{}

func (d *driveUploaderStub) ObtenerOCrearCarpetaVehiculo(ctx context.Context, placa string) (string, error) {
	log.Printf("[STUB Drive - sin credenciales configuradas] Carpeta solicitada para placa: %s", placa)
	return "stub-folder-" + placa, nil
}

func (d *driveUploaderStub) ObtenerOCrearRutaDocumento(ctx context.Context, tipoDocumento, placa string) (string, error) {
	log.Printf("[STUB Drive - sin credenciales configuradas] Ruta solicitada: %s/%s", tipoDocumento, placa)
	return "stub-folder-" + tipoDocumento + "-" + placa, nil
}

func (d *driveUploaderStub) SubirArchivo(ctx context.Context, carpetaID, nombreArchivo, mimeType string, contenido io.Reader) (string, string, error) {
	log.Printf("[STUB Drive - sin credenciales configuradas] Subida simulada: %s en carpeta %s", nombreArchivo, carpetaID)
	return "stub-file-id", "https://drive.google.com/stub", nil
}

func main() {
	cfg := config.Load()

	db, err := database.NewConnection(cfg)
	if err != nil {
		log.Fatalf("no se pudo conectar a la base de datos: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// --- Repositorios ---
	vehiculoRepo := repository.NewVehiculoRepository(db)
	documentoRepo := repository.NewDocumentoRepository(db)
	alertaRepo := repository.NewAlertaRepository(db)
	historialRepo := repository.NewHistorialRepository(db)
	dashboardRepo := repository.NewDashboardRepository(db)
	usuarioRepo := repository.NewUsuarioRepository(db)
	dependenciaRepo := repository.NewDependenciaRepository(db)
	catalogoRepo := repository.NewCatalogoRepository(db)
	organizacionRepo := repository.NewOrganizacionRepository(db)

	// --- Integraciones de Google Drive y envío de correo ---
	// Se configuran de forma independiente: Drive usa la cuenta de servicio
	// (sin domain-wide delegation, vía Unidad Compartida) y el correo usa
	// SMTP con una cuenta de Gmail institucional + contraseña de aplicación
	// (alternativa a Gmail API mientras no haya acceso a admin.google.com
	// para activar domain-wide delegation).
	var driveUploader service.DriveUploader
	var notificador service.NotificadorEmail

	if cfg.GoogleServiceAccountFile != "" || cfg.GoogleServiceAccountJSONBase64 != "" {
		driveSvc, err := service.NewGoogleDriveService(ctx, cfg.GoogleServiceAccountFile, cfg.GoogleDriveRootFolderID, cfg.AllowedDomain)
		if err != nil {
			log.Fatalf("error al inicializar integración con Google Drive: %v", err)
		}
		driveUploader = driveSvc
		log.Println("Integración con Google Drive inicializada correctamente")
	} else {
		log.Println("GOOGLE_SERVICE_ACCOUNT_FILE / GOOGLE_SERVICE_ACCOUNT_JSON_BASE64 no configurados: usando stub de Drive (solo para desarrollo)")
		driveUploader = &driveUploaderStub{}
	}

	if cfg.SMTPUsuario != "" && cfg.SMTPContrasena != "" {
		notificador = service.NewSMTPEmailService(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUsuario, cfg.SMTPContrasena)
		log.Printf("Envío de correo configurado vía SMTP (%s) como %s", cfg.SMTPHost, cfg.SMTPUsuario)
	} else {
		log.Println("SMTP no configurado: usando stub de correo (solo consola, para desarrollo)")
		notificador = &notificadorGmailStub{}
	}

	// --- Repositorios adicionales ---
	listaRepo := repository.NewListaRepository(db)
	programacionRepo := repository.NewProgramacionRepository(db)

	// --- Servicios ---
	vehiculoSvc := service.NewVehiculoService(vehiculoRepo, historialRepo)
	documentoSvc := service.NewDocumentoService(documentoRepo, historialRepo)
	dashboardSvc := service.NewDashboardService(dashboardRepo)
	alertaSvc := service.NewAlertaService(alertaRepo, documentoRepo, documentoSvc, vehiculoRepo, usuarioRepo, notificador)
	usuarioSvc := service.NewUsuarioService(usuarioRepo, catalogoRepo, historialRepo)
	dependenciaSvc := service.NewDependenciaService(dependenciaRepo, historialRepo)
	catalogoSvc := service.NewCatalogoService(catalogoRepo)
	temaSvc := service.NewTemaService(organizacionRepo)
	listaSvc := service.NewListaService(listaRepo)
	programacionSvc := service.NewProgramacionService(programacionRepo)

	// --- Handlers ---
	handlers := router.Handlers{
		Vehiculo:     handler.NewVehiculoHandler(vehiculoSvc),
		Documento:    handler.NewDocumentoHandler(documentoSvc, vehiculoRepo, catalogoRepo, driveUploader, notificador, cfg.SMTPUsuario),
		Dashboard:    handler.NewDashboardHandler(dashboardSvc),
		Alerta:       handler.NewAlertaHandler(alertaSvc),
		Auth:         handler.NewAuthHandler(cfg, usuarioSvc, organizacionRepo, usuarioRepo),
		Usuario:      handler.NewUsuarioHandler(usuarioSvc),
		Dependencia:  handler.NewDependenciaHandler(dependenciaSvc),
		Catalogo:     handler.NewCatalogoHandler(catalogoSvc),
		Tema:         handler.NewTemaHandler(temaSvc),
		Organizacion: handler.NewOrganizacionHandler(organizacionRepo),
		Historial:    handler.NewHistorialHandler(historialRepo),
		Lista:        handler.NewListaHandler(listaSvc),
		Programacion: handler.NewProgramacionHandler(programacionSvc),
	}

	// --- Job diario de vencimientos y alertas ---
	c := cron.New()
	_, err = c.AddFunc(cfg.AlertasCronSpec, func() {
		log.Println("Iniciando revisión diaria de vencimientos...")
		if err := alertaSvc.EjecutarRevisionDiaria(context.Background()); err != nil {
			log.Printf("error en revisión diaria: %v", err)
		}
	})
	if err != nil {
		log.Fatalf("no se pudo programar el job de alertas: %v", err)
	}
	c.Start()
	defer c.Stop()

	// --- Servidor HTTP ---
	r := router.Setup(cfg, cfg.CORSAllowedOrigins, handlers)
	log.Printf("SIGPA backend escuchando en el puerto %s", cfg.AppPort)
	if err := r.Run(":" + cfg.AppPort); err != nil {
		log.Fatalf("error al iniciar el servidor: %v", err)
	}
}
