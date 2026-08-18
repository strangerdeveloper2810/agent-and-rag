#!/usr/bin/env bash
# ═══════════════════════════════════════════════════════════════════════════════
# J.A.R.V.I.S. — Deploy lên VPS (LUÔN chạy ở máy LOCAL của bạn, KHÔNG bao giờ
# dùng sudo, KHÔNG bao giờ chạy trực tiếp trên VPS)
# ═══════════════════════════════════════════════════════════════════════════════
# Chạy script này từ THƯ MỤC GỐC của repo (nơi có pnpm-workspace.yaml), với tài
# khoản người dùng BÌNH THƯỜNG của bạn (không sudo) — vì nó cần dùng đúng
# ~/.ssh/config + SSH key của CHÍNH BẠN để nối tới VPS. Chạy bằng `sudo` sẽ tìm
# nhầm sang cấu hình SSH của root (/var/root/.ssh/...) và báo lỗi kết nối.
#
# Tự động, KHÔNG cần chạy tay deploy/setup-vps.sh hay bất kỳ script nào khác
# trước — script này tự SSH vào VPS chạy phần bootstrap (tạo /opt/jarvis, mở
# firewall, cron backup) ở MỖI LẦN deploy, an toàn để chạy lại nhiều lần
# (idempotent — không ghi đè/xoá gì đã có).
#   - Cần cấu hình SSH tới VPS trong ~/.ssh/config (mặc định dùng host "hr-vps",
#     đổi qua biến JARVIS_SSH_HOST nếu khác).
#
# Cách hoạt động:
#   - KHÔNG cần VPS có quyền pull GitHub riêng — code được rsync thẳng từ máy
#     local (đã pull/checkout đúng commit muốn deploy) lên qua kênh SSH đã có.
#   - Secret KHÔNG bao giờ nằm trong repo hay gửi qua git: file thật sống ở
#     env/.env.production (bạn tự điền/tái dùng key có sẵn) + secret hạ tầng
#     (JWT, mật khẩu Postgres/MinIO) TỰ SINH 1 LẦN và lưu ở
#     env/.env.production.secrets (cả 2 đã có trong .gitignore) — giữ nguyên
#     qua các lần deploy sau, không sinh lại mỗi lần (sinh lại sẽ làm session
#     JWT cũ + kết nối Postgres cũ hỏng hết).
#
# Usage:
#   ./deploy/deploy-to-vps.sh              # deploy code hiện tại đang checkout
#   JARVIS_SSH_HOST=my-vps ./deploy/deploy-to-vps.sh
#   WEB_HOST_PORT=8091 ./deploy/deploy-to-vps.sh   # đổi port public nếu 8090 bận
# ═══════════════════════════════════════════════════════════════════════════════

set -euo pipefail

SSH_HOST="${JARVIS_SSH_HOST:-hr-vps}"
REMOTE_DIR="${JARVIS_REMOTE_DIR:-/opt/jarvis}"
WEB_HOST_PORT="${WEB_HOST_PORT:-8090}"
COMPOSE_FILE="docker/deployment/docker-compose.yml"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

log()  { echo -e "${GREEN}[✓]${NC} $*"; }
warn() { echo -e "${YELLOW}[!]${NC} $*"; }
err()  { echo -e "${RED}[✗]${NC} $*" >&2; }
info() { echo -e "${CYAN}[→]${NC} $*"; }

# ── -1. KHÔNG được chạy bằng sudo/root ──────────────────────────────────────
# sudo đổi $HOME sang thư mục của root -> ssh đọc nhầm /var/root/.ssh/config
# (rỗng) thay vì ~/.ssh/config của bạn -> luôn báo "Không kết nối được SSH".
if [[ "${EUID}" -eq 0 ]]; then
  err "ĐỪNG chạy script này bằng sudo/root — nó cần SSH key của TÀI KHOẢN BẠN,"
  err "không phải của root. Chạy lại KHÔNG có sudo:"
  err "  ./deploy/deploy-to-vps.sh"
  exit 1
fi

# ── 0. Chạy đúng chỗ ─────────────────────────────────────────────────────────
if [[ ! -f "pnpm-workspace.yaml" ]]; then
  err "Chạy script này từ THƯ MỤC GỐC của repo (không thấy pnpm-workspace.yaml ở đây)."
  exit 1
fi

ENV_PROD="env/.env.production"
ENV_SECRETS="env/.env.production.secrets"

