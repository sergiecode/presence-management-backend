# BE-ABSTI-CLOCKIN Backend

## ⚠️ Requisito: Configuración de .env

Antes de iniciar (con Docker o Go), **debes** crear un archivo `.env`:

```sh
cp .env.example .env
# Edita .env según tu entorno
```

Ejemplo:

```env
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=absti
PORT=8080
JWT_SECRET=822728e6b99e46e7ba2ab65942602ba11a67c4ed542cef1e71442d2db35ca0b00e77105e9350c94fccfc30bb6010929afbec6a9f7a7dba482462db9bb8be63728d5f1cbf0130c561213b3d342844750b82f7f58feb8ddb1dca6c8f72f187dfe6389122b27bc0d000c57a77653400a64f56072a74ddb7b5d63f8af28d92637b4fbe3728c84c1c46d69953fe2079ce1ed3ae3314980fefb6503363243b18cf53db120524561717df3c50d630e09586d8f4668fd8d81c8bca2af41806520252978e1ecc10dac81e8558c7a40d41b7ddba3e3234d3afb7adec0a799c764ef638866236ac1302af4dbc5597bf2fe5636fe718a15437a99ccaee155c8cca613094ca70
JWT_EXPIRATION=1h
```

---

## 🚨 Usuario Administrador por Defecto

En la primera ejecución, el backend creará automáticamente un usuario administrador por defecto:

- **Email:** `admin@absti.com`
- **Contraseña:** `admin123`

---

## ⚡️ Inicio Rápido para Desarrolladores (Docker Compose)

**¡No necesitas tener Go instalado localmente!**

Solo asegúrate de tener Docker y Docker Compose, luego ejecuta:

```sh
docker-compose up --build
```

- Esto compila y ejecuta el backend de Go y la base de datos Postgres dentro de contenedores.
- Accede a la API en [http://localhost:8080](http://localhost:8080)
- Documentación Swagger: [http://localhost:8080/swagger/index.html](http://localhost:8080/swagger/index.html)

---

## Inicio Manual/Avanzado: Backend + Base de Datos

> **Solo necesario si quieres ejecutar el backend fuera de Docker.**

### Requisitos

- Go 1.20+
- Docker y Docker Compose (para Postgres)
- Archivo `.env` (ver arriba)

### 1. Clonar y Configurar

```sh
git clone <repo-url>
cd BE-ABSTI-CLOCKIN
cp .env.example .env   # Edita según sea necesario
```

### 2. Iniciar la Base de Datos

```sh
docker-compose up -d
# Inicia Postgres según docker-compose.yml
```

### 3. Ejecutar el Backend

```sh
go mod tidy
go run cmd/main.go
# O: go build -o clockin cmd/main.go && ./clockin
```

- Puerto por defecto: `8080` (configurable con la variable `PORT`)
- Documentación Swagger: [http://localhost:8080/swagger/index.html](http://localhost:8080/swagger/index.html)
- Health check: [http://localhost:8080/healthz](http://localhost:8080/healthz)

### 4. Migraciones de Base de Datos

- Las migraciones SQL están en `migrations/`. Ejecútalas con `psql` o una herramienta de migración:

```sh
psql -h localhost -U <user> -d <dbname> -f migrations/20240701_create_daily_checkins_view_and_summary.sql
```

---

## ¿Para Qué Usa el Frontend este Backend?

El backend expone una API REST (con JWT) para:

### 1. Gestión de Usuarios

- Registro, login, logout, refrescar JWT
- Admin: listar, crear, actualizar, desactivar usuarios
- HR/Admin: aprobar usuarios, configurar horario y zona horaria de check-in

### 2. Check-in/Check-out

- Empleados: registrar check-in diario (ubicación, hora, notas, GPS o Ubicaciones Pre definidas, motivo de tardanza)
- Empleados: registrar check-out diario (fin de día, horas extra, estado)
- HR/Admin: ver todos los check-ins, editar/borrar, aprobar en lote

### 3. Gestión de Ausencias

- Empleados: reportar ausencia/tardanza/médica, subir documentos
- HR/Admin: ver todas las ausencias, editar/borrar, bloquear/desbloquear, aprobar en lote

### 4. Dashboard y Analíticas

- HR/Admin: estadísticas de asistencia (% a tiempo, % tarde, horas extra)
- HR/Admin: estadísticas de ausencias, usuarios, logs de auditoría
- HR/Admin: exportar check-ins a Excel, ver resúmenes diarios, pase de lista

### 5. Seguridad

- Autenticación JWT para todos los endpoints protegidos
- Acceso por roles: empleado, HR, admin

---

## Flujos Típicos del Frontend

**App de Empleado:**

- Login → Check-in (con ubicación o no) → Check-out → Ver historial propio/ausencias

**Dashboard HR/Admin:**

- Login → Ver todos los check-ins/ausencias y editarlos si es necesario → Aprobar/rechazar usuarios registrados → Gestionar ausencias (enfermedad, vacaciones, estudio, etc) → Analíticas (asistencia, ausencias, usuarios) → Exportar datos

---

## Referencia de la API

- Consulta `/swagger/index.html` tras iniciar el backend para ver la documentación completa de la API.

---

**TL;DR:**

- Copia y edita `.env` antes de cualquier cosa
- Inicia todo: `docker-compose up --build`
- No necesitas Go a menos que quieras ejecutar el backend fuera de Docker
- Usa `/swagger` para la documentación de la API
- El frontend usa este backend para toda la gestión de usuarios, check-in, ausencias y analíticas del dashboard.
