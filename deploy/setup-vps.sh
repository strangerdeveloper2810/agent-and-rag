#!/usr/bin/env bash
# ═══════════════════════════════════════════════════════════════════════════════
# J.A.R.V.I.S. — VPS Deployment Setup Script
# ═══════════════════════════════════════════════════════════════════════════════
# Usage:
#   1. Copy to VPS:  scp deploy/setup-vps.sh user@your-vps:/tmp/
#   2. Run on VPS:   sudo bash /tmp/setup-vps.sh
#
# Prerequisites: Ubuntu 22.04+ / Debian 12+ VPS with root/sudo access
# ═══════════════════════════════════════════════════════════════════════════════

set -euo pipefail

# ── Configuration ────────────────────────────────────────────────────────────
APP_NAME="jarvis"
APP_DIR="/opt/${APP_NAME}"
REPO_URL="git@github.com:strangerdeveloper2810/agent-and-rag.git"
BRANCH="feat/frontend-redesign-deepseek"
DOMAIN="${JARVIS_DOMAIN:-}"  # Set via env or prompted later
DEPLOY_USER="deploy"

# ── Colors ───────────────────────────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

log()  { echo -e "${GREEN}[✓]${NC} $*"; }
warn() { echo -e "${YELLOW}[!]${NC} $*"; }
err()  { echo -e "${RED}[✗]${NC} $*" >&2; }
info() { echo -e "${CYAN}[→]${NC} $*"; }

# ═══════════════════════════════════════════════════════════════════════════════
# Step 0: Pre-flight checks
# ═══════════════════════════════════════════════════════════════════════════════
preflight() {
  info "Running pre-flight checks..."

  if [[ $EUID -ne 0 ]]; then
    err "This script must be run as root (use sudo)"
    exit 1
  fi

  # Detect OS
  if ! grep -qiE 'ubuntu|debian' /etc/os-release 2>/dev/null; then
    warn "This script is tested on Ubuntu/Debian. Continuing anyway..."
  fi

  log "Pre-flight checks passed"
}

# ═══════════════════════════════════════════════════════════════════════════════
# Step 1: System packages
# ═══════════════════════════════════════════════════════════════════════════════
install_system_deps() {
  info "Updating system packages..."
  apt-get update -qq
  apt-get upgrade -y -qq

  info "Installing essential packages..."
  apt-get install -y -qq \
    curl wget git unzip jq htop \
    ca-certificates gnupg lsb-release \
    ufw fail2ban \
    certbot

  log "System packages installed"
}

# ═══════════════════════════════════════════════════════════════════════════════
# Step 2: Docker & Docker Compose
# ═══════════════════════════════════════════════════════════════════════════════
install_docker() {
  if command -v docker &>/dev/null; then
    log "Docker already installed: $(docker --version)"
    return
  fi

  info "Installing Docker..."
  curl -fsSL https://get.docker.com | sh

  # Add deploy user to docker group
  if id "${DEPLOY_USER}" &>/dev/null; then
    usermod -aG docker "${DEPLOY_USER}"
  fi

  systemctl enable docker
  systemctl start docker

  log "Docker installed: $(docker --version)"
  log "Docker Compose: $(docker compose version)"
}

# ═══════════════════════════════════════════════════════════════════════════════
# Step 3: Create deploy user & directories
# ═══════════════════════════════════════════════════════════════════════════════
setup_user_and_dirs() {
  if ! id "${DEPLOY_USER}" &>/dev/null; then
    info "Creating deploy user: ${DEPLOY_USER}"
    useradd -m -s /bin/bash "${DEPLOY_USER}"
    usermod -aG docker "${DEPLOY_USER}"
    log "User '${DEPLOY_USER}' created"
  else
    log "User '${DEPLOY_USER}' already exists"
  fi

  info "Creating application directories..."
  mkdir -p "${APP_DIR}"
  mkdir -p "${APP_DIR}/data"
  mkdir -p "${APP_DIR}/backups"
  mkdir -p "${APP_DIR}/logs"
  mkdir -p "${APP_DIR}/ssl"
  chown -R "${DEPLOY_USER}:${DEPLOY_USER}" "${APP_DIR}"

  log "Directories created at ${APP_DIR}"
}