# ── 1. Đảm bảo có file API key thật cho production ──────────────────────────
if [[ ! -f "${ENV_PROD}" ]]; then
  warn "Chưa có ${ENV_PROD}."
  if [[ -f "env/.env" ]]; then
    info "Tìm thấy env/.env (dev) — copy làm điểm bắt đầu (giữ nguyên API key thật,"
    info "chỉ cần sửa lại phần hạ tầng bên dưới nếu cần)."
    cp "env/.env" "${ENV_PROD}"
  else
    info "Không có env/.env — copy từ template env/.env.example."
    cp "env/.env.example" "${ENV_PROD}"
  fi
  chmod 600 "${ENV_PROD}"
  err "Mở ${ENV_PROD} và điền/kiểm tra lại các API key thật:"
  err "  MONGODB_URI, GEMINI_API_KEY hoặc ANTHROPIC_API_KEY/DEEPSEEK_API_KEY,"
  err "  VOYAGE_API_KEY, GOOGLE_CLIENT_ID/SECRET, RESEND_API_KEY"
  err "Rồi chạy lại script này. (File này KHÔNG commit git — đã có trong .gitignore.)"
  exit 1
fi
log "Dùng API key từ ${ENV_PROD}"

# ── 2. Sinh secret hạ tầng 1 LẦN, giữ nguyên qua các lần deploy sau ──────────
if [[ ! -f "${ENV_SECRETS}" ]]; then
  info "Sinh secret hạ tầng mới (JWT, mật khẩu Postgres/MinIO) — chỉ sinh 1 lần,"
  info "lưu tại ${ENV_SECRETS} để các lần deploy sau dùng lại (sinh lại sẽ làm"
  info "session JWT cũ + dữ liệu Postgres cũ không đọc được nữa)."
  {
    echo "JWT_SECRET=$(openssl rand -hex 32)"
    echo "JWT_REFRESH_SECRET=$(openssl rand -hex 32)"
    echo "POSTGRES_PASSWORD=$(openssl rand -hex 16)"
    echo "S3_SECRET_KEY=$(openssl rand -hex 16)"
  } > "${ENV_SECRETS}"
  chmod 600 "${ENV_SECRETS}"
  log "Đã sinh secret hạ tầng mới tại ${ENV_SECRETS}"
else
  log "Dùng lại secret hạ tầng đã sinh trước đó tại ${ENV_SECRETS}"
fi

# ── 3. Ghép thành 1 file .env hoàn chỉnh gửi lên VPS ────────────────────────
# Docker Compose (giống mọi dotenv loader chuẩn) đọc env_file TUẦN TỰ và dùng
# giá trị GẶP SAU CÙNG khi 1 key bị lặp — nên phải ghi .env.production (có thể
# còn sót key kiểu dev như PG_CONNECTION_STRING/NODE_ENV) TRƯỚC, rồi secrets hạ
# tầng, rồi override production CỐ ĐỊNH ở CUỐI CÙNG để chắc chắn thắng.
TMP_ENV="$(mktemp)"
trap 'rm -f "${TMP_ENV}"' EXIT

POSTGRES_PASSWORD="$(grep '^POSTGRES_PASSWORD=' "${ENV_SECRETS}" | cut -d= -f2-)"

{
  cat "${ENV_PROD}"
  echo ""
  cat "${ENV_SECRETS}"
  echo ""
  echo "# ── Override production cố định, đặt CUỐI để luôn thắng (Docker Compose"
  echo "# env_file: key lặp lấy giá trị gặp SAU CÙNG) — không sửa tay trên VPS ──"
  echo "NODE_ENV=production"
  echo "AGENT_BACKEND=go"
  echo "PG_CONNECTION_STRING=postgresql://jarvis:${POSTGRES_PASSWORD}@postgres:5432/ai_agent_tut"
  echo "REDIS_URL=redis://redis:6379"
  echo "S3_ENDPOINT=http://minio:9000"
} > "${TMP_ENV}"

# ── 3b. Xác nhận trước khi động vào production ──────────────────────────────
# MONGODB_URI KHÔNG được script này tự đổi (Mongo Atlas không tự tạo cluster/DB
# mới qua đây) — nếu ${ENV_PROD} copy từ env/.env dev mà chưa sửa, deploy sẽ
# ghi đè NHẦM vào đúng DB dev đang dùng. In lại các giá trị quan trọng để tự
# kiểm tra bằng mắt trước khi tiếp tục.
MONGO_URI_MASKED=$(grep '^MONGODB_URI=' "${TMP_ENV}" | sed -E 's#(mongodb\+?srv?://[^:]+:)[^@]+(@)#\1****\2#' || echo "(không tìm thấy MONGODB_URI)")
CORS_LINE=$(grep '^CORS_ORIGIN=' "${TMP_ENV}" || echo "CORS_ORIGIN=(chưa đặt)")

