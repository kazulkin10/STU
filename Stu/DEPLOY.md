# DEPLOY (Ubuntu/RedOS)

## Зависимости
```bash
sudo apt-get update
sudo apt-get install -y git docker.io docker-compose-plugin
sudo systemctl enable --now docker
```

## Подготовка
```bash
cd /opt/Stu   # путь к репозиторию
cp .env.example .env   # заполните пароли, MAIL_MODE, MAILERSEND_API_KEY, S3 ключи, TIMEWEB_AGENT_API_KEY
```

## Dev (localhost)
```bash
docker compose -f deploy/docker-compose.dev.yml up --build
# Накатить миграции (один раз для новой БД)
docker compose -f deploy/docker-compose.dev.yml exec postgres psql -U stu -d stu -f /app/migrations/0001_init.sql
docker compose -f deploy/docker-compose.dev.yml exec postgres psql -U stu -d stu -f /app/migrations/0002_auth_and_indexes.sql
docker compose -f deploy/docker-compose.dev.yml exec postgres psql -U stu -d stu -f /app/migrations/0003_messages_status.sql
docker compose -f deploy/docker-compose.dev.yml exec postgres psql -U stu -d stu -f /app/migrations/0004_admin_reports.sql
docker compose -f deploy/docker-compose.dev.yml exec postgres psql -U stu -d stu -f /app/migrations/0005_admin_security_ai.sql
```
Порты dev: gateway 8080, auth 8081, realtime 8082, media 8083, mailpit 8025/1025, postgres 5432, redis 6379, minio 9000/9001.

## Prod (VPS)
Gateway наружу на 9000, остальные сервисы внутри сети Compose.
```bash
docker compose -f deploy/docker-compose.prod.yml up -d --build
```
При свежей БД примените миграции так же, как в dev, но с prod compose:
```bash
docker compose -f deploy/docker-compose.prod.yml exec postgres psql -U stu -d stu -f /app/migrations/0001_init.sql
...
docker compose -f deploy/docker-compose.prod.yml exec postgres psql -U stu -d stu -f /app/migrations/0005_admin_security_ai.sql
```

## Проверки
```bash
curl -i http://<host>:9000/v1/ping
curl -i -X POST http://<host>:9000/v1/auth/register -H "Content-Type: application/json" -d '{"email":"t1@example.com","password":"Passw0rd"}'
```
Mailpit UI (dev/prod тест): http://<host>:8025

## Сделать админом
```bash
docker compose -f deploy/docker-compose.prod.yml exec postgres \
  psql -U stu -d stu -c "update users set is_admin=true where email='admin@example.com';"
```

## Прокси / статика
- /v1/* идут через api-gateway внутри compose.
- Web UI: http://<host>:9000/
- WS: ws://<host>:9000/v1/ws (Upgrade проксируется на realtime)
- Admin UI: http://<host>:9000/admin (через gateway)

## Примечания
- mailpit в prod по умолчанию открыт на 8025/1025 — выключите или ограничьте firewall, если не нужно.
- Все секреты (MailerSend, Timeweb agent) должны быть только в .env, не в клиенте.
