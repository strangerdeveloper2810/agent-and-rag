---
name: security-audit
description: Security review — find vulnerabilities, check OWASP Top 10, review auth/encryption/input validation
when_to_use: When the user needs a security assessment: code review for vulnerabilities, architecture security review, or pre-deployment audit
triggers: [bảo mật, bao mat, security, lỗ hổng, lo hong, vulnerability, kiểm tra an toàn, kiem tra an toan, owasp]
tools: [file.read, shell.exec, git]
---

# Security Audit Skill

## Trước khi audit

Xác định: audit cái gì (file/module/service/toàn hệ thống) · threat model (ai tấn
công, họ muốn gì) · độ nhạy dữ liệu (PII? tài chính?) · `git log`/`git diff` để
tập trung vào phần mới thay đổi.

## Checklist

**1. Auth & phân quyền**
- [ ] Auth áp lên MỌI endpoint, không chỉ endpoint hiển nhiên
- [ ] Không có credential/API key/token hardcode (`grep -rE '(password|secret|api_key|token)\s*=\s*["'"'"']'`)
- [ ] RBAC chặt: user quyền thấp không leo thang được
- [ ] Session/token bị vô hiệu khi logout
- [ ] Không có đường bypass: debug endpoint, route nội bộ bị hở, tài khoản admin mặc định

**2. Input & injection**
- [ ] SQL: mọi query đều tham số hoá, không nối chuỗi
- [ ] Command injection: input người dùng không đi vào shell
- [ ] XSS: nội dung do user tạo được escape khi xuất
- [ ] Path traversal: đường dẫn dựng từ input phải được làm sạch
- [ ] Validate giống nhau ở client và server; có giới hạn kích thước input (chống DoS)

**3. Mã hoá**
- [ ] Thuật toán hiện đại: AES-256-GCM (không ECB), bcrypt/argon2 cho mật khẩu (không MD5/SHA1)
- [ ] Key không nằm trong source, không vào log
- [ ] TLS cấu hình đúng, bật HSTS
- [ ] Random dùng `crypto/rand`, **không** `math/rand`
- [ ] JWT verify đủ: signature, expiration, issuer, audience

**4. Access control**
- [ ] IDOR: đổi ID có đọc được dữ liệu người khác không?
- [ ] Least privilege
- [ ] Endpoint admin có chặn user thường không?
- [ ] CORS: không `Allow-Origin: *` kèm credentials

**5. Cấu hình**
- [ ] Đã đổi credential mặc định
- [ ] Production không trả stack trace cho user
- [ ] Tắt tính năng/port/service không dùng
- [ ] Có security header: CSP, X-Frame-Options, nosniff, Referrer-Policy
- [ ] Tắt directory listing

**6. Lộ dữ liệu**
- [ ] Secret lấy từ env/secret manager
- [ ] Log không chứa mật khẩu, token, PII
- [ ] Dữ liệu cần thì được mã hoá khi lưu
- [ ] Response chỉ trả đúng thứ client cần

**7. Dependency**
- [ ] Quét CVE (`govulncheck ./...`), cờ package trễ hơn 6 tháng, phát hiện package bị bỏ rơi

**8. SSRF**
- [ ] App có gọi ra URL do user cung cấp? Có allowlist? Tài nguyên mạng nội bộ có được che?

**9. Log & monitoring**
- [ ] Ghi log mọi lần đăng nhập (cả thành công và thất bại) và hành động nhạy cảm
- [ ] Có rate limit / phát hiện bất thường; log chống bị sửa

**10. Lỗi logic nghiệp vụ**
- [ ] Bỏ qua bước được không (skip thanh toán, skip phê duyệt)?
- [ ] Race condition (double-spend, dùng lại coupon)?
- [ ] Lách được hạn mức (rate limit, quota, số lượng âm)?

## Riêng cho Go

- [ ] Error được kiểm, không `_` cho giá trị lỗi
- [ ] `go test -race`
- [ ] `unsafe.Pointer` phải có lý do
- [ ] Xuất HTML dùng `html/template`, không `text/template`
- [ ] Quyền file mới tạo hợp lý
- [ ] Context được truyền và tôn trọng; goroutine không leak; HTTP handler có recover panic

## Báo cáo

Mỗi finding gồm: **ID** · **severity** · **CWE** · **vị trí** `file.go:42` ·
**nhóm OWASP** · mô tả cách khai thác · PoC nếu có · tác động · cách sửa cụ thể ·
cách xác nhận đã sửa. Kết thúc báo cáo bằng bảng tổng hợp theo severity, phần
**làm tốt**, và khuyến nghị dài hạn.

| Severity | Tiêu chí |
|---|---|
| Critical | Chiếm được hệ thống, RCE, lấy sạch dữ liệu, không cần auth |
| High | Bypass auth, leo thang quyền, lộ dữ liệu lớn, lỗi mã hoá |
| Medium | Lộ thông tin, cấu hình sai tác động hạn chế, thiếu security header |
| Low | Vi phạm best practice nhưng chưa khai thác được |
| Info | Quan sát, gợi ý hardening |

## Anti-pattern

- **Hổ giấy**: chỉ tìm lỗi nhẹ cho trông có vẻ kỹ. Đào sâu hơn.
- **Thổi phồng**: gọi CRITICAL cho lỗi cần truy cập vật lý vào server. Đúng mức.
- **Audit không ngữ cảnh**: rủi ro thấp ở tool nội bộ có thể là critical ở service
  public. Luôn xét ngữ cảnh.
- **Bỏ qua điểm tốt**: nêu cả những gì đã làm đúng.
- **Lý thuyết suông**: luôn giải thích kẻ tấn công khai thác BẰNG CÁCH NÀO, không
  chỉ nói là có lỗ hổng.