# ═══════════════════════════════════════════════════════════════════════════════
# Step 4: Firewall (UFW)
# ═══════════════════════════════════════════════════════════════════════════════
setup_firewall() {
  info "Configuring firewall..."
  ufw --force reset
  ufw default deny incoming
  ufw default allow outgoing

  # SSH
  ufw allow 22/tcp comment 'SSH'
  # HTTP/HTTPS
  ufw allow 80/tcp comment 'HTTP'
  ufw allow 443/tcp comment 'HTTPS'

  ufw --force enable
  log "Firewall configured (SSH + HTTP + HTTPS only)"
}

# ═══════════════════════════════════════════════════════════════════════════════
# Step 5: Clone repository
# ═══════════════════════════════════════════════════════════════════════════════
clone_repo() {
  if [[ -d "${APP_DIR}/.git" ]]; then
    info "Repository exists, pulling latest..."
    cd "${APP_DIR}"
    sudo -u "${DEPLOY_USER}" git fetch origin
    sudo -u "${DEPLOY_USER}" git checkout "${BRANCH}"
    sudo -u "${DEPLOY_USER}" git pull origin "${BRANCH}"
  else
    info "Cloning repository..."
    sudo -u "${DEPLOY_USER}" git clone -b "${BRANCH}" "${REPO_URL}" "${APP_DIR}"
  fi

  log "Repository ready at ${APP_DIR}"
}

# ═══════════════════════════════════════════════════════════════════════════════
# Step 6: Generate production .env
# ═══════════════════════════════════════════════════════════════════════════════
setup_env() {
  local ENV_FILE="${APP_DIR}/env/.env"

  if [[ -f "${ENV_FILE}" ]]; then
    warn "Production .env already exists at ${ENV_FILE}"
    warn "Skipping .env generation. Edit manually if needed."
    return
  fi

  info "Generating production .env from template..."
  cp "${APP_DIR}/env/.env.example" "${ENV_FILE}"

  # Generate secure secrets
  JWT_SECRET=$(openssl rand -hex 32)
  JWT_REFRESH_SECRET=$(openssl rand -hex 32)
  PG_PASSWORD=$(openssl rand -hex 16)
  S3_SECRET=$(openssl rand -hex 16)

  # Replace placeholders
  sed -i "s|JWT_SECRET=.*|JWT_SECRET=${JWT_SECRET}|" "${ENV_FILE}"
  sed -i "s|JWT_REFRESH_SECRET=.*|JWT_REFRESH_SECRET=${JWT_REFRESH_SECRET}|" "${ENV_FILE}"
  sed -i "s|POSTGRES_PASSWORD=.*|POSTGRES_PASSWORD=${PG_PASSWORD}|" "${ENV_FILE}"
  sed -i "s|PG_CONNECTION_STRING=.*|PG_CONNECTION_STRING=postgresql://jarvis:${PG_PASSWORD}@postgres:5432/ai_agent_tut|" "${ENV_FILE}"
  sed -i "s|S3_SECRET_KEY=.*|S3_SECRET_KEY=${S3_SECRET}|" "${ENV_FILE}"

  # Set CORS origin if domain is known
  if [[ -n "${DOMAIN}" ]]; then
    sed -i "s|CORS_ORIGIN=.*|CORS_ORIGIN=https://${DOMAIN}|" "${ENV_FILE}"
    sed -i "s|GOOGLE_REDIRECT_URI=.*|GOOGLE_REDIRECT_URI=https://${DOMAIN}/api/auth/google/callback|" "${ENV_FILE}"
  fi

  chmod 600 "${ENV_FILE}"
  chown "${DEPLOY_USER}:${DEPLOY_USER}" "${ENV_FILE}"

  warn "═══════════════════════════════════════════════════════════════════"
  warn "  IMPORTANT: Edit ${ENV_FILE} to fill in:"
  warn "    - MONGODB_URI (MongoDB Atlas connection string)"
  warn "    - ANTHROPIC_API_KEY / GOOGLE_API_KEY / DEEPSEEK_API_KEY"
  warn "    - VOYAGE_API_KEY (embedding)"
  warn "    - GOOGLE_CLIENT_ID/SECRET (OAuth — optional)"
  warn "    - CORS_ORIGIN (your domain)"
  warn "═══════════════════════════════════════════════════════════════════"
  log "Production .env generated with secure secrets"
}

