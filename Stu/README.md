# Stu — приватный мессенджер

Домашний, безопасный и быстрый мессенджер уровня Telegram по UX, с криптографией уровня Signal. Монорепозиторий на Go (сервер, протокол, крипта) + Python (модерация по жалобам). Клиент Web/PWA + desktop (через общую Go-библиотеку/WASM).

## Быстрый старт (dev)

```bash
# 1) Скопируйте .env.example и при необходимости правьте порты/ключи
cp .env.example .env

# 2) Запустите окружение
make dev

# После старта доступны:
# - API gateway: http://localhost:8080/healthz
# - Auth: http://localhost:8081/healthz
# - Realtime: http://localhost:8082/healthz
# - Media: http://localhost:8083/healthz
# - Admin: http://localhost:8084/healthz
# - Moderation agent (Python): http://localhost:8085/healthz
# - MinIO console: http://localhost:9001
# - Mailpit (песочница почты): http://localhost:8025
```

### Проверка auth-флоу (Mailpit)
1. `POST http://localhost:8081/v1/auth/register` с `{ "email": "you@example.com", "password": "P@ssw0rd" }`
2. Откройте Mailpit (`http://localhost:8025`), возьмите код из письма.
3. `POST http://localhost:8081/v1/auth/verify` с `{ "email": "...", "code": "XXXXXX", "device_name": "pc", "platform": "win" }` → получите access/refresh.
4. `POST /v1/auth/refresh` с refresh → новые токены, reuse старого refresh вызовет revoke.
5. `POST /v1/auth/logout` или `/logout_all` с refresh для ревокации.

## Команды

- `make build` — собрать Go сервисы.
- `make test` — запустить тесты Go.
- `make lint` — golangci-lint (требует установленного бинаря).
- `make dev` — docker compose для разработки.
- `make down` — остановить dev окружение.

## Структура

- `cmd/*` — исполняемые сервисы (api-gateway, auth, realtime, media, admin).
- `internal/*` — общие пакеты: конфиги, middleware, транспорт, хелперы.
- `pkg/crypto` — общая криптография (X3DH/Double Ratchet) и обёртки для клиентов (WASM/Go mobile) — сейчас каркас.
- `client/web` — PWA/desktop UI (пока каркас).
- `client/shared-go` — общая клиентская Go-библиотека.
- `services/moderation-agent` — Python FastAPI сервис классификации (только по жалобам).
- `deploy/` — docker-compose dev/prod, systemd unit, nginx конфиг, Dockerfile.
- `migrations/` — SQL миграции PostgreSQL.
- `docs/` — ТЗ, поток данных, планы.

## Минимально готовые функции

- Базовые HTTP сервисы с health/ready и метриками (Prometheus handler).
- Auth сервис: регистрация/логин с созданием пользователя, устройства и сессии (opaque токены, bcrypt).
- Moderation-agent: FastAPI заглушка `/analyze` + `/healthz`.
- Миграция БД для пользователей, сессий, диалогов, сообщений, ключей, репортов.
- Логи (zerolog), CORS/безопасные заголовки, request-id, базовый CORS.
- Тесты для токенов/паролей, golangci-lint конфиг, CI workflow.

## Что дальше

См. `docs/01_spec.md` и `docs/02_dataflows.md` для полного списка требований и дорожной карты. Следующие крупные этапы: полнофункциональный auth (email-коды, 2FA), realtime/диалоги, E2EE (X3DH + Double Ratchet), медиа/MinIO, группы/каналы, репорты/админка, звонки.
