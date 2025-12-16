# Stu: быстрый старт на Windows 11 (WSL2/Docker Desktop)

1. Установите [Docker Desktop](https://www.docker.com/products/docker-desktop/) и включите WSL2 backend.
2. Установите Git и Go (по желанию, для локальной сборки), но для dev достаточно Docker:
   - `winget install Git.Git`
   - `winget install GoLang.Go`
3. Клонируйте репозиторий и перейдите в каталог.
4. Скопируйте переменные окружения:
   ```powershell
   copy .env.example .env
   ```
5. Запуск окружения разработки (в PowerShell/WSL):
   ```powershell
   make dev   # или: docker compose -f deploy/docker-compose.dev.yml up --build
   ```
6. Доступные сервисы:
   - API gateway: http://localhost:8080/healthz
   - Auth: http://localhost:8081/healthz
   - Realtime: http://localhost:8082/healthz
   - Media: http://localhost:8083/healthz
   - Admin: http://localhost:8084/healthz
   - Moderation-agent: http://localhost:8085/healthz
   - Mailpit (письма с кодами): http://localhost:8025 (SMTP 1025)
7. Проверка auth-флоу (пример через PowerShell):
   ```powershell
   # Регистрация
   Invoke-RestMethod -Method Post -Uri http://localhost:8081/v1/auth/register -Body (@{email="you@example.com";password="P@ssw0rd"} | ConvertTo-Json) -ContentType "application/json"
   # Откройте Mailpit и найдите код подтверждения
   # Подтверждение
   Invoke-RestMethod -Method Post -Uri http://localhost:8081/v1/auth/verify -Body (@{email="you@example.com";code="123456";device_name="pc";platform="win"} | ConvertTo-Json) -ContentType "application/json"
   # Ротация refresh
   Invoke-RestMethod -Method Post -Uri http://localhost:8081/v1/auth/refresh -Body (@{refresh_token="..."} | ConvertTo-Json) -ContentType "application/json"
   ```
8. Остановка:
   ```powershell
   make down   # или docker compose -f deploy/docker-compose.dev.yml down
   ```

Примечание: Если порты заняты, поменяйте их в `.env`. Для HTTPS в проде используйте nginx-конфиг `deploy/nginx/stu.conf` и свои сертификаты.
