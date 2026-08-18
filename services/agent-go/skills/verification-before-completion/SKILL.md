---
name: verification-before-completion
description: Never claim something works without running fresh verification and showing the evidence
when_to_use: Before declaring work done, fixed, passing, or deployed — and before commit/PR
triggers: [đã xong chưa, da xong chua, kiểm tra lại, kiem tra lai, verify, xác nhận, xac nhan, chạy thử, chay thu, đã chạy được chưa, da chay duoc chua, test lại, test lai, có chắc, co chac]
tools: [shell.exec, file.read]
---

# Verification Before Completion

## Luật sắt

```
KHÔNG TUYÊN BỐ XONG KHI CHƯA CÓ BẰNG CHỨNG VỪA CHẠY
```

"Chắc là chạy được" không phải bằng chứng. Bằng chứng là output của một lệnh vừa
chạy xong.

## Quy trình bắt buộc, 5 bước

1. **Xác định** lệnh nào chứng minh được điều mình định nói.
2. **Chạy** nó, chạy trọn vẹn và chạy MỚI (không dùng kết quả cũ, không dùng cache).
3. **Đọc** toàn bộ output và exit code — không đọc lướt vài dòng đầu.
4. **Đối chiếu** output có thật sự chứng minh điều mình định nói không.
5. **Chỉ khi đó** mới nói, và nói kèm bằng chứng.

Bỏ bất kỳ bước nào thì phát biểu đó là phỏng đoán được trình bày như sự thật.

## Áp dụng trước khi

- Nói "xong", "đã sửa", "đã pass", "đã deploy".
- Nói những câu tự tin kiểu "chạy tốt rồi", "ổn rồi".
- Commit, mở PR, đóng task.
- Báo lại kết quả cho user sau khi giao việc cho công cụ/agent khác.

## Dấu hiệu đang vi phạm

- Dùng từ nước đôi: "chắc là", "có lẽ", "should work".
- Kết luận dựa vào việc mình đã đọc code, không dựa vào việc chạy.
- Kiểm một phần rồi suy ra toàn bộ ("test A pass nên chắc B cũng pass").
- Tin báo cáo của công cụ khác mà không tự xác nhận.

## Bằng chứng theo loại việc

| Loại việc | Bằng chứng cần có |
|---|---|
| Test | Chạy cả suite, đếm số pass/fail thật, không chỉ nói "test pass" |
| Sửa bug | Chứng minh đỏ-trước / xanh-sau, không chỉ xanh-sau |
| Build | Exit code 0, không chỉ "lint không báo gì" |
| Đúng yêu cầu | Đối chiếu từng dòng yêu cầu với output thật |
| Sửa UI | Xem lại bằng mắt sau khi reload, cẩn thận dev server còn cache bản cũ |

## Khi không thể xác nhận

Nếu không chạy được lệnh xác nhận (thiếu quyền, thiếu môi trường, thiếu dữ liệu)
thì **nói rõ là chưa xác nhận được và thiếu gì**, thay vì nói xong. Trình bày
phỏng đoán như sự thật gây thiệt hại lớn hơn nhiều so với việc nói "tôi chưa kiểm
được phần này".

---

Adapt từ [obra/superpowers](https://github.com/obra/superpowers) — skill
`verification-before-completion` (MIT License).
