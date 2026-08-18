#!/usr/bin/env bash
# ═══════════════════════════════════════════════════════════════════════════════
# J.A.R.V.I.S. — Automated Disk & RAM Optimizer
# ═══════════════════════════════════════════════════════════════════════════════
# Runs periodically (every 30 mins) to:
#  1. Prune dangling Docker images, stopped containers, build caches
#  2. Truncate oversized local docker json logs (>50MB)
#  3. Vacuum systemd journal logs
#  4. Safely flush & drop Linux memory page caches
# ═══════════════════════════════════════════════════════════════════════════════

set -euo pipefail

TIMESTAMP=$(date "+%Y-%m-%d %H:%M:%S")
echo "════════════════════════════════════════════════════════════"
echo "[$TIMESTAMP] Starting VPS Disk & RAM Optimization..."

# ── 1. Snapshot Before ──
RAM_BEFORE=$(free -h | awk '/^Mem:/ {print "Used: " $3 " / Total: " $2}')
DISK_BEFORE=$(df -h / | awk 'NR==2 {print "Used: " $3 " / Avail: " $4 " (" $5 ")"}')
echo "→ Before: RAM [$RAM_BEFORE] | Disk [$DISK_BEFORE]"

# ── 2. Docker Disk Cleanup ──
echo "→ Pruning stopped containers & dangling images..."
docker container prune -f >/dev/null 2>&1 || true
docker image prune -f >/dev/null 2>&1 || true

# Prune builder cache older than 24 hours (keep last 1GB)
docker builder prune -f --keep-storage 1GB --filter "until=24h" >/dev/null 2>&1 || true

# ── 3. Truncate local Docker json logs (>50MB) ──
# (VictoriaLogs stores long-term history, so local json logs can be kept small)
if [ -d "/var/lib/docker/containers" ]; then
  find /var/lib/docker/containers/ -type f -name "*-json.log" -size +50M -exec truncate -s 10M {} + 2>/dev/null || true
fi

# ── 4. System Logs & Cache Cleanup ──
journalctl --vacuum-size=100M --vacuum-time=3d >/dev/null 2>&1 || true
apt-get clean >/dev/null 2>&1 || true

# ── 5. RAM Cache Optimization ──
# Safely sync writes to disk, then free up clean page caches
sync
echo 3 > /proc/sys/vm/drop_caches 2>/dev/null || true

# ── 6. Snapshot After ──
RAM_AFTER=$(free -h | awk '/^Mem:/ {print "Used: " $3 " / Total: " $2}')
DISK_AFTER=$(df -h / | awk 'NR==2 {print "Used: " $3 " / Avail: " $4 " (" $5 ")"}')
echo "→ After:  RAM [$RAM_AFTER] | Disk [$DISK_AFTER]"
echo "[$TIMESTAMP] VPS Optimization completed successfully."
echo ""
