#!/usr/bin/env bash
set -euo pipefail

BASE=${1:-http://localhost}

curl -sf "$BASE:8080/healthz" && echo " api-gateway ok"
curl -sf "$BASE:8081/healthz" && echo " auth ok"
curl -sf "$BASE:8082/healthz" && echo " realtime ok"
curl -sf "$BASE:8083/healthz" && echo " media ok"
curl -sf "$BASE:8084/healthz" && echo " admin ok"
curl -sf "$BASE:8085/healthz" && echo " moderation-agent ok"
