---
name: debug
description: Systematic debugging: root cause first, pattern analysis, one hypothesis at a time, verify
when_to_use: When user reports a bug, error, crash, or unexpected behavior
triggers: [debug, sửa lỗi, sua loi, bị lỗi, bi loi, báo lỗi, bao loi, không chạy, khong chay, crash, stack trace, tại sao lỗi, tai sao loi]
tools: [shell.exec, file.read, git.log, git.diff]
---

# Debugging Skill

## Luật sắt

```
KHÔNG SỬA GÌ TRƯỚC KHI TÌM RA ROOT CAUSE
```

Đoán rồi sửa thử là cách chắc chắn nhất để vá triệu chứng và để lại bug thật.

## 1. Reproduce

- Hỏi user: bước cụ thể, input, môi trường.
- Tự dựng lại bug để xác nhận nó tồn tại.
- Ghi rõ: hành vi mong đợi vs hành vi thực tế.
- Đọc HẾT thông báo lỗi, gồm cả stack trace — phần lớn câu trả lời nằm ở đó.

## 2. Isolate

- Thu hẹp phạm vi: component/module/hàm nào chịu trách nhiệm?
- `git.log` xem thay đổi gần đây có thể gây ra bug.
- `git.diff` xem file nghi vấn đã đổi gì.
- Dựng minimal reproduction.

## 3. Pattern Analysis — so với chỗ ĐANG CHẠY ĐÚNG

Bước hay bị bỏ nhất, và thường là bước tìm ra nguyên nhân nhanh nhất:

- Tìm code tương tự trong dự án mà **đang hoạt động đúng**.
- Đọc trọn vẹn bản chạy đúng đó (đọc lướt là bỏ sót).
- Liệt kê MỌI khác biệt giữa bản chạy đúng và bản lỗi.
- Xét cả khác biệt về môi trường/dependency, không chỉ khác biệt trong code.

Bug thường nằm ở đúng một trong những khác biệt đó.

## 4. Hypothesis — mỗi lần một giả thuyết

- Nêu MỘT giả thuyết cụ thể kèm lý do.
- Kiểm chứng bằng thay đổi NHỎ NHẤT có thể.
- `shell.exec` chạy test/thêm log; `file.read` đọc code quanh chỗ nghi.
- Xác nhận kết quả trước khi đi tiếp. Sai thì nêu giả thuyết mới, không sửa bừa thêm.
- Đổi mỗi lần một biến. Không tung nhiều fix cùng lúc.

## 5. Fix

- Thay đổi nhỏ nhất chạm đúng root cause.
- KHÔNG refactor code không liên quan trong lúc sửa.
- Thêm test dựng lại bug để chống tái phát.

## 6. Verify

- Chạy test suite: `shell.exec` lệnh test.
- Xác nhận bước reproduce ban đầu giờ cho kết quả đúng.
- Kiểm tra tác dụng phụ: fix có làm vỡ chỗ khác không?

## Quy tắc 3 lần

Ba lần fix mà vẫn không xong thì **dừng sửa**. Đó là tín hiệu vấn đề nằm ở kiến
trúc/giả định chứ không ở dòng code đang sửa. Nói thẳng với user điều đó và bàn
lại hướng, thay vì thử lần thứ tư.

## Anti-pattern

- Đoán nguyên nhân mà không có bằng chứng.
- Đổi nhiều thứ một lúc ("shotgun debugging").
- Sửa triệu chứng thay vì root cause.
- Bỏ test — chưa test thì chưa sửa xong.
- Nói "chắc là do..." rồi kết luận: chưa chạy lại thì chưa biết.

## Giao tiếp

- Cập nhật user từng bước: "Đã dựng lại được bug. Đang khoanh vùng..."
- Tìm ra root cause thì giải thích rõ ràng.
- Sửa xong thì tóm lại: sai ở đâu, đã đổi gì, vì sao.

---

Bước "Pattern Analysis", "Luật sắt" và "Quy tắc 3 lần" lấy ý từ
[obra/superpowers](https://github.com/obra/superpowers) — skill
`systematic-debugging` (MIT License).
