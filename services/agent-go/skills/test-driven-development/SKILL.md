---
name: test-driven-development
description: Write the failing test first, watch it fail, then write the minimum code to pass
when_to_use: When the user asks to write tests, do TDD, add a feature test-first, or fix a bug with a regression test
triggers: [tdd, test driven, viết test trước, viet test truoc, red green refactor, unit test, viết unit test, viet unit test, thêm test, them test, test case, regression test, kiểm thử, kiem thu]
tools: [shell.exec, file.read, file.write]
---

# Test-Driven Development

## Luật sắt

```
KHÔNG CÓ CODE PRODUCTION NÀO TRƯỚC KHI CÓ MỘT TEST ĐANG ĐỎ
```

Code viết trước test thì xoá, không giữ lại làm "bản tham khảo" rồi sửa dần —
giữ lại là mất luôn tác dụng của TDD (test viết sau sẽ được uốn cho vừa code có
sẵn, kể cả khi code đó sai).

## Vòng RED → GREEN → REFACTOR

### RED — viết test thất bại

- Viết MỘT test nhỏ nhất diễn tả hành vi muốn có.
- Tên test nói rõ hành vi, không phải tên hàm.
- **Bắt buộc chạy test và xem nó đỏ.** Chưa thấy đỏ thì chưa biết test có thật
  sự kiểm tra điều gì.
- Đỏ phải vì lý do ĐÚNG (thiếu tính năng), không phải vì typo hay import sai.

### GREEN — code tối thiểu cho test xanh

- Viết đoạn code đơn giản nhất làm test đi qua.
- Không thêm tính năng chưa có test. Không tối ưu sớm.
- **Bắt buộc chạy lại:** test này xanh VÀ các test cũ vẫn xanh.

### REFACTOR — dọn khi đang xanh

- Bỏ trùng lặp, đặt lại tên cho rõ.
- Không thêm hành vi mới ở bước này.
- Chạy test sau mỗi lần dọn.

## Khi nào dùng

**Luôn:** tính năng mới, sửa bug, refactor, đổi hành vi.

**Ngoại lệ (phải hỏi user trước):** prototype dùng một lần, code sinh tự động,
file cấu hình.

## Test tốt

- **Nhỏ:** mỗi test kiểm một điều.
- **Rõ:** đọc tên test là biết nó bảo đảm gì.
- **Thật:** assert trên hành vi của code thật, không assert trên hành vi của mock.
  Test chỉ chứng minh mock hoạt động là test vô nghĩa.

## Dấu hiệu đã chệch khỏi TDD

Gặp bất kỳ dấu hiệu nào dưới đây thì xoá code vừa viết và làm lại từ RED:

- Viết code trước, test sau.
- Test vừa viết đã xanh ngay — nó không kiểm tra gì cả.
- Tự nhủ "lần này thôi, đơn giản mà".
- Lấy việc chạy thử bằng tay để thay cho test tự động.

## Bug thì bắt đầu bằng test tái hiện

Sửa bug: viết test dựng lại bug (đỏ) → sửa (xanh). Test đó chính là bằng chứng
bug đã hết và là hàng rào chống tái phát.

## Checklist trước khi nói xong

- Mỗi hành vi mới đều có test đã từng đỏ trước đó.
- Mọi test đỏ đúng lý do.
- Code viết ra là tối thiểu cho từng test.
- Toàn bộ suite xanh, output sạch.
- Đã phủ ca biên (rỗng, null, lỗi, giá trị lớn).

Không tick đủ nghĩa là đã bỏ bước — làm lại, đừng khai là xong.

---

Adapt từ [obra/superpowers](https://github.com/obra/superpowers) — skill
`test-driven-development` (MIT License).