echo ""
warn "Kiểm tra lại trước khi deploy THẬT lên ${SSH_HOST}:${REMOTE_DIR}:"
echo "    ${MONGO_URI_MASKED}"
echo "    ${CORS_LINE}"
echo "    Public port: ${WEB_HOST_PORT}"
warn "MONGODB_URI có đang trỏ ĐÚNG database production (không phải DB dev) không?"
read -rp "$(echo -e "${CYAN}Tiếp tục deploy? (y/N):${NC} ")" CONFIRM
if [[ "${CONFIRM}" != "y" && "${CONFIRM}" != "Y" ]]; then
  err "Huỷ deploy. Sửa ${ENV_PROD} rồi chạy lại."
  exit 1
fi

# ── 4. Kiểm tra SSH kết nối được trước khi rsync ─────────────────────────────
info "Kiểm tra kết nối SSH tới ${SSH_HOST}..."
if ! ssh -o BatchMode=yes -o ConnectTimeout=10 "${SSH_HOST}" "echo ok" &>/dev/null; then
  err "Không kết nối được SSH tới '${SSH_HOST}'. Kiểm tra ~/.ssh/config."
  exit 1
fi
log "SSH tới ${SSH_HOST} OK"

# ── 4b. Bootstrap VPS từ xa (idempotent — an toàn chạy lại mỗi lần deploy) ──
# Không cần bạn tự scp/ssh chạy deploy/setup-vps.sh tay nữa (dễ lỡ chạy nhầm
# trên máy local). SSH host "${SSH_HOST}" đã đăng nhập thẳng bằng root (xem
# ~/.ssh/config), nên không cần sudo/mật khẩu ở bước này.
info "Bootstrap VPS (tạo thư mục, mở firewall, cron backup — bỏ qua nếu đã có)..."
ssh "${SSH_HOST}" "WEB_HOST_PORT=${WEB_HOST_PORT} bash -s" < deploy/setup-vps.sh
log "Bootstrap VPS xong"

# ── 5. Rsync code (không kèm .git, node_modules, env thật) ──────────────────
info "Đồng bộ code lên ${SSH_HOST}:${REMOTE_DIR}..."
rsync -az --delete \
  --exclude '.git' \
  --exclude 'node_modules' \
  --exclude '.turbo' \
  --exclude 'dist' \
  --exclude 'coverage' \
  --exclude 'env/.env*' \
  --exclude '.claude' \
  ./ "${SSH_HOST}:${REMOTE_DIR}/"
log "Đồng bộ code xong"

# ── 6. Gửi .env đã ghép riêng (không qua rsync exclude ở trên) ──────────────
info "Gửi env production lên VPS..."
ssh "${SSH_HOST}" "mkdir -p ${REMOTE_DIR}/env"
scp "${TMP_ENV}" "${SSH_HOST}:${REMOTE_DIR}/env/.env"
ssh "${SSH_HOST}" "chmod 600 ${REMOTE_DIR}/env/.env"
log "Env production đã gửi"

# ── 7. Build + chạy trên VPS ─────────────────────────────────────────────────
info "Build + khởi động container trên VPS (có thể mất vài phút lần đầu)..."
ssh "${SSH_HOST}" "cd ${REMOTE_DIR} && \
  WEB_HOST_PORT=${WEB_HOST_PORT} docker compose -f ${COMPOSE_FILE} build --pull && \
  WEB_HOST_PORT=${WEB_HOST_PORT} docker compose -f ${COMPOSE_FILE} up -d"
log "Container đã khởi động"

# ── 8. Health check ──────────────────────────────────────────────────────────
info "Đợi service khởi động (15s)..."
sleep 15

info "Kiểm tra health..."
SERVICES=("jarvis-postgres" "jarvis-redis" "jarvis-minio" "jarvis-agent-go" "jarvis-api" "jarvis-web")
ALL_HEALTHY=true
for svc in "${SERVICES[@]}"; do
  STATUS=$(ssh "${SSH_HOST}" "docker inspect --format='{{.State.Health.Status}}' ${svc} 2>/dev/null || echo 'no-healthcheck'")
  if [[ "${STATUS}" == "healthy" || "${STATUS}" == "no-healthcheck" ]]; then
    log "${svc}: ${STATUS}"
  else
    warn "${svc}: ${STATUS}"
    ALL_HEALTHY=false
  fi
done

echo ""
if $ALL_HEALTHY; then
  log "Deploy xong! Truy cập: http://<VPS_IP>:${WEB_HOST_PORT}"
else
  warn "Một vài service chưa healthy — xem log:"
  warn "  ssh ${SSH_HOST} 'docker compose -f ${REMOTE_DIR}/${COMPOSE_FILE} logs -f'"
fi
