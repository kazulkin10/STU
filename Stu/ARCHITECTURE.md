# Архитектура Stu (уровень 0-1)

## Сервисы

- `api-gateway` — внешний HTTP/JSON API, прокси в gRPC/внутренние сервисы, выдача presigned URL, аутентификация и авторизация, rate limit.
- `auth` — регистрация/вход/2FA, управление устройствами/сессиями, refresh rotation, хранение публичных ключей устройств, аудит.
- `realtime` — WebSocket/long-poll хабы, presence/typing/read receipts, доставка событий new_message/message_edit/delete/call_signal, очереди (Redis).
- `media` — presigned URL к MinIO, приём метаданных/антивирус/превью, контроль TTL и лимитов.
- `admin` — админ-панель, жалобы/решения/апелляции, статистика, управление ролями/каналами.
- `moderation-agent` (Python) — классификация контента из report packet (только по жалобам), отдаёт вердикт в очередь.
- Хранилища: PostgreSQL (данные), Redis (кэш/очереди/presence), MinIO (объекты), Prometheus/Loki/Jaeger (метрики/логи/трейсы), nginx (edge/TLS).

## Данные и модели (базово)

- Пользователь: email, password_hash, username, настройки приватности, флаги 2FA.
- Устройства/сессии: device_id, platform, last_seen, access/refresh token hashes, аудит (ip/ua).
- Диалоги: тип (direct/group/channel), owner, encrypted (bool), настройки доступа.
- Сообщения: sender, cipher_text, reply_to, metadata, реакции, прочтения.
- Ключи: identity key, signed prekey, one-time prekeys (только публичные части), sender keys для групп (будет в crypto pkg).
- Медиа: object_key, mime, size, bucket, ttl.
- Репорты: reporter, target, encrypted_blob (report packet), статус, решение.

## Потоки (см. также docs/02_dataflows.md)

- Регистрация: email → код/ссылка (MailerSend) → подтверждение → загрузка публичных ключей → выдача device-bound сессии.
- Сообщение (1:1 E2EE): клиент шифрует (Double Ratchet) → api-gateway → realtime → Redis stream → доставляется WS + сохраняются метаданные.
- Медиа: presigned URL → загрузка в MinIO → запись метаданных → выдача подписанных ссылок для чтения.
- Репорт: клиент формирует report packet (ключ модерации) → admin → moderation-agent → решение/действие → аудит/уведомления.
- Звонки: сигнальные сообщения через realtime (call_signal), медиапоток WebRTC (SRTP, STUN/TURN).

## Протокол и крипта (задел)

- X3DH + Double Ratchet (Noise-подобный), prekeys и signed prekeys, Ed25519 подписи, ECDH X25519, AEAD ChaCha20-Poly1305 или AES-GCM.
- Проверка ключей через safety number/QR, хранение ключей у клиента (пароль/OS keystore), сервер видит только публичные ключи.
- Sender keys для групп, приватный режим каналов (контент шифруется ключом канала).

## Транспорт и безопасность

- TLS 1.2+, HSTS, CORS/CSRF, secure cookies/opaque токены с device binding, refresh rotation, rate limiting.
- WS с токеном доступа, request-id, структурные логи (zerolog), метрики Prometheus (`/metrics` на отдельном порту).
- Минимизация метаданных: TTL логов, раздельное хранение ключей/метаданных, audit trail с ограниченным доступом.

## Развёртывание

- Dev: `make dev` → docker compose (Postgres, Redis, MinIO, Mailpit, Go сервисы, Python агент).
- Prod: `make prod` (docker-compose.prod) или systemd (`deploy/systemd/stu@.service`), nginx как reverse proxy, .env в `/etc/stu/.env`.

## Ограничения текущего состояния

- E2EE протокол и группы/каналы/вызовы пока в виде каркаса.
- Нет UI/админки/Swagger (будут добавлены после реализации API).
- Email отправка/2FA, refresh rotation, управление сессиями — в разработке.
