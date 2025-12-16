#!/usr/bin/env bash
set -euo pipefail

# Quick installer for RedOS/RHEL-like systems
sudo dnf install -y git make golang python3 python3-venv docker docker-compose nginx
sudo systemctl enable --now docker

echo "Create stu user (system)"
if ! id -u stu >/dev/null 2>&1; then
  sudo useradd -r -m stu
fi

echo "Copy .env to /etc/stu/.env and binaries to /opt/stu/bin before starting systemd units."
