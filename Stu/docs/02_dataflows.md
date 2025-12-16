# Схема сервисов и потоков данных (текстовая диаграмма)

```
client/web|desktop (Go WASM lib)
   |
   | HTTPS/HTTP2 (REST/gRPC-web) + WebSocket
   v
api-gateway (authz, HTTP/JSON, gRPC proxy)
   |----------- gRPC ------------+
   |                             |
 auth (sessions, devices, email) | realtime (WS hubs, presence)
   |                             |
 PostgreSQL <------> Redis <-----+
   |
 media (presigned URLs, uploads) ----> MinIO (S3)
   |
 admin (web panel, reports, stats)
   |
 moderation-agent (Python, reports only)

Prometheus/Loki/Jaeger для метрик/логов/трассировок.
nginx как edge proxy/TLS/HSTS.
```

## Ключевые потоки

- Регистрация:
  1. Клиент отправляет e-mail → `api-gateway` → `auth`.
  2. `auth` создает запись пользователя (статус pending), шлет код/ссылку через MailerSend, пишет в PostgreSQL, ставит rate limit в Redis.
  3. Клиент подтверждает код → выдается device-bound сессия (opaque token + refresh), публичные ключи устройства (identity/prekeys) загружаются в `auth`.

- Логин/сессии:
  - E-mail + пароль + код 2FA → проверка → refresh rotation, выдача access/refresh (opaque), биндинг к устройству (fingerprint), аудит.
  - Выход из всех: инвалидация refresh, отзыв prekeys.

- Отправка сообщения (1:1, E2EE):
  1. Клиент шифрует payload через shared Go lib (Double Ratchet), формирует envelope (ciphertext + headers).
  2. Отправка в `api-gateway` → `realtime`.
  3. `realtime` валидирует права, сохраняет метаданные/статусы в PostgreSQL, публикует событие в Redis stream.
  4. Получатели с активным WS получают событие; оффлайн — хранятся в outbox, доставляются при reconnection.

- Группы/каналы:
  - Малые группы: sender keys, пересылка событий через `realtime`, метаданные групп в PostgreSQL, ACL через `auth`.
  - Каналы: автор публикует, подписчики читают; приватный канал — ключ канала рассылается через E2EE лички.

- Медиа:
  1. Клиент шифрует или подготавливает превью (на клиенте).
  2. Запрос на upload → `media` выдает presigned URL (MinIO) + metadata-id.
  3. Клиент грузит напрямую в MinIO; `media` пишет запись в PostgreSQL (TTL, размеры, mime), опционально запускает антивирус/thumbnailer.
  4. Для скачивания выдаются временные подписанные ссылки.

- Репорты:
  1. Пользователь выбирает сообщения → клиент формирует report packet (копии шифрует публичным ключом модерации).
  2. Отправка в `api-gateway` → `admin` → очередь в `moderation-agent`.
  3. Python-сервис расшифровывает, классифицирует, возвращает решение; `admin` применяет mute/бан, пишет в аудит.

- Звонки:
  - Сигнальные сообщения проходят через `realtime` (call_signal events), медиапоток — WebRTC (p2p/STUN/TURN, конфиг в .env), E2EE SRTP.

## Безопасность транспортов

- TLS 1.2+ с HSTS, CSP, CSRF tokens, strict CORS.
- WebSocket с токеном доступа (opaque) + device binding.
- Rate limit/абьюз через Redis (token bucket), капчи при подозрении (в клиенте).

## Хранение и ретенция

- PostgreSQL: пользователи, сессии, диалоги, метаданные сообщений, ключи устройств (публичные), настройки, жалобы, аудиты.
- Redis: кэш профилей/ключей, очереди событий, presence, rate limit counters.
- MinIO: файлы/медиа (шифротексты), TTL и presigned URLs.
- Логи: JSON с request-id/trace-id; метрики Prometheus; трассировки Jaeger/OTel.