# ═══════════════════════════════════════════════════════════════════════════════
# Step 7: Nginx reverse proxy + SSL
# ═══════════════════════════════════════════════════════════════════════════════
setup_nginx_ssl() {
  if [[ -z "${DOMAIN}" ]]; then
    warn "No DOMAIN set. Skipping Nginx/SSL setup."
    warn "Set JARVIS_DOMAIN=your-domain.com and re-run, or configure manually."
    return
  fi

  info "Installing Nginx..."
  apt-get install -y -qq nginx

  info "Creating Nginx config for ${DOMAIN}..."
  cat > "/etc/nginx/sites-available/${APP_NAME}" <<NGINX
server {
    listen 80;
    server_name ${DOMAIN};

    # Certbot ACME challenge
    location /.well-known/acme-challenge/ {
        root /var/www/certbot;
    }

    # Redirect HTTP -> HTTPS (enabled after SSL cert)
    location / {
        return 301 https://\$host\$request_uri;
    }
}

server {
    listen 443 ssl http2;
    server_name ${DOMAIN};

    ssl_certificate     /etc/letsencrypt/live/${DOMAIN}/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/${DOMAIN}/privkey.pem;
    ssl_protocols       TLSv1.2 TLSv1.3;
    ssl_ciphers         HIGH:!aNULL:!MD5;

    client_max_body_size 50M;

    # Frontend (React SPA via Docker nginx on port 80)
    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
    }

    # API Gateway (Fastify on port 3001)
    location /api/ {
        proxy_pass http://127.0.0.1:3001;
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        proxy_read_timeout 300s;
        proxy_buffering off;   # SSE streaming support
    }

    # Security headers
    add_header X-Frame-Options DENY;
    add_header X-Content-Type-Options nosniff;
    add_header X-XSS-Protection "1; mode=block";
    add_header Referrer-Policy strict-origin-when-cross-origin;

    gzip on;
    gzip_types text/plain text/css application/json application/javascript text/xml application/xml;
}
NGINX

  # Enable site
  ln -sf "/etc/nginx/sites-available/${APP_NAME}" "/etc/nginx/sites-enabled/"
  rm -f /etc/nginx/sites-enabled/default

  # Create ACME challenge dir
  mkdir -p /var/www/certbot

  # Test config (HTTP only first)
  info "Obtaining SSL certificate via Let's Encrypt..."
  # Temporarily serve HTTP for ACME challenge
  cat > "/etc/nginx/sites-available/${APP_NAME}-temp" <<TEMP
server {
    listen 80;
    server_name ${DOMAIN};
    location /.well-known/acme-challenge/ {
        root /var/www/certbot;
    }
    location / {
        return 200 'Setting up...';
    }
}
TEMP
  ln -sf "/etc/nginx/sites-available/${APP_NAME}-temp" "/etc/nginx/sites-enabled/${APP_NAME}"
  nginx -t && systemctl restart nginx

  certbot certonly --webroot -w /var/www/certbot -d "${DOMAIN}" --agree-tos --non-interactive --email "admin@${DOMAIN}" || {
    warn "SSL certificate failed. You can run certbot manually later:"
    warn "  certbot certonly --webroot -w /var/www/certbot -d ${DOMAIN}"
  }

  # Restore full config
  ln -sf "/etc/nginx/sites-available/${APP_NAME}" "/etc/nginx/sites-enabled/${APP_NAME}"
  rm -f "/etc/nginx/sites-available/${APP_NAME}-temp"
  nginx -t && systemctl restart nginx

  # Auto-renew cron
  echo "0 3 * * * certbot renew --quiet --post-hook 'systemctl reload nginx'" | crontab -

  log "Nginx + SSL configured for ${DOMAIN}"
}

