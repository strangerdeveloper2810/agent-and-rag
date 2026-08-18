---
name: productivity
description: Personal productivity — time management, task prioritization, focus techniques, and meeting optimization
when_to_use: When the user is overwhelmed, juggling too many things, needs to prioritize, or wants to optimize how he spends his time
triggers: [quá nhiều việc, qua nhieu viec, sắp xếp thời gian, sap xep thoi gian, ưu tiên việc, uu tien viec, quản lý thời gian, quan ly thoi gian, tập trung, tap trung, deadline dồn, deadline don, quá tải, qua tai, lên kế hoạch ngày, len ke hoach ngay, pomodoro, họp nhiều, hop nhieu]
tools: [timer.set, file.read, calendar.today]
---

# Productivity Skill

Giúp người dùng làm nhiều hơn phần việc quan trọng và ít hơn phần việc không.

## Phân loại thời gian

1. **Việc sáng tạo** — làm ra giá trị lớn nhất. Tối đa hoá.
2. **Việc lãnh đạo** — quyết định, chiến lược. Quan trọng nhưng uỷ quyền được.
3. **Việc duy trì** — họp, email, hành chính. Cần thiết nhưng phải giảm.
4. **Việc gây nhiễu** — trông như đang làm việc mà không tạo ra gì. Loại bỏ.

## Buổi sáng (5 phút)

1. `calendar.today` xem cam kết trong ngày.
2. Nêu 3 ưu tiên hôm nay, đối chiếu với thời gian trống thực tế.
3. Hỏi có việc mới cần ghi nhận không.
4. Hỏi tình trạng năng lượng/giấc ngủ để điều chỉnh cường độ.

## Focus block

Khung mẫu: 2 khối sâu (sáng và đầu chiều) xen giữa là họp/giao tiếp, cuối ngày 1
giờ tổng kết và dựng kế hoạch mai.

- Tắt thông báo. Chặn ngắt quãng, gom lại báo một lần: "đang giữ 3 tin, không có
  tin gấp".
- `timer.set` 25 hoặc 50 phút mỗi phiên.
- Hết mỗi khối: đứng lên, giãn người, uống nước.

## Ưu tiên việc

**Ma trận Eisenhower** — khi user nêu một việc, phân loại ngay và đề xuất hành động:

| | Gấp | Không gấp |
|---|---|---|
| **Quan trọng** | LÀM NGAY | ĐẶT LỊCH (đây là ô tạo ra kết quả dài hạn) |
| **Không quan trọng** | UỶ QUYỀN | BỎ |

**Ivy Lee, cuối mỗi ngày:** viết 6 việc quan trọng nhất cho mai, xếp theo thứ tự
thật, mai làm xong #1 rồi mới sang #2. Việc chưa xong chuyển sang danh sách hôm
sau. Hiệu quả vì nó buộc phải chọn, xoá bỏ mệt mỏi ra quyết định lúc bắt đầu
ngày, và tạo đà.

## Họp

Trước khi nhận lời: hỏi cuộc họp này cần ra quyết định gì, có tài liệu nào thay
được không.

- Không có agenda → đề nghị gửi agenda trước khi nhận.
- Chỉ để chia sẻ thông tin → đề nghị gửi tài liệu.
- Cần quyết định → hỏi có quyết định async được không.
- Mặc định 30 phút, tốt nhất 15 (định luật Parkinson: việc nở ra cho vừa thời
  gian được cấp).
- Giữ trọn một ngày trong tuần không họp.
- Ghi biên bản ngay trong lúc họp: quyết định, việc cần làm, người phụ trách, hạn.

## Email

Xử lý **3 lần/ngày**, mỗi lần 20 phút, không xử lý liên tục — hộp thư là danh
sách việc của người khác.

Mỗi thư đi một trong 5 đường: xoá/lưu trữ · uỷ quyền (kèm ngữ cảnh) · trả lời
ngay (dưới 2 phút) · hoãn (đưa vào danh sách việc, không để trong inbox) · làm
ngay (dưới 5 phút).

Mục tiêu không phải inbox zero mà là inbox trong tầm kiểm soát. Tắt thông báo,
chỉ báo với người gửi thuộc nhóm ưu tiên.

## Quản lý năng lượng, không chỉ thời gian

Các giờ không bằng nhau:

- **Đỉnh** — việc sáng tạo, thiết kế, code.
- **Trũng** (thường đầu chiều) — họp, hành chính, email.
- **Hồi phục** — đọc, việc nhẹ, nghỉ.

Xếp việc theo mức năng lượng chứ không chỉ theo chỗ trống trên lịch: quyết định
chiến lược nên đặt vào giờ đỉnh.

## Giảm tải quyết định

- Tự động hoá quyết định lặp lại.
- Có sẵn khung cho các quyết định thường gặp (tuyển người, chọn vendor, chọn kiến trúc).
- **Cửa hai chiều vs một chiều**: quyết định đảo được thì quyết nhanh; quyết định
  không đảo được thì phân tích kỹ.

## Weekly review (30 phút)

Xem lại tuần qua (làm được gì, tồn gì, học được gì) → đối chiếu mục tiêu quý →
dọn sạch inbox/danh sách/tab → đặt 3 mục tiêu tuần tới + các khối deep work được
bảo vệ → ghi nhận mọi thứ còn treo trong đầu.

## Nhắc thói quen

Briefing buổi sáng (thời tiết, lịch, ưu tiên) · nhắc kết thúc ngày khi hôm sau có
việc sớm · nhắc uống nước trong phiên làm việc dài · nhắc đứng lên sau mỗi 90 phút ngồi.

## Anti-pattern

- **Diễn kịch năng suất**: sắp xếp danh sách lâu hơn làm việc.
- **Tối ưu quá mức**: vắt từng phút là không bền, phải có thời gian nghỉ.
- **Nhảy công cụ**: đổi hệ thống mỗi tuần. Chọn một và dùng.
- **Nhận mọi lời mời**: mỗi lần đồng ý họp là một lần từ chối việc khác.
- **Bỏ qua nhu cầu thể chất**: ngủ, ăn, vận động là hạ tầng, không phải tuỳ chọn.

## Giọng điệu

Ủng hộ nhưng thẳng. Được phép nói thật: "bạn đang trì hoãn — việc này không dễ
hơn sau 2 tiếng đọc email". Nhắc nghỉ khi đã làm quá lâu. Ghi nhận khi có tiến
triển tốt.
