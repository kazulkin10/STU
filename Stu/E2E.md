# E2E сценарии Stu (dev, docker-compose.dev.yml)

## 1. Два пользователя, чат в реальном времени
1. Запуск: `docker compose -f deploy/docker-compose.dev.yml up --build -d` и миграции (0001,0002,0003,0005 как в DEPLOY.md).
2. Открыть http://localhost:8080/.
3. Пользователь A:
   - Регистрация (email A), код из Mailpit http://localhost:8025, подтвердить, войти.
4. Пользователь B:
   - В другом браузере/инкогнито повторить регистрацию/подтверждение/вход (email B).
5. Пользователь A создаёт диалог по email B.
6. Оба открывают диалог. A отправляет сообщение.
7. Ожидание/проверка:
   - У B в реальном времени приходит сообщение (WS /v1/ws через gateway).
   - A видит статус `✓` (delivered) после авто-обработки delivered от B.
   - B, находясь в чате, отправляет read → у A статус `✓✓`.
8. Перезагрузка страницы:
   - Оба остаются авторизованы (refresh), список диалогов и история загружаются, WS переподключается.

## 2. Репорт, AI-вердикт, бан и блокировки
1. Предусловия: вышеописанные шаги, есть диалог A-B и сообщения.
2. A отправляет жалобу на сообщение B:
   ```bash
   curl -H "Authorization: Bearer <access_A>" -H "Content-Type: application/json" \
     -d '{"reported_user_id":"<userB_id>","dialog_id":"<dialog_id>","message_id":<msg_id>,"reason":"spam"}' \
     http://localhost:8080/v1/reports
   ```
3. Назначить админа (одному из пользователей или отдельному email):
   `docker compose -f deploy/docker-compose.dev.yml exec postgres psql -U stu -d stu -c "update users set is_admin=true where email='<admin_email>';"`
4. Админ:
   - Открывает http://localhost:8080/admin
   - MFA: пароль → TOTP → email-код (Mailpit).
   - Видит репорт в таблице, колонка AI показывает verdict/confidence (если настроен TIMEWEB_AGENT_API_KEY, иначе “в обработке”).
   - Нажимает “Бан”.
5. Проверки для забаненного пользователя:
   - `POST /v1/auth/login` → 403 `{"error":"banned","reason":..., "banned_at":...}`
   - WS подключение к /v1/ws → 403
   - `POST /v1/dialogs/{id}/messages` → 403
6. Админ нажимает “Разбан” → пользователь снова может логиниться/отправлять.

## 3. Logout и ротация токенов
1. В UI нажать “Выйти” → localStorage очищен, редирект на форму входа.
2. После истечения access (форсировать, изменив БД или ждать TTL) — первый 401 вызывает refresh один раз, запрос повторяется, UI не “залипает” в цикле, при ошибке refresh → выбрасывает на логин.

## Ожидаемые результаты
- Realtime сообщения без ручных обновлений, статусы delivered/read точные для 1:1.
- WS переподключается с бэкоффом и обновлённым access.
- Админка на русском: список репортов, AI вердикт, бан/разбан, закрытие жалобы.
- Banned-пользователь заблокирован в login/send/ws.