# ═══════════════════════════════════════════════════════════════════════════════
# Step 8: Create deploy script (for subsequent deployments)
# ═══════════════════════════════════════════════════════════════════════════════
create_deploy_script() {
  info "Creating deployment helper script..."

  cat > "${APP_DIR}/deploy.sh" <<'DEPLOY'
#!/usr/bin/env bash
# ── J.A.R.V.I.S. Quick Deploy ──
# Run: cd /opt/jarvis && ./deploy.sh
set -euo pipefail

APP_DIR="/opt/jarvis"
BRANCH="${1:-feat/frontend-redesign-deepseek}"

echo "═══ J.A.R.V.I.S. Deployment ═══"
echo "Branch: ${BRANCH}"
echo ""

cd "${APP_DIR}"

# 1. Pull latest code
echo "[→] Pulling latest code..."
git fetch origin
git checkout "${BRANCH}"
git pull origin "${BRANCH}"

# 2. Build & restart containers
echo "[→] Building Docker images..."
docker compose -f docker/deployment/docker-compose.yml build --parallel

echo "[→] Stopping old containers..."
docker compose -f docker/deployment/docker-compose.yml down

echo "[→] Starting new containers..."
docker compose -f docker/deployment/docker-compose.yml up -d

# 3. Health check
echo "[→] Waiting for services to start..."
sleep 15

echo "[→] Checking service health..."
SERVICES=("jarvis-agent-go" "jarvis-api" "jarvis-web" "jarvis-postgres" "jarvis-redis")
ALL_HEALTHY=true

for svc in "${SERVICES[@]}"; do
  STATUS=$(docker inspect --format='{{.State.Health.Status}}' "${svc}" 2>/dev/null || echo "missing")
  if [[ "${STATUS}" == "healthy" ]]; then
    echo "  [✓] ${svc}: ${STATUS}"
  else
    echo "  [!] ${svc}: ${STATUS}"
    ALL_HEALTHY=false
  fi
done

if $ALL_HEALTHY; then
  echo ""
  echo "[✓] All services healthy!"
else
  echo ""
  echo "[!] Some services are not healthy yet. Check logs:"
  echo "    docker compose -f docker/deployment/docker-compose.yml logs -f"
fi

# 4. Cleanup old images
echo "[→] Cleaning up old Docker images..."
docker image prune -f

echo ""
echo "═══ Deployment complete ═══"
DEPLOY

  chmod +x "${APP_DIR}/deploy.sh"
  chown "${DEPLOY_USER}:${DEPLOY_USER}" "${APP_DIR}/deploy.sh"

  log "Deploy script created at ${APP_DIR}/deploy.sh"
}

# ═══════════════════════════════════════════════════════════════════════════════
# Step 9: Create backup script
# ═══════════════════════════════════════════════════════════════════════════════
create_backup_script() {
  info "Creating backup script..."

  cat > "${APP_DIR}/backup.sh" <<'BACKUP'
#!/usr/bin/env bash
# ── J.A.R.V.I.S. Database Backup ──
set -euo pipefail

BACKUP_DIR="/opt/jarvis/backups"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

echo "[→] Backing up PostgreSQL..."
docker exec jarvis-postgres pg_dump -U jarvis ai_agent_tut | gzip > "${BACKUP_DIR}/postgres_${TIMESTAMP}.sql.gz"

echo "[→] Backing up Redis..."
docker exec jarvis-redis redis-cli BGSAVE
sleep 2
docker cp jarvis-redis:/data/dump.rdb "${BACKUP_DIR}/redis_${TIMESTAMP}.rdb"

# Keep only last 7 days of backups
find "${BACKUP_DIR}" -type f -mtime +7 -delete

echo "[✓] Backup completed: ${BACKUP_DIR}/*_${TIMESTAMP}.*"
BACKUP

  chmod +x "${APP_DIR}/backup.sh"
  chown "${DEPLOY_USER}:${DEPLOY_USER}" "${APP_DIR}/backup.sh"

  # Daily backup cron at 2:00 AM
  (crontab -u "${DEPLOY_USER}" -l 2>/dev/null || true; echo "0 2 * * * ${APP_DIR}/backup.sh >> ${APP_DIR}/logs/backup.log 2>&1") | crontab -u "${DEPLOY_USER}" -

  log "Backup script created with daily cron"
}

