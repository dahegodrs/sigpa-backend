# SIGPA Backend (Go + Gin)

Backend del Sistema Integral de Gestión del Parque Automotor y Control de
Vencimientos Documentales. Fase 2 del roadmap del proyecto — **completa**.

## Arquitectura

```
cmd/api/main.go        → wiring de dependencias, cron, arranque del servidor
internal/config        → carga de variables de entorno
internal/database      → conexión a SQL Server
internal/models        → structs de dominio (reflejan 01_sigpa_schema.sql)
internal/repository    → acceso a datos (SQL directo, sin ORM)
internal/service       → reglas de negocio, validaciones, historial, integraciones Google
internal/handler       → controladores HTTP (Gin)
internal/middleware    → JWT, roles, CORS
internal/router        → registro de rutas
pkg/response           → formato estándar de respuestas JSON
pkg/apperrors          → errores tipados de dominio
```

Patrón por módulo: `handler → service → repository`. Multi-tenant vía
`organization_id`, resuelto automáticamente desde el JWT en cada request
(`middleware.OrganizationID(c)`), nunca confiado desde el body/query del cliente.

## Estado de avance: Fase 2 completa

- **Vehículos**: CRUD completo, filtros, paginación, historial, validación de
  placa y unicidad por organización.
- **Documentos**: versionado automático, cálculo de estado (Vigente / Próximo
  a vencer / Vencido / Pendiente) según la regla de 45 días, **subida real de
  archivos a Google Drive** (organiza una carpeta por placa, evita duplicidad,
  genera enlace de solo lectura restringido al dominio institucional).
- **Alertas**: motor de revisión diaria (umbrales 45/30/15/7/1/vencido), job
  programado con cron, **resolución real de destinatarios** (responsable del
  vehículo + usuarios de su dependencia; para niveles Urgente/Crítica se
  agregan automáticamente Administrador y Gerencia), y **envío real por
  Gmail API**.
- **Dashboard**: KPIs, agrupaciones por dependencia/tipo/estado, vencimientos
  por mes.
- **Auth**: login con Google Identity Services — el backend valida el
  `id_token` contra los servidores de Google, verifica que el correo
  pertenezca al dominio institucional (`ALLOWED_DOMAIN`), y crea o resuelve el
  usuario automáticamente (rol `Consulta` por defecto en el primer ingreso).
- **Usuarios**: listado y gestión de rol/dependencia/estado activo desde el
  panel de Administración.
- **Dependencias**: CRUD completo.
- **Catálogos**: endpoint único (`GET /api/v1/catalogos`) para poblar los
  selects de roles, estados, tipos de vehículo y tipos de documento en el
  frontend.
- **Tema/branding multi-tenant**: cada Alcaldía tiene su propio logo, paleta
  de colores y tipografía guardados en `organizacion_tema` (migración
  `02_sigpa_tema_organizacional.sql`). El login se resuelve dinámicamente por
  el dominio del correo institucional, así **un mismo backend sirve a varias
  Alcaldías** sin redesplegar nada — dar de alta una nueva Alcaldía es solo
  insertar filas en `organizaciones` y `organizacion_tema`.
  - `GET /api/v1/public/tema?dominio=alcaldiadefunza.gov.co` — sin
    autenticación, para tematizar la pantalla de login antes de que el
    usuario inicie sesión.
  - `GET /api/v1/tema` — con sesión, para refrescar el branding dentro de la
    app ya autenticada.

### Modo desarrollo sin credenciales de Google

Si `GOOGLE_SERVICE_ACCOUNT_FILE` no está configurado, el backend arranca
igual usando stubs de Drive y Gmail que solo registran en consola lo que
habrían hecho — así se puede desarrollar y probar el resto de la aplicación
sin depender de que ya estén listas las credenciales institucionales.

### Pendiente de configuración (no de código)

Para que Drive/Gmail funcionen de verdad en producción, quien administre
Google Workspace debe:

1. Crear una cuenta de servicio en Google Cloud Console y descargar su JSON.
2. Habilitar **domain-wide delegation** para esa cuenta.
3. En `admin.google.com > Seguridad > Controles de API > Delegación de todo
   el dominio`, autorizar el `client_id` de la cuenta de servicio con los
   scopes `https://www.googleapis.com/auth/drive` y
   `https://www.googleapis.com/auth/gmail.send`.
4. Completar `GOOGLE_SERVICE_ACCOUNT_FILE`, `GOOGLE_DRIVE_IMPERSONATE_EMAIL`,
   `GOOGLE_DRIVE_ROOT_FOLDER_ID` y `GMAIL_IMPERSONATE_EMAIL` en `.env`.
5. Registrar un **OAuth Client ID** (tipo "Web application") para el
   frontend en Google Cloud Console, y usar ese mismo ID como
   `GOOGLE_CLIENT_ID` en el backend (es la audiencia contra la que se valida
   el `id_token`).

Pendiente para fases posteriores del roadmap (no de esta fase):

- Exportación a PDF/Excel (Fase 4).
- Mantenimientos, combustible, kilometraje, conductores, etc. (Fase 5).

## Cómo correr localmente

```bash
cp .env.example .env
# Ajustar credenciales de SQL Server, Google y JWT_SECRET

go mod tidy
go run ./cmd/api
```

Requiere que los scripts `01_sigpa_schema.sql` y `02_sigpa_tema_organizacional.sql`
ya hayan sido ejecutados, en ese orden, contra la instancia de SQL Server.

> Nota: este código se generó sin acceso a internet para descargar módulos de
> Go, por lo que no se pudo correr `go mod tidy && go build ./...` en este
> entorno. Ejecútalo como primer paso al bajar el proyecto para atrapar
> cualquier detalle menor de compilación o de versión de dependencias.

## Endpoints principales

| Método | Ruta | Rol requerido |
|---|---|---|
| POST | /api/v1/auth/google | público |
| GET | /api/v1/catalogos | cualquier rol autenticado |
| GET | /api/v1/vehiculos | cualquier rol autenticado |
| POST | /api/v1/vehiculos | Administrador |
| PUT | /api/v1/vehiculos/:id | Administrador, Dependencia |
| DELETE | /api/v1/vehiculos/:id | Administrador |
| GET | /api/v1/vehiculos/:id/documentos | cualquier rol autenticado |
| POST | /api/v1/vehiculos/:id/documentos/upload | Administrador, Dependencia |
| GET | /api/v1/dependencias | cualquier rol autenticado |
| POST/PUT/DELETE | /api/v1/dependencias | Administrador |
| GET | /api/v1/usuarios | Administrador |
| PUT | /api/v1/usuarios/:id/rol | Administrador |
| PUT | /api/v1/usuarios/:id/activo | Administrador |
| GET | /api/v1/dashboard | cualquier rol autenticado |
| GET | /api/v1/alertas | cualquier rol autenticado |
| POST | /api/v1/alertas/ejecutar-revision | Administrador |
