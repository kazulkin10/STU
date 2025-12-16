# API (скелет)

Версия: `v1` (HTTP/JSON). Внутренний транспорт — gRPC (будет добавлен). Реалтайм — WebSocket (`/ws`) с событиями `new_message`, `message_edit`, `message_delete`, `typing`, `read_receipt`, `presence`, `call_signal`.

## Auth (HTTP, сервис auth)

- `POST /v1/auth/register` — {email, password} → {user_id, status:verification_sent} (код уходит в Mailpit/SMTP)
- `POST /v1/auth/verify` — {email, code, device_name?, platform?} → {user_id, device_id, access_token, refresh_token}
- `POST /v1/auth/login` — {email, password, device_name?, platform?} → {user_id, device_id, access_token, refresh_token} (только для активных аккаунтов)
- `POST /v1/auth/refresh` — {refresh_token} → {access_token, refresh_token} (ротация + reuse detection)
- `POST /v1/auth/logout` — {refresh_token} → revoke session
- `POST /v1/auth/logout_all` — {refresh_token} → revoke all user sessions

## Dialogs/messages (HTTP, через api-gateway)

- `POST /v1/dialogs` — Bearer access; {user_id? or email} → {dialog_id}
- `GET /v1/dialogs` — список диалогов с last_message и unread_count
- `GET /v1/dialogs/{id}/messages?limit=&before=` — история сообщений
- `POST /v1/dialogs/{id}/messages` — {text} → создаёт сообщение
- `POST /v1/dialogs/{id}/messages/{mid}/delivered` — отметить доставку
- `POST /v1/dialogs/{id}/messages/{mid}/read` — отметить прочтение

Токены — opaque, предполагается хранение в httpOnly cookie (web) или защищённом storage (desktop/mobile). Access TTL 15m, Refresh TTL 30d (будет настраиваемо).

## API gateway

- `GET /v1/ping` — ping.
- План: proxy `/v1/users`, `/v1/dialogs`, `/v1/messages`, `/v1/media`, `/v1/keys`, `/v1/calls`.

## Media

- План: `POST /v1/media/upload-request` → presigned URL, metadata_id.
- План: `GET /v1/media/{id}` → временная ссылка (TT L).

## Admin

- План: `GET/POST /v1/admin/reports`, `/v1/admin/actions`, `/v1/admin/stats`.

## Moderation-agent (Python)

- `POST /analyze` — принимает report packet (ciphertext), возвращает вердикт; авторизация через сервисный токен (будет).

## Версионирование

- HTTP префикс `/v1`.
- gRPC: отдельные proto с package `stu.v1`, последующая эволюция через новые версии.