# ═══════════════════════════════════════════════════════════════════════════════
# Step 10: Update docker-compose for VPS (expose on localhost only)
# ═══════════════════════════════════════════════════════════════════════════════
patch_docker_compose() {
  info "Patching docker-compose for VPS deployment..."

  local COMPOSE="${APP_DIR}/docker/deployment/docker-compose.yml"

  # Replace port mappings to bind to localhost only (Nginx handles external traffic)
  # web: 80 -> 8080 (Nginx upstream)
  # api: 3001 -> 127.0.0.1:3001
  # postgres/redis/minio: localhost only

  if grep -q '"80:80"' "${COMPOSE}"; then
    sed -i 's/"80:80"/"8080:80"/' "${COMPOSE}"
    sed -i 's/"3001:3001"/"127.0.0.1:3001:3001"/' "${COMPOSE}"
    sed -i 's/"3002:3002"/"127.0.0.1:3002:3002"/' "${COMPOSE}"
    sed -i 's/"5432:5432"/"127.0.0.1:5432:5432"/' "${COMPOSE}"
    sed -i 's/"6379:6379"/"127.0.0.1:6379:6379"/' "${COMPOSE}"
    sed -i 's/"9000:9000"/"127.0.0.1:9000:9000"/' "${COMPOSE}"
    sed -i 's/"9001:9001"/"127.0.0.1:9001:9001"/' "${COMPOSE}"
    log "Docker ports bound to localhost (Nginx handles external traffic)"
  else
    log "Docker ports already patched"
  fi
}

# ═══════════════════════════════════════════════════════════════════════════════
# Main
# ═══════════════════════════════════════════════════════════════════════════════
main() {
  echo "═══════════════════════════════════════════════════════════════"
  echo "  J.A.R.V.I.S. — VPS Deployment Setup"
  echo "═══════════════════════════════════════════════════════════════"
  echo ""

  # Prompt for domain if not set
  if [[ -z "${DOMAIN}" ]]; then
    read -rp "$(echo -e "${CYAN}Enter your domain (or press Enter to skip SSL):${NC} ")" DOMAIN
  fi

  preflight
  install_system_deps
  install_docker
  setup_user_and_dirs
  setup_firewall
  clone_repo
  setup_env
  patch_docker_compose
  setup_nginx_ssl
  create_deploy_script
  create_backup_script

  echo ""
  echo "═══════════════════════════════════════════════════════════════"
  echo "  Setup Complete!"
  echo "═══════════════════════════════════════════════════════════════"
  echo ""
  echo "  Next steps:"
  echo "    1. Edit production secrets:"
  echo "       nano ${APP_DIR}/env/.env"
  echo ""
  echo "    2. Deploy the application:"
  echo "       cd ${APP_DIR} && ./deploy.sh"
  echo ""
  echo "    3. View logs:"
  echo "       docker compose -f ${APP_DIR}/docker/deployment/docker-compose.yml logs -f"
  echo ""
  echo "    4. Manual backup:"
  echo "       ${APP_DIR}/backup.sh"
  echo ""
  if [[ -n "${DOMAIN}" ]]; then
    echo "    5. Access: https://${DOMAIN}"
  else
    echo "    5. Access: http://YOUR_VPS_IP"
  fi
  echo ""
}

main "$@"
