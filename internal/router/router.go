package router

import (
	"github.com/gin-gonic/gin"

	"github.com/alcaldia/sigpa-backend/internal/config"
	"github.com/alcaldia/sigpa-backend/internal/handler"
	"github.com/alcaldia/sigpa-backend/internal/middleware"
)

type Handlers struct {
	Vehiculo     *handler.VehiculoHandler
	Documento    *handler.DocumentoHandler
	Dashboard    *handler.DashboardHandler
	Alerta       *handler.AlertaHandler
	Auth         *handler.AuthHandler
	Usuario      *handler.UsuarioHandler
	Dependencia  *handler.DependenciaHandler
	Catalogo     *handler.CatalogoHandler
	Tema         *handler.TemaHandler
	Organizacion *handler.OrganizacionHandler
	Historial    *handler.HistorialHandler
	Lista        *handler.ListaHandler
	Programacion *handler.ProgramacionHandler
}

// Setup registra todas las rutas de la API bajo /api/v1, aplicando el
// middleware de autenticación (JWT) y de roles donde corresponde.
func Setup(cfg *config.Config, corsOrigins []string, h Handlers) *gin.Engine {
	r := gin.Default()

	r.Use(middleware.CORS(corsOrigins))

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	api := r.Group("/api/v1")
	{
		// Autenticación (sin JWT, es el punto de entrada)
		auth := api.Group("/auth")
		{
			auth.POST("/google", h.Auth.GoogleLogin)
		}

		// Rutas públicas adicionales: la pantalla de login necesita saber el
		// branding de la Alcaldía ANTES de que el usuario se autentique.
		public := api.Group("/public")
		{
			public.GET("/tema", h.Tema.ObtenerPorDominio)
		}

		// Login con credenciales locales (usuario + contraseña) — no requiere JWT
		auth.POST("/local", h.Auth.LocalLogin)
		// Cambio de contraseña — requiere JWT de Administrador
		authProtegido := api.Group("/auth")
		authProtegido.Use(middleware.AuthRequired(cfg.JWTSecret))
		authProtegido.POST("/local/set-password", middleware.RequireRoles("Administrador"), h.Auth.SetPassword)

		// Todo lo demás requiere JWT válido
		protected := api.Group("")
		protected.Use(middleware.AuthRequired(cfg.JWTSecret))
		{
			protected.GET("/catalogos", h.Catalogo.Todos)
			protected.GET("/tema", h.Tema.ObtenerPropio)
			protected.GET("/organizacion", h.Organizacion.ObtenerPropia)

			vehiculos := protected.Group("/vehiculos")
			{
				vehiculos.GET("", h.Vehiculo.List)
				vehiculos.GET("/:id", h.Vehiculo.Get)
				vehiculos.GET("/:id/historial", h.Vehiculo.Historial)
				vehiculos.GET("/:id/documentos", h.Documento.ListarPorVehiculo)
				vehiculos.GET("/:id/documentos/:tipoId/historico", h.Documento.Historico)

				// Mutaciones: Administrador y Dependencia (dueños de sus propios vehículos)
				vehiculos.POST("", middleware.RequireRoles("Administrador"), h.Vehiculo.Create)
				vehiculos.PUT("/:id", middleware.RequireRoles("Administrador", "Dependencia"), h.Vehiculo.Update)
				vehiculos.DELETE("/:id", middleware.RequireRoles("Administrador"), h.Vehiculo.Delete)
				vehiculos.POST("/:id/documentos/upload",
					middleware.RequireRoles("Administrador", "Dependencia"), h.Documento.SubirNuevaVersion)
				vehiculos.POST("/:id/documentos/notificar",
					middleware.RequireRoles("Administrador", "Dependencia"), h.Documento.NotificarDocumento)
			}

			dependencias := protected.Group("/dependencias")
			{
				dependencias.GET("", h.Dependencia.List)
				dependencias.POST("", middleware.RequireRoles("Administrador"), h.Dependencia.Create)
				dependencias.PUT("/:id", middleware.RequireRoles("Administrador"), h.Dependencia.Update)
				dependencias.DELETE("/:id", middleware.RequireRoles("Administrador"), h.Dependencia.Delete)
			}

			usuarios := protected.Group("/usuarios")
			usuarios.Use(middleware.RequireRoles("Administrador"))
			{
				usuarios.GET("", h.Usuario.List)
				usuarios.POST("", h.Usuario.Invitar)
				usuarios.PUT("/:id/rol", h.Usuario.ActualizarRol)
				usuarios.PUT("/:id/activo", h.Usuario.ActualizarActivo)
			}

			dashboard := protected.Group("/dashboard")
			{
				dashboard.GET("", h.Dashboard.Resumen)
			}

			alertas := protected.Group("/alertas")
			{
				alertas.GET("", h.Alerta.List)
				alertas.POST("/ejecutar-revision",
					middleware.RequireRoles("Administrador"), h.Alerta.EjecutarRevisionManual)
				alertas.PUT("/marcar-todas-leidas", h.Alerta.MarcarTodasLeidas)
				alertas.PUT("/:id/leida", h.Alerta.MarcarLeida)

				alertas.GET("/config", h.Alerta.ListarConfig)
				alertas.POST("/config", middleware.RequireRoles("Administrador"), h.Alerta.CrearConfig)
				alertas.PUT("/config/:id", middleware.RequireRoles("Administrador"), h.Alerta.ActualizarConfig)
				alertas.DELETE("/config/:id", middleware.RequireRoles("Administrador"), h.Alerta.EliminarConfig)
			}

			protected.GET("/historial/reciente", h.Historial.Reciente)

			documentos := protected.Group("/documentos")
			{
				documentos.GET("", h.Documento.ListarGlobal)
				documentos.GET("/conteo-por-tipo", h.Documento.ConteoPorTipo)
			}

			// Listas configurables (conductores, actividades, etc.)
			listas := protected.Group("/listas")
			{
				listas.GET("", h.Lista.Listar)
				listas.POST("", middleware.RequireRoles("Administrador"), h.Lista.Crear)
				listas.PUT("/:id", middleware.RequireRoles("Administrador"), h.Lista.Actualizar)
				listas.DELETE("/:id", middleware.RequireRoles("Administrador"), h.Lista.Eliminar)
			}

			// Programación diaria de vehículos (15-FR-36)
			programaciones := protected.Group("/programaciones")
			{
				programaciones.GET("", h.Programacion.Listar)
				programaciones.POST("", middleware.RequireRoles("Administrador", "Dependencia"), h.Programacion.Crear)
				programaciones.GET("/:id", h.Programacion.Obtener)
				programaciones.PUT("/:id", middleware.RequireRoles("Administrador", "Dependencia"), h.Programacion.Actualizar)
				programaciones.DELETE("/:id", middleware.RequireRoles("Administrador"), h.Programacion.Eliminar)
			}
		}
	}

	return r
}
