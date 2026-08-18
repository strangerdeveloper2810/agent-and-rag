#!/usr/bin/env bash
# ═══════════════════════════════════════════════════════════════════════════════
# J.A.R.V.I.S. — VPS Bootstrap (1 LẦN, an toàn cho VPS DÙNG CHUNG)
# ═══════════════════════════════════════════════════════════════════════════════
# Khác bản trước: KHÔNG cài host Nginx, KHÔNG reset UFW, KHÔNG giả định VPS
# chỉ chạy 1 mình JARVIS. Viết cho đúng thực tế đã kiểm tra: VPS đã có Docker,
# UFW đã cấu hình sẵn (chỉ mở 22/80/443 + vài rule chặn scanner), port 80/443
# đã bị 1 app khác chiếm (nginx container riêng của app đó). Script này chỉ:
#   1. Kiểm tra Docker đã có (không tự ý apt upgrade/cài lại trên server dùng chung)
#   2. Tạo thư mục /opt/jarvis (data/backups/logs)
#   3. Mở thêm đúng 1 port cho JARVIS trên UFW (KHÔNG đụng rule khác đã có)
#   4. Tạo cron backup Postgres hàng ngày
#
# KHÔNG clone code / KHÔNG build container ở đây — việc đó do
# deploy/deploy-to-vps.sh (chạy từ máy LOCAL, dùng SSH đã cấu hình sẵn) đảm nhiệm,
# vì code build từ nguồn local rsync lên, không cần VPS có quyền pull GitHub riêng.
#
# Usage:
#   Script này được deploy/deploy-to-vps.sh TỰ ĐỘNG chạy từ xa qua SSH ở mỗi
#   lần deploy (idempotent) — bạn KHÔNG cần tự tay chạy script này.
#
#   Chỉ chạy tay khi cần debug/bootstrap riêng lẻ, và LUÔN chạy TRÊN VPS
#   (không phải trên máy local của bạn):
#     ssh your-vps
#     sudo bash /opt/jarvis-tmp-setup.sh   # hoặc scp file này lên trước rồi chạy
# ═══════════════════════════════════════════════════════════════════════════════

set -euo pipefail

APP_NAME="jarvis"
APP_DIR="/opt/${APP_NAME}"
WEB_PORT="${WEB_HOST_PORT:-8090}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

log()  { echo -e "${GREEN}[✓]${NC} $*"; }
warn() { echo -e "${YELLOW}[!]${NC} $*"; }
err()  { echo -e "${RED}[✗]${NC} $*" >&2; }
info() { echo -e "${CYAN}[→]${NC} $*"; }

preflight() {
  info "Kiểm tra điều kiện..."

  if [[ $EUID -ne 0 ]]; then
    err "Script này cần chạy bằng root (sudo)"
    exit 1
  fi

  if ! command -v docker &>/dev/null; then
    err "Docker chưa cài. VPS dùng chung — KHÔNG tự động cài đặt/nâng cấp hệ thống"
    err "  để tránh ảnh hưởng app khác đang chạy. Cài Docker thủ công trước:"
    err "  curl -fsSL https://get.docker.com | sh"
    exit 1
  fi
  log "Docker đã có: $(docker --version)"

  if ! docker compose version &>/dev/null; then
    err "Docker Compose plugin chưa có. Cài: apt-get install docker-compose-plugin"
    exit 1
  fi
  log "Docker Compose: $(docker compose version)"

  if ! command -v rsync &>/dev/null; then
    info "rsync chưa có trên VPS, đang tự động cài đặt..."
    apt-get update -qq && apt-get install -y -qq rsync
  fi
  log "rsync đã có: $(rsync --version | head -n1)"
}

setup_dirs() {
  info "Tạo thư mục ứng dụng tại ${APP_DIR}..."
  mkdir -p "${APP_DIR}" "${APP_DIR}/backups" "${APP_DIR}/logs"
  log "Thư mục sẵn sàng: ${APP_DIR}"
  warn "Code sẽ được rsync lên đây bởi deploy/deploy-to-vps.sh (chạy từ máy local),"
  warn "không tạo git repo trên VPS này."
}

setup_firewall() {
  info "Mở thêm port ${WEB_PORT} cho JARVIS trên UFW (không đụng rule khác)..."
  if ! command -v ufw &>/dev/null; then
    warn "UFW chưa cài — bỏ qua bước này, tự cấu hình firewall theo cách VPS đang dùng."
    return
  fi
  ufw allow "${WEB_PORT}/tcp" comment 'JARVIS web' || true
  log "UFW: đã mở port ${WEB_PORT}/tcp (các rule khác giữ nguyên)"
  ufw status | grep -i "${WEB_PORT}" || warn "Không thấy rule vừa thêm — kiểm tra 'ufw status' thủ công"
}

create_backup_script() {
  info "Tạo script backup Postgres..."

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

# Giữ backup 7 ngày gần nhất
find "${BACKUP_DIR}" -type f -mtime +7 -delete

echo "[✓] Backup xong: ${BACKUP_DIR}/*_${TIMESTAMP}.*"
BACKUP

  chmod +x "${APP_DIR}/backup.sh"

  # Cron backup hằng ngày 2h sáng — chỉ thêm dòng của JARVIS, không ghi đè cron
  # khác đã có trên VPS dùng chung.
  ( crontab -l 2>/dev/null | grep -v "${APP_DIR}/backup.sh" || true; \
    echo "0 2 * * * ${APP_DIR}/backup.sh >> ${APP_DIR}/logs/backup.log 2>&1" ) | crontab -

  # Cron tự động dọn dẹp Disk & tối ưu RAM mỗi 30 phút
  ( crontab -l 2>/dev/null | grep -v "${APP_DIR}/scripts/optimize-vps.sh" || true; \
    echo "*/30 * * * * ${APP_DIR}/scripts/optimize-vps.sh >> ${APP_DIR}/logs/optimize.log 2>&1" ) | crontab -

  log "Backup script + cron tối ưu RAM/Disk đã tạo"
}

main() {
  echo "═══════════════════════════════════════════════════════════════"
  echo "  J.A.R.V.I.S. — VPS Bootstrap (shared-server safe)"
  echo "═══════════════════════════════════════════════════════════════"
  echo ""

  preflight
  setup_dirs
  setup_firewall
  create_backup_script

  echo ""
  echo "═══════════════════════════════════════════════════════════════"
  echo "  Bootstrap xong!"
  echo "═══════════════════════════════════════════════════════════════"
  echo ""
  echo "  Bước tiếp theo (chạy từ máy LOCAL, không phải trên VPS):"
  echo "    ./deploy/deploy-to-vps.sh"
  echo ""
  echo "  Script đó sẽ rsync code + env production lên ${APP_DIR}, build và"
  echo "  chạy docker compose. Ứng dụng sẽ chạy ở cổng ${WEB_PORT} (không phải"
  echo "  80/443 vì đã bị app khác trên VPS này chiếm)."
  echo ""
}

main "$@"
