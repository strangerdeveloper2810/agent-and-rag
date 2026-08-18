#!/usr/bin/env bash
# Coverage ratchet cho agent-go.
#
# CỐ TÌNH không gate toàn repo: hiện tổng ~81% statements (internal/mongo 40.9%,
# internal/tools 76.3%), nên một ngưỡng global 90% chỉ làm CI đỏ vĩnh viễn.
# Thay vào đó gate theo TỪNG PACKAGE ở mức đã đạt được — coverage chỉ đi lên,
# không tụt lại. Thêm package vào bảng dưới khi nó đã có test tử tế.
#
# Chạy: ./scripts/coverage-gate.sh [coverage.out]
# Không truyền file thì script tự chạy `go test` để sinh profile.
set -euo pipefail

cd "$(dirname "$0")/.."

# package<TAB>ngưỡng tối thiểu (%)
#
# internal/memory CẦN MongoDB để đạt ngưỡng: saveFactToMongo /
# saveKnowledgeItemToMongo / LoadFromMongo là I/O thuần, không fake được (
# mongo.Client chỉ dựng qua Connect() có ping ngay lúc tạo). Không có
# MONGODB_TEST_URI thì các test đó skip và coverage tụt xuống ~88%.
#
#	docker run -d --rm --name jarvis-test-mongo -p 27117:27017 mongo:7
#	MONGODB_TEST_URI=mongodb://localhost:27117 ./scripts/coverage-gate.sh
GATES=$(
	cat <<'EOF'
internal/agent	95
internal/memory	95
internal/provider/fallback	95
internal/skills	95
EOF
)

PROFILE="${1:-}"
if [[ -z "$PROFILE" ]]; then
	PROFILE=$(mktemp -t agentgo-cover)
	trap 'rm -f "$PROFILE"' EXIT
	pkgs=$(echo "$GATES" | cut -f1 | sed 's|^|./|' | tr '\n' ' ')
	# shellcheck disable=SC2086
	go test -coverprofile="$PROFILE" $pkgs >/dev/null
fi

failed=0
while IFS=$'\t' read -r pkg minimum; do
	[[ -z "$pkg" ]] && continue

	# Lọc dòng của package rồi tính tỉ lệ statement đã chạy từ chính profile —
	# `go tool cover -func` chỉ tổng hợp theo file/hàm nên không có số per-package.
	pct=$(awk -F'[: ]' -v p="$pkg/" '
		$0 ~ p {
			n = split($0, parts, " ")
			stmts = parts[n-1]
			count = parts[n]
			total += stmts
			if (count > 0) covered += stmts
		}
		END {
			if (total == 0) { print "NA"; exit }
			printf "%.1f", covered * 100 / total
		}
	' "$PROFILE")

	if [[ "$pct" == "NA" ]]; then
		echo "❌ $pkg: không tìm thấy dữ liệu coverage trong profile"
		failed=1
		continue
	fi

	if awk -v a="$pct" -v b="$minimum" 'BEGIN { exit !(a + 0 < b + 0) }'; then
		echo "❌ $pkg: $pct% < ngưỡng $minimum%"
		failed=1
	else
		echo "✅ $pkg: $pct% (ngưỡng $minimum%)"
	fi
done <<<"$GATES"

exit "$failed"
