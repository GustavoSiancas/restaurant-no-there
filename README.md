# Backend

API modular en Go y PostgreSQL para usuarios, turnos y alimentación.

## Ejecución

```bash
docker compose up -d
go run ./cmd/api
```

Base API: `http://localhost:8080/api/v1`. Swagger: `http://localhost:8080/swagger/index.html`.

La configuración se carga desde `.env`. El servidor utiliza siempre la hora real y evalúa las reglas operativas en `America/Lima`.

## Autenticación

- `WORKER` inicia sesión únicamente con DNI y recibe un access token temporal sin refresh token.
- `ADMIN`, `OWNER`, `RRHH` y `COLLABORATOR` usan usuario y contraseña.
- Los refresh tokens administrativos se guardan como hash, se rotan de forma atómica y tienen uso único.

## API utilizada por el frontend

- Autenticación: login por DNI, login por contraseña y refresh.
- Usuarios: usuario actual, registros y listados por rol.
- Turnos: creación, actualización, eliminación, rangos propios o por trabajador y estado actual.
- Comidas: horarios, vista previa, confirmación de impresión, pedidos y validación.
- Reportes: reporte y Excel de comidas; vista previa y Excel de turnos.

El contrato completo y los permisos por rol están definidos en `docs/openapi.yaml`.

## WebSocket

Las rutas operativas de este rol usan el prefijo `/api/v1/collaborator`. `WS /api/v1/collaborator/ws/meal-orders` recibe un evento `CLAIMED_ORDERS` con los pedidos pendientes al conectarse y cada vez que cambia la lista, además de los eventos de creación y validación. Los navegadores deben enviar los subprotocolos `["bearer", "JWT"]`; el token no se acepta en la URL.

## Migraciones

`migrations/000001_initial_schema.up.sql` contiene directamente el esquema vigente. Las instalaciones nuevas se crean en el estado final y las bases existentes que ya registraron la versión 1 no se modifican.
