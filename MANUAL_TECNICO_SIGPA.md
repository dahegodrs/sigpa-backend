# Manual Técnico — SIGPA

### Sistema Integral de Gestión del Parque Automotor

**Alcaldía de Funza — Cundinamarca**

---

## Tabla de Contenidos

**PARTE A — Arquitectura y Tecnologías**

1. [Resumen ejecutivo del sistema](#1-resumen-ejecutivo-del-sistema)
2. [Stack tecnológico](#2-stack-tecnológico)
3. [Arquitectura del Backend](#3-arquitectura-del-backend)
4. [Arquitectura del Frontend](#4-arquitectura-del-frontend)
5. [Modelo de datos](#5-modelo-de-datos)
6. [Autenticación y autorización](#6-autenticación-y-autorización)
7. [Integraciones externas](#7-integraciones-externas)
8. [Sistema de alertas y vencimientos](#8-sistema-de-alertas-y-vencimientos)

**PARTE B — Guía de Despliegue Paso a Paso** 9. [Requisitos previos](#9-requisitos-previos) 10. [Repositorios en GitHub](#10-repositorios-en-github) 11. [Base de datos — Supabase](#11-base-de-datos--supabase) 12. [Google Cloud — Cuenta de servicio y Drive](#12-google-cloud--cuenta-de-servicio-y-drive) 13. [Brevo — Envío de correos transaccionales](#13-brevo--envío-de-correos-transaccionales) 14. [Backend — Despliegue en Render](#14-backend--despliegue-en-render) 15. [Frontend — Despliegue en Vercel](#15-frontend--despliegue-en-vercel) 16. [Google OAuth — Login con Workspace](#16-google-oauth--login-con-workspace) 17. [Checklist final de verificación](#17-checklist-final-de-verificación) 18. [Troubleshooting — problemas conocidos y soluciones](#18-troubleshooting--problemas-conocidos-y-soluciones)

---

# PARTE A — Arquitectura y Tecnologías

## 1. Resumen ejecutivo del sistema

SIGPA es una plataforma web multi-tenant (multi-Alcaldía) diseñada para centralizar la administración del parque automotor de una entidad pública: registro de vehículos, control de documentos obligatorios (SOAT, Tecnomecánica, Póliza de seguros, Tarjeta de propiedad), generación automática de alertas de vencimiento, dashboard ejecutivo con indicadores en tiempo real, y programación diaria de uso de vehículos.

### Diagrama de arquitectura general

```
┌─────────────────┐        HTTPS        ┌──────────────────┐
│   Frontend       │ ──────────────────► │   Backend API     │
│  Next.js / React │ ◄────────────────── │   Go + Gin         │
│  (Vercel)        │       JSON/REST      │  (Render)          │
└─────────────────┘                      └──────┬────────────┘
                                                  │
                    ┌─────────────────────────────┼───────────────────────────┐
                    │                             │                           │
              ┌─────▼──────┐              ┌───────▼───────┐          ┌────────▼────────┐
              │ PostgreSQL  │              │ Google Drive   │          │   Brevo API      │
              │ (Supabase)  │              │ (documentos)   │          │ (correo transac.)│
              └────────────┘              └───────────────┘          └─────────────────┘
                                                  │
                                          ┌────────▼────────┐
                                          │ Google OAuth 2.0 │
                                          │ (login Workspace) │
                                          └─────────────────┘
```

El sistema opera bajo un modelo **cliente-servidor desacoplado**: el frontend consume exclusivamente la API REST del backend (nunca accede a la base de datos ni a Google Drive directamente). Todo el estado persistente vive en PostgreSQL; los archivos binarios (PDFs, imágenes de documentos) se almacenan en Google Drive, y solo se guarda el enlace/ID del archivo en la base de datos.

---

## 2. Stack tecnológico

### Backend

| Componente           | Tecnología                                 | Versión | Propósito                                                     |
| -------------------- | ------------------------------------------ | ------- | ------------------------------------------------------------- |
| Lenguaje             | Go                                         | 1.21    | Rendimiento, concurrencia nativa, binarios estáticos livianos |
| Framework HTTP       | Gin (`gin-gonic/gin`)                      | 1.10.0  | Router, middleware, binding de JSON                           |
| Driver de BD         | pgx/v5 (`jackc/pgx`)                       | 5.5.5   | Driver PostgreSQL nativo de alto rendimiento                  |
| Autenticación        | golang-jwt/jwt/v5                          | 5.2.1   | Generación y validación de tokens JWT                         |
| Hash de contraseñas  | golang.org/x/crypto (bcrypt)               | 0.24.0  | Cifrado de contraseñas de login local                         |
| Tareas programadas   | robfig/cron/v3                             | 3.0.1   | Job diario de revisión de vencimientos                        |
| OAuth / Google APIs  | golang.org/x/oauth2, google.golang.org/api | —       | Login con Google, Drive API                                   |
| CORS                 | gin-contrib/cors                           | 1.7.2   | Control de orígenes permitidos                                |
| Variables de entorno | joho/godotenv                              | 1.5.1   | Carga de `.env` en desarrollo local                           |

### Frontend

| Componente       | Tecnología        | Versión             | Propósito                                        |
| ---------------- | ----------------- | ------------------- | ------------------------------------------------ |
| Framework        | Next.js           | 14.2.5 (App Router) | SSR/SSG, routing basado en archivos              |
| Librería UI      | React             | 18.3.1              | Componentes                                      |
| Lenguaje         | TypeScript        | 5.5.4               | Tipado estático                                  |
| Componentes UI   | MUI (Material UI) | 5.16.7              | Sistema de diseño, iconos, DataGrid, DatePickers |
| Gráficos         | Recharts          | 2.12.7              | Gráficos del dashboard ejecutivo                 |
| Manejo de fechas | date-fns          | 3.6.0               | Formateo y cálculo de fechas                     |

### Infraestructura y servicios externos

| Servicio                     | Rol en el proyecto                                                                       |
| ---------------------------- | ---------------------------------------------------------------------------------------- |
| **GitHub**                   | Control de versiones — 2 repositorios independientes (`sigpa-backend`, `sigpa-frontend`) |
| **Render**                   | Hosting del backend (Web Service con Docker)                                             |
| **Vercel**                   | Hosting del frontend (build y despliegue automático de Next.js)                          |
| **Supabase**                 | Base de datos PostgreSQL administrada (con pooler de conexiones)                         |
| **Google Cloud / Drive API** | Almacenamiento de documentos (PDFs/imágenes) y autenticación OAuth                       |
| **Brevo** (antes Sendinblue) | Envío de correos transaccionales de alertas vía API HTTP                                 |

> **Nota de arquitectura:** se eligió Brevo sobre SMTP tradicional porque plataformas cloud gratuitas como Render bloquean por defecto las conexiones salientes al puerto 587 (anti-spam). La API de Brevo usa HTTPS (puerto 443), que nunca está bloqueado.

---

## 3. Arquitectura del Backend

El backend sigue una variante de **Clean Architecture** organizada en capas, con separación estricta de responsabilidades:

```
sigpa-backend/
├── cmd/api/main.go          → Punto de entrada: wiring de dependencias
├── internal/
│   ├── config/              → Carga de variables de entorno
│   ├── database/            → Conexión a PostgreSQL (pool de conexiones)
│   ├── models/               → Structs de dominio (Vehiculo, Documento, Alerta...)
│   ├── repository/           → Acceso a datos (SQL puro, sin lógica de negocio)
│   ├── service/               → Lógica de negocio (validaciones, reglas, orquestación)
│   ├── handler/                → Controladores HTTP (parseo de requests, respuestas)
│   ├── middleware/              → Autenticación JWT, CORS, RBAC
│   └── router/                   → Definición de rutas y aplicación de middlewares
└── pkg/
    ├── apperrors/             → Errores tipados reutilizables (ErrNotFound, etc.)
    └── response/               → Formato estándar de respuestas JSON
```

### Flujo de una petición típica (ejemplo: crear un vehículo)

```
Cliente HTTP
   │  POST /api/v1/vehiculos
   ▼
router.go          → aplica middleware.AuthRequired + middleware.RequireRoles("Administrador")
   ▼
vehiculo_handler.go → parsea el JSON del body, valida estructura
   ▼
vehiculo_service.go → aplica reglas de negocio (validar placa única, etc.)
   ▼
vehiculo_repository.go → ejecuta el INSERT en PostgreSQL
   ▼
response.Success(c, ...) → responde con formato JSON estándar {success, data, meta}
```

### Middleware clave

- **`middleware.AuthRequired(jwtSecret)`**: valida el token JWT del header `Authorization: Bearer <token>`, inyecta `organization_id` y `user_id` en el contexto de Gin.
- **`middleware.RequireRoles(...roles)`**: verifica que el rol del usuario autenticado esté en la lista permitida (RBAC).
- **`middleware.CORS(origins)`**: solo permite peticiones desde los orígenes configurados en `CORS_ALLOWED_ORIGINS`.

### Multi-tenancy

Todas las tablas de negocio incluyen la columna `organization_id`. El middleware de autenticación resuelve automáticamente la organización del usuario autenticado (según el dominio de su correo institucional contra la tabla `organizaciones`), y cada repositorio filtra siempre por ese `organization_id` — esto permite que un mismo backend sirva a múltiples Alcaldías sin mezclar datos entre ellas.

---

## 4. Arquitectura del Frontend

```
sigpa-frontend/
├── app/
│   ├── layout.tsx                 → Layout raíz (ThemeRegistry, providers globales)
│   ├── page.tsx                    → Redirección inicial
│   ├── login/                       → Pantalla de login (Google OAuth + credenciales locales)
│   └── (app)/                        → Grupo de rutas protegidas (requieren sesión)
│       ├── dashboard/                  → Dashboard ejecutivo
│       ├── vehiculos/                   → Listado y detalle de vehículos
│       ├── documentos/                   → Gestión documental global
│       ├── alertas/                       → Centro de alertas
│       ├── programaciones/                 → Programación diaria de vehículos
│       ├── reportes/                        → Exportables
│       └── admin/                            → Gestión de usuarios y dependencias
├── components/
│   ├── layout/                       → AppShell, Sidebar, Topbar (estructura visual)
│   ├── dashboard/                     → KpiCard, gráficos Recharts
│   ├── vehiculos/                      → Diálogos de formulario, subida de documentos
│   ├── admin/                           → Diálogos de administración
│   └── ui/                               → Componentes atómicos (StatusBadge, etc.)
├── contexts/
│   ├── auth-context.tsx              → Sesión del usuario, login/logout
│   ├── tema-context.tsx               → Branding dinámico por Alcaldía (multi-tenant)
│   └── modo-context.tsx                → Modo claro/oscuro
├── lib/
│   ├── api-client.ts                  → Cliente HTTP centralizado (fetch + manejo de errores)
│   ├── services.ts                     → Funciones que llaman a cada endpoint del backend
│   └── hooks/                            → Hooks personalizados (useCatalogos, useConteos)
└── types/
    └── index.ts                          → Interfaces TypeScript compartidas
```

### Capa de comunicación con el backend

Todo el frontend pasa obligatoriamente por `lib/api-client.ts`, que centraliza:

- Inyección automática del token JWT en cada petición (`Authorization: Bearer`)
- Manejo uniforme de errores (`ApiError`)
- Parseo de la envoltura estándar de respuesta `{ success, data, meta }`

Los componentes de página **nunca** hacen `fetch` directo — siempre usan las funciones tipadas de `lib/services.ts` (ej. `vehiculosService.listar()`, `documentosService.subir()`), lo que facilita el mantenimiento y la detección de errores en tiempo de compilación.

### Sistema de temas multi-tenant

`tema-context.tsx` consulta `/api/v1/public/tema?dominio=...` **antes** de que el usuario inicie sesión, para pintar el logo y los colores institucionales correctos según el dominio de correo de la Alcaldía. Esto permite que el mismo código de frontend sirva visualmente distinto a cada Alcaldía sin necesidad de despliegues separados.

---

## 5. Modelo de datos

### Tablas principales

| Tabla                                             | Propósito                                                                                                         |
| ------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------- |
| `organizaciones`                                  | Cada Alcaldía (tenant) — nombre, dominio de correo, estado activo                                                 |
| `usuarios`                                        | Cuentas de acceso — vinculadas a Google Workspace o credenciales locales                                          |
| `roles`                                           | Catálogo: Administrador, Dependencia, Consulta, Gerencia                                                          |
| `dependencias`                                    | Secretarías/dependencias de la Alcaldía                                                                           |
| `vehiculos`                                       | Datos generales del vehículo (placa, marca, tipo, estado, dependencia, responsable)                               |
| `tipos_vehiculo`, `estados_vehiculo`              | Catálogos configurables                                                                                           |
| `documentos`                                      | Cada versión de cada documento por vehículo (SOAT, Tecnomecánica, etc.) con fecha de vencimiento y enlace a Drive |
| `tipos_documento`                                 | Catálogo de tipos de documento obligatorios                                                                       |
| `alertas`                                         | Notificaciones generadas automáticamente por el job diario                                                        |
| `config_alertas`                                  | Reglas configurables (cuántos días antes de vencer, qué nivel de severidad)                                       |
| `historial_cambios`                               | Auditoría de cambios sobre cualquier entidad (quién, qué, cuándo, valor anterior/nuevo)                           |
| `programaciones_vehiculos` / `programacion_items` | Programación diaria de uso de vehículos                                                                           |
| `listas_configurables`                            | Listas editables por el Administrador (conductores, actividades, etc.)                                            |

### Relaciones clave

```
organizaciones 1───N usuarios
organizaciones 1───N vehiculos
vehiculos      1───N documentos
vehiculos      N───1 dependencias
vehiculos      N───1 responsable (usuarios)
documentos     1───N alertas
usuarios       N───1 roles
```

### Borrado lógico

Los documentos **no se eliminan físicamente**: el endpoint DELETE marca el registro con `eliminado = TRUE`, `eliminado_por` y `fecha_eliminacion`, conservando el archivo en Google Drive y el registro en base de datos para trazabilidad y auditoría. Solo el rol Administrador puede ejecutar esta acción.

---

## 6. Autenticación y autorización

### Dos métodos de login soportados

1. **Google OAuth 2.0 (Workspace)** — flujo estándar: el frontend obtiene un `id_token` de Google Sign-In y lo envía a `POST /api/v1/auth/google`. El backend valida el token con Google, extrae el dominio del correo, busca la organización correspondiente en la tabla `organizaciones`, crea o actualiza el usuario, y devuelve un JWT propio del sistema.

2. **Credenciales locales** — para usuarios sin cuenta de Google Workspace (ej. proveedores externos, cuentas de servicio). El backend valida `email` + `password` contra el hash bcrypt guardado en `usuarios.password_hash`, vía `POST /api/v1/auth/local`.

### Estructura del JWT emitido

```json
{
  "user_id": 42,
  "organization_id": 1,
  "rol_nombre": "Administrador",
  "exp": 1735689600
}
```

El frontend guarda este token en `localStorage` y lo envía en cada petición mediante el header `Authorization: Bearer <token>`.

### Matriz de roles y permisos

| Acción                                   | Administrador | Dependencia           | Consulta | Gerencia |
| ---------------------------------------- | ------------- | --------------------- | -------- | -------- |
| Ver dashboard y reportes                 | ✅            | ✅                    | ✅       | ✅       |
| Crear/editar vehículos                   | ✅            | ✅ (solo los propios) | ❌       | ❌       |
| Eliminar vehículos                       | ✅            | ❌                    | ❌       | ❌       |
| Subir/renovar documentos                 | ✅            | ✅                    | ❌       | ❌       |
| Eliminar documentos (lógico)             | ✅            | ❌                    | ❌       | ❌       |
| Gestionar usuarios/roles                 | ✅            | ❌                    | ❌       | ❌       |
| Configurar reglas de alertas             | ✅            | ❌                    | ❌       | ❌       |
| Ejecutar revisión manual de vencimientos | ✅            | ❌                    | ❌       | ❌       |

La validación de roles ocurre en el **backend** (middleware `RequireRoles`), nunca solo en el frontend — el frontend oculta botones por UX, pero la seguridad real está en la API.

---

## 7. Integraciones externas

### 7.1 Google Drive (almacenamiento documental)

- Se usa una **cuenta de servicio** (Service Account) de Google Cloud, sin necesidad de "domain-wide delegation" (que requeriría acceso a `admin.google.com`).
- La cuenta de servicio debe ser miembro de una **Unidad Compartida** de Drive con rol "Administrador de contenido".
- Estructura de carpetas generada automáticamente: `[Carpeta raíz] / [Tipo de documento] / [Placa del vehículo] / archivo.pdf`
- El backend expone la interfaz `DriveUploader` (`internal/service/drive_service.go`), con métodos para crear carpetas y subir archivos — si no hay credenciales configuradas, se usa un _stub_ que solo loguea en consola (útil en desarrollo local sin credenciales reales).

### 7.2 Brevo (envío de correos transaccionales)

- Implementado en `internal/service/brevo_service.go`, satisface la misma interfaz `NotificadorEmail` que la alternativa SMTP.
- Usa la API HTTP `POST https://api.brevo.com/v3/smtp/email` con la cabecera `api-key`.
- El sistema prioriza Brevo sobre SMTP automáticamente si la variable `BREVO_API_KEY` está configurada (ver `cmd/api/main.go`).
- Las plantillas de correo combinan un **wrapper HTML institucional** (header rojo Funza + card blanca) con el contenido dinámico (texto editado por el usuario o tabla de datos generada automáticamente).

### 7.3 Google OAuth 2.0 (login)

- Requiere credenciales OAuth 2.0 (`Client ID` / `Client Secret`) creadas en Google Cloud Console.
- El frontend usa el Client ID para el botón de "Iniciar sesión con Google"; el backend valida el `id_token` recibido contra los servidores de Google.

---

## 8. Sistema de alertas y vencimientos

### Cálculo del estado documental

Cada documento tiene un campo `estado_documento` calculado según la función `models.CalcularEstado`:

```go
switch {
case diasRestantes < 0:
    return EstadoVencido
case diasRestantes <= diasVentana: // 45 días por defecto
    return EstadoProximoAVencer
default:
    return EstadoVigente
}
```

> **Importante:** este campo se persiste en base de datos y solo se recalcula cuando corre el job diario o el botón manual "Ejecutar revisión ahora". El frontend, en las vistas de detalle, **recalcula el estado visual en tiempo real** a partir de la fecha de vencimiento real, para evitar inconsistencias visuales entre el badge y el texto de días restantes.

### Niveles de alerta configurables

| Nivel       | Días antes del vencimiento (por defecto) |
| ----------- | ---------------------------------------- |
| Preventiva  | 45                                       |
| Importante  | 30                                       |
| Prioritaria | 15                                       |
| Urgente     | 7                                        |
| Crítica     | 1 / vencido                              |

Estas reglas se administran desde **Centro de Alertas → Reglas de Notificación** (solo Administrador), y se guardan en la tabla `config_alertas`.

### Job diario (cron)

`internal/service/alerta_service.go → EjecutarRevisionDiaria()`:

1. Recalcula el `estado_documento` de todos los documentos vigentes
2. Recorre los documentos y evalúa cada regla de `config_alertas` activa
3. Si corresponde generar una alerta y no existe ya una del mismo nivel para ese documento en el día, la crea en la tabla `alertas`
4. Envía el correo correspondiente vía Brevo/SMTP

Se ejecuta automáticamente todos los días a las 6:00 AM (`ALERTAS_CRON_SPEC=0 6 * * *`), y también puede dispararse manualmente desde la UI (botón "Ejecutar revisión ahora", solo Administrador) — útil para pruebas sin esperar al día siguiente.

---

# PARTE B — Guía de Despliegue Paso a Paso

## 9. Requisitos previos

Antes de comenzar, crea o ten a mano cuentas activas en los siguientes servicios (todos tienen plan gratuito suficiente para este proyecto):

| Servicio             | URL                      | Para qué                        |
| -------------------- | ------------------------ | ------------------------------- |
| GitHub               | github.com               | Alojar el código fuente         |
| Render               | render.com               | Desplegar el backend (Go)       |
| Vercel               | vercel.com               | Desplegar el frontend (Next.js) |
| Supabase             | supabase.com             | Base de datos PostgreSQL        |
| Brevo                | brevo.com                | Envío de correos de alertas     |
| Google Cloud Console | console.cloud.google.com | Drive API + OAuth 2.0           |

También necesitas instalado localmente: **Git**, **Go 1.21+** (opcional, para pruebas locales), **Node.js 18+** (opcional, para pruebas locales del frontend).

---

## 10. Repositorios en GitHub

El proyecto se organiza en **dos repositorios independientes** — esto permite desplegar y escalar backend y frontend de forma completamente separada.

### 10.1 Crear los repositorios

1. Ve a [github.com/new](https://github.com/new)
2. Crea el repositorio **`sigpa-backend`** (privado o público, según la política de la Alcaldía)
3. Repite el proceso para **`sigpa-frontend`**

### 10.2 Subir el código por primera vez

Desde la carpeta `sigpa-backend`:

```bash
git init
git add -A
git commit -m "Initial commit: SIGPA backend"
git branch -M main
git remote add origin https://github.com/TU_USUARIO/sigpa-backend.git
git push -u origin main
```

Repite lo mismo desde la carpeta `sigpa-frontend`, apuntando al repositorio `sigpa-frontend`.

### 10.3 Convención de trabajo usada en este proyecto

- Rama única `main` en producción (sin ramas de feature separadas para este proyecto, dado su tamaño)
- Cada cambio funcional se sube con un commit descriptivo en formato `tipo: descripción` (ej. `feat: soft-delete for documents`, `fix: consistent real-time expiration status`)
- Cada `git push` a `main` dispara automáticamente el redeploy en Render (backend) y Vercel (frontend) — no se requiere ninguna acción manual adicional después del push

### 10.4 Archivos `.gitignore` importantes

Verifica que ambos repos ignoren archivos sensibles:

**`sigpa-backend/.gitignore`** debe incluir:

```
.env
credentials/*.json
```

**`sigpa-frontend/.gitignore`** debe incluir:

```
.env.local
node_modules/
.next/
```

> ⚠️ **Nunca subas** el archivo de credenciales de la cuenta de servicio de Google (`credentials/*.json`) ni los archivos `.env` con secretos reales al repositorio.

---

## 11. Base de datos — Supabase

### 11.1 Crear el proyecto

1. Ingresa a [supabase.com](https://supabase.com) → **New Project**
2. Elige un nombre (ej. `sigpa-funza`), una contraseña segura para la base de datos, y la región más cercana (ej. `us-east-1`)
3. Espera ~2 minutos a que se aprovisione el proyecto

### 11.2 Obtener las credenciales de conexión

1. Ve a **Project Settings → Database**
2. En la sección **Connection string**, selecciona la pestaña **Connection pooling** (modo `Transaction`, puerto **6543**) — este es el modo recomendado para producción, ya que evita agotar el límite de conexiones simultáneas
3. Copia estos valores, los necesitarás como variables de entorno del backend:
   - `DB_HOST` → algo como `aws-0-us-east-1.pooler.supabase.com`
   - `DB_PORT` → `6543`
   - `DB_USER` → `postgres.xxxxxxxxxxxxxxxxxxxx`
   - `DB_PASSWORD` → la contraseña que definiste al crear el proyecto
   - `DB_NAME` → `postgres`

### 11.3 Ejecutar los scripts SQL

En el panel de Supabase, ve a **SQL Editor → New query**, y ejecuta en este orden los scripts que están en la raíz del repositorio:

1. `supabase_schema_postgresql.sql` — crea todas las tablas, índices y relaciones
2. `supabase_import_1_dependencias.sql` — carga inicial de dependencias de la Alcaldía
3. `supabase_import_2_vehiculos.sql` — carga inicial del parque automotor (migrado desde Excel/SQL Server)
4. `crear_usuario_admin.sql` — crea el primer usuario Administrador (edítalo antes de ejecutar para poner el correo/contraseña reales)
5. Migraciones incrementales según se hayan generado (ej. `supabase_migracion_borrado_logico_documentos.sql`)

> 💡 Ejecuta cada script completo de una sola vez (Ctrl+Enter en el editor SQL de Supabase) y verifica que no arroje errores antes de continuar con el siguiente.

### 11.4 Verificación

Ve a **Table Editor** y confirma que existan las tablas principales: `organizaciones`, `usuarios`, `roles`, `vehiculos`, `documentos`, `alertas`, `historial_cambios`.

---

## 12. Google Cloud — Cuenta de servicio y Drive

### 12.1 Crear el proyecto en Google Cloud Console

1. Ve a [console.cloud.google.com](https://console.cloud.google.com)
2. Crea un nuevo proyecto (ej. `sigpa-funza`)
3. En el buscador superior, busca **"Google Drive API"** y haz clic en **Habilitar**

### 12.2 Crear la cuenta de servicio (Service Account)

1. Ve a **IAM y administración → Cuentas de servicio → Crear cuenta de servicio**
2. Nombre: `sigpa-drive-service` (o el que prefieras)
3. No es necesario asignar roles de proyecto (los permisos se otorgan directamente en Drive)
4. Una vez creada, entra a la cuenta → pestaña **Claves** → **Agregar clave → Crear clave nueva → JSON**
5. Se descargará un archivo `.json` — este es el archivo de credenciales. **Guárdalo de forma segura, no lo subas a GitHub**
6. Copia el campo `client_email` del JSON (algo como `sigpa-drive-service@sigpa-funza.iam.gserviceaccount.com`) — lo necesitarás en el siguiente paso

### 12.3 Crear la Unidad Compartida y otorgar acceso

> Este proyecto usa una **Unidad Compartida** de Drive (Shared Drive) en vez de una carpeta personal, porque las cuentas de servicio no tienen cuota de almacenamiento propia — necesitan pertenecer a una Unidad Compartida para poder escribir archivos.

1. En Google Drive (con una cuenta institucional de Google Workspace), ve a **Unidades compartidas → Nueva unidad compartida**
2. Nómbrala (ej. "SIGPA - Documentos Vehículos")
3. Dentro de la unidad, ve a **Administrar miembros → Agregar miembros**
4. Pega el `client_email` de la cuenta de servicio (paso anterior) y asígnale el rol **"Administrador de contenido"**
5. Copia el **ID de la carpeta raíz** de la Unidad Compartida — está en la URL cuando estás dentro de ella: `https://drive.google.com/drive/folders/`**`ESTE_ES_EL_ID`**

### 12.4 Preparar la credencial para producción (Render no permite subir archivos)

Como Render no tiene un sistema de archivos persistente para subir el JSON directamente, se codifica en Base64 y se pasa como variable de entorno:

```bash
# En PowerShell (Windows):
[Convert]::ToBase64String([IO.File]::ReadAllBytes("ruta\a\tu-credencial.json")) | Set-Clipboard

# En Linux/Mac:
base64 -i tu-credencial.json | pbcopy
```

Este valor Base64 se pega en la variable de entorno `GOOGLE_SERVICE_ACCOUNT_JSON_BASE64` en Render (ver sección 14).

---

## 13. Brevo — Envío de correos transaccionales

### 13.1 Crear la cuenta

1. Ve a [brevo.com](https://www.brevo.com) → **Sign up free**
2. Puedes registrarte con cualquier correo (no tiene que ser el correo institucional que enviará las notificaciones) — el login de Brevo y el remitente de los correos son cosas independientes

### 13.2 Verificar el remitente institucional

1. Dentro de Brevo, ve a **Configuración de la organización → Remitentes, dominio, IP → pestaña Remitentes**
2. Clic en **Agregar un remitente**
   - Nombre: ej. `SIGPA - Patio Alcaldía de Funza`
   - Email: el correo institucional real que aparecerá como remitente (ej. `Patio@funza-cundinamarca.gov.co`)
3. Brevo enviará un correo de verificación a esa dirección — alguien con acceso a esa bandeja debe abrirlo y confirmar el enlace

### 13.3 Generar la API Key

1. Ve a **Configuración de la organización → SMTP y API → pestaña API Keys**
2. Clic en **Generar una nueva clave API**, ponle un nombre (ej. `SIGPA-Backend`)
3. Copia la clave generada (formato `xkeysib-...`) — la necesitarás en la variable de entorno `BREVO_API_KEY` de Render

> 💡 El plan gratuito de Brevo permite 300 correos/día, más que suficiente para las alertas diarias de una Alcaldía.

---

## 14. Backend — Despliegue en Render

### 14.1 Crear el Web Service

1. Ve a [render.com](https://render.com) → **New → Web Service**
2. Conecta tu cuenta de GitHub y selecciona el repositorio `sigpa-backend`
3. Configuración:
   - **Name**: `sigpa-backend`
   - **Region**: la más cercana a tu base de datos de Supabase
   - **Branch**: `main`
   - **Runtime**: **Docker** (Render detecta automáticamente el `Dockerfile` en la raíz del repo)
   - **Plan**: Free (suficiente para producción de una sola Alcaldía)

### 14.2 Variables de entorno completas

En la pestaña **Environment**, agrega estas variables (según `internal/config/config.go`):

| Variable                             | Ejemplo / Valor                                                  | Origen                                      |
| ------------------------------------ | ---------------------------------------------------------------- | ------------------------------------------- |
| `APP_PORT`                           | `8080`                                                           | Fijo                                        |
| `DB_HOST`                            | `aws-0-us-east-1.pooler.supabase.com`                            | Supabase (sección 11.2)                     |
| `DB_PORT`                            | `6543`                                                           | Supabase                                    |
| `DB_USER`                            | `postgres.xxxxxxxxxxxxxxxxxxxx`                                  | Supabase                                    |
| `DB_PASSWORD`                        | tu contraseña de Supabase                                        | Supabase                                    |
| `DB_NAME`                            | `postgres`                                                       | Supabase                                    |
| `GOOGLE_SERVICE_ACCOUNT_JSON_BASE64` | (cadena Base64 larga)                                            | Sección 12.4                                |
| `GOOGLE_DRIVE_ROOT_FOLDER_ID`        | ID de la carpeta/unidad compartida                               | Sección 12.3                                |
| `JWT_SECRET`                         | cadena aleatoria larga (ej. generada con `openssl rand -hex 32`) | Generar una vez, no cambiar después         |
| `BREVO_API_KEY`                      | `xkeysib-...`                                                    | Brevo (sección 13.3)                        |
| `BREVO_REMITENTE_EMAIL`              | `Patio@funza-cundinamarca.gov.co`                                | El remitente verificado en Brevo            |
| `BREVO_REMITENTE_NOMBRE`             | `SIGPA - Patio Alcaldía de Funza`                                | Nombre visible del remitente                |
| `ALERTAS_CRON_SPEC`                  | `0 6 * * *`                                                      | Hora de ejecución diaria del job (6:00 AM)  |
| `CORS_ALLOWED_ORIGINS`               | `https://sigpa-frontend.vercel.app`                              | URL de producción del frontend (sección 15) |
| `GOOGLE_CLIENT_ID`                   | `xxxx.apps.googleusercontent.com`                                | Google Cloud Console (sección 16)           |
| `GOOGLE_CLIENT_SECRET`               | (secreto de OAuth)                                               | Google Cloud Console                        |
| `GOOGLE_REDIRECT_URL`                | URL de callback OAuth                                            | Google Cloud Console                        |

> ⚠️ Las variables `SMTP_*` y `GMAIL_*` de `.env.example` quedaron como alternativas de respaldo — **no son necesarias** si ya configuraste Brevo, que es el método usado en producción.

### 14.3 Desplegar

1. Clic en **Create Web Service** — Render clonará el repo, construirá la imagen Docker y la desplegará automáticamente
2. Espera a que el estado pase a **"Live"** (usualmente 2-4 minutos)
3. Copia la URL pública asignada, algo como `https://sigpa-backend.onrender.com`

### 14.4 Verificar que el backend responde

```bash
curl https://sigpa-backend.onrender.com/health
```

Debe responder `{"status":"ok"}`.

### 14.5 Redeploys posteriores

Cada `git push` a la rama `main` del repositorio `sigpa-backend` dispara automáticamente un nuevo build y despliegue en Render — no se requiere ninguna acción manual. Puedes ver el progreso y los logs en la pestaña **Logs** del servicio en Render.

---

## 15. Frontend — Despliegue en Vercel

### 15.1 Conectar el repositorio

1. Ve a [vercel.com](https://vercel.com) → **Add New → Project**
2. Conecta tu cuenta de GitHub y selecciona el repositorio `sigpa-frontend`
3. Vercel detecta automáticamente que es un proyecto **Next.js** — no requiere configuración adicional de build

### 15.2 Variables de entorno

En **Settings → Environment Variables**, agrega (según `.env.example`):

| Variable                       | Valor                                       | Origen                               |
| ------------------------------ | ------------------------------------------- | ------------------------------------ |
| `NEXT_PUBLIC_API_URL`          | `https://sigpa-backend.onrender.com/api/v1` | URL del backend (sección 14.3)       |
| `NEXT_PUBLIC_GOOGLE_CLIENT_ID` | `xxxx.apps.googleusercontent.com`           | Google Cloud Console (sección 16)    |
| `NEXT_PUBLIC_TENANT_DOMAIN`    | `funza-cundinamarca.gov.co`                 | Dominio institucional de la Alcaldía |

Aplica estas variables a los entornos **Production**, **Preview** y **Development** según corresponda.

### 15.3 Desplegar

1. Clic en **Deploy** — Vercel construye el proyecto y lo publica automáticamente
2. Al finalizar, Vercel asigna:
   - Una **URL de producción estable**: `https://sigpa-frontend.vercel.app` (esta es la que se debe compartir siempre con los usuarios finales)
   - Una URL específica por cada deployment individual (con hash aleatorio) — **esta cambia en cada deploy y nunca debe usarse como enlace permanente**

> ⚠️ **Importante:** solo la URL de dominio de producción (`https://sigpa-frontend.vercel.app` o el dominio personalizado que configures) se actualiza automáticamente con cada nuevo despliegue. Las URLs de deployment individuales quedan "congeladas" en la versión de ese momento — si comparte una de esas por error, el usuario seguirá viendo una versión antigua del sistema aunque hagas nuevos cambios.

### 15.4 Actualizar CORS en el backend

Una vez tengas la URL definitiva de Vercel, regresa a Render (sección 14.2) y actualiza la variable `CORS_ALLOWED_ORIGINS` con esa URL exacta, para que el backend acepte peticiones del frontend.

### 15.5 Redeploys posteriores

Igual que con Render: cada `git push` a `main` del repositorio `sigpa-frontend` dispara un nuevo build y despliegue automático en Vercel.

---

## 16. Google OAuth — Login con Workspace

### 16.1 Configurar la pantalla de consentimiento OAuth

1. En Google Cloud Console (mismo proyecto de la sección 12), ve a **APIs y servicios → Pantalla de consentimiento de OAuth**
2. Tipo de usuario: **Interno** (si la Alcaldía usa Google Workspace) o **Externo** si se requiere acceso más amplio
3. Completa nombre de la app, correo de soporte, dominios autorizados

### 16.2 Crear las credenciales OAuth 2.0

1. Ve a **APIs y servicios → Credenciales → Crear credenciales → ID de cliente de OAuth**
2. Tipo de aplicación: **Aplicación web**
3. **Orígenes de JavaScript autorizados**: agrega la URL de producción del frontend, ej. `https://sigpa-frontend.vercel.app`
4. **URIs de redirección autorizados**: agrega la URL de callback del backend, ej. `https://sigpa-backend.onrender.com/api/v1/auth/google/callback`
5. Al guardar, copia el **Client ID** y el **Client Secret**

### 16.3 Configurar las variables de entorno

- En **Render** (backend): `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `GOOGLE_REDIRECT_URL`
- En **Vercel** (frontend): `NEXT_PUBLIC_GOOGLE_CLIENT_ID` (solo el Client ID, nunca el secreto en el frontend)

---

## 17. Checklist final de verificación

Una vez completados todos los pasos anteriores, verifica end-to-end:

- [ ] `curl https://TU-BACKEND.onrender.com/health` responde `{"status":"ok"}`
- [ ] El frontend carga correctamente en `https://TU-FRONTEND.vercel.app` con el logo y colores de la Alcaldía
- [ ] Puedes iniciar sesión con Google Workspace (o con las credenciales locales del Administrador)
- [ ] Puedes crear un vehículo de prueba
- [ ] Puedes subir un documento (PDF) a un vehículo y ver que aparece el enlace de Google Drive
- [ ] El archivo subido es visible en la Unidad Compartida de Drive
- [ ] Puedes ejecutar "Ejecutar revisión ahora" en Centro de Alertas (como Administrador) sin errores
- [ ] Si hay documentos próximos a vencer o vencidos, se generan alertas visibles
- [ ] Puedes enviar una notificación de prueba desde el botón "Notificar" de un documento, y el correo llega correctamente
- [ ] El dashboard ejecutivo muestra los KPIs con datos reales (no en cero, si ya hay vehículos cargados)

---

## 18. Troubleshooting — problemas conocidos y soluciones

Esta sección documenta problemas reales encontrados durante el desarrollo y despliegue de SIGPA, como referencia para resolución rápida.

### 18.1 Error de CORS al llamar al backend desde el frontend

**Síntoma:** la consola del navegador muestra `blocked by CORS policy`.
**Causa:** la variable `CORS_ALLOWED_ORIGINS` en Render no coincide exactamente con la URL del frontend.
**Solución:** verificar que el valor en Render sea idéntico (sin `/` al final) a la URL real de Vercel, y hacer un manual redeploy tras el cambio.

### 18.2 Los correos de alerta nunca llegan (timeout de ~2 minutos)

**Síntoma:** el envío de notificaciones se queda "cargando" por 2 minutos y luego falla.
**Causa:** Render (y otras plataformas cloud gratuitas) bloquean por defecto las conexiones salientes al puerto 587 (SMTP), por políticas anti-spam.
**Solución:** usar Brevo (API HTTP sobre el puerto 443, nunca bloqueado) en vez de SMTP — ya implementado en `brevo_service.go`. Verificar que `BREVO_API_KEY` esté configurada en Render.

### 18.3 Compartí un enlace de la app y el usuario ve una versión vieja

**Causa:** se compartió una URL de **deployment individual** de Vercel (con hash aleatorio), en vez de la URL de dominio de producción.
**Solución:** usar siempre `https://sigpa-frontend.vercel.app` (o el dominio personalizado configurado) para compartir con usuarios — esa URL sí se actualiza automáticamente con cada nuevo deploy.

### 18.4 La tarjeta de un documento muestra "Próximo a vencer" pero el texto dice "Venció hace X días"

**Causa:** el campo `estado_documento` se persiste en base de datos y solo se recalcula cuando corre el job diario o el botón manual — puede quedar desactualizado si ha pasado tiempo desde la última ejecución.
**Solución:** el frontend ya recalcula el estado visual en tiempo real a partir de la fecha real de vencimiento (independiente del campo cacheado), por lo que este bug ya no debería reproducirse en versiones actuales del sistema. Si reaparece, ejecutar "Ejecutar revisión ahora" para sincronizar el campo en base de datos.

### 18.5 El correo de notificación llega con una plantilla distinta a la que edité

**Causa:** el frontend no enviaba el asunto/cuerpo editado al backend; el backend generaba siempre su plantilla fija por defecto.
**Solución:** ya corregido — el backend ahora respeta el texto personalizado enviado desde el diálogo de notificación, envolviéndolo con el mismo estilo visual institucional (header rojo + card blanca) que la plantilla automática.

### 18.6 Búsqueda por placa desde el topbar no funciona si ya estás en el módulo de Vehículos

**Causa:** Next.js no remonta el componente al navegar a la misma ruta con distintos query params; el estado inicial de React nunca se actualizaba.
**Solución:** se agregó un `useEffect` que sincroniza el filtro de placa cada vez que cambia el parámetro de la URL, sin depender del montaje inicial del componente.

### 18.7 Bloqueo por rol al intentar una acción (403 Forbidden)

**Causa:** el usuario autenticado no tiene el rol requerido para la acción (ej. Dependencia intentando eliminar un documento).
**Solución:** verificar la matriz de roles (sección 6) — solo Administrador puede eliminar documentos/vehículos, gestionar usuarios y configurar alertas.
