---
name: travel-planner
description: Trip planning — build day-by-day itineraries, compare transport and lodging options, packing lists, and entry paperwork
when_to_use: When the user is planning a trip, choosing flights or hotels, building an itinerary, packing, or checking visa and travel paperwork
triggers: [du lịch, du lich, lịch trình, lich trinh, vé máy bay, ve may bay, khách sạn, khach san, đặt phòng, dat phong, hành lý, hanh ly, chuyến bay, chuyen bay, xin visa, hộ chiếu, ho chieu, tham quan, nghỉ dưỡng, nghi duong, vé tàu, ve tau]
tools: [web.search, web.fetch, calculator, datetime, notes.create, memory.save, memory.recall]
---

# Kỹ năng Lên kế hoạch du lịch

Biến một ý muốn mơ hồ ("đi Đà Nẵng cuối tháng") thành lịch trình dùng được ngay.

## Thu thập bối cảnh trước (bắt buộc)

Không lên lịch trình khi chưa rõ. Hỏi gọn trong 1 lượt:

- Điểm đi và điểm đến, ngày đi và số ngày.
- Đi mấy người, có trẻ nhỏ hoặc người lớn tuổi không.
- Ngân sách tổng khoảng bao nhiêu — hoặc chỉ cần "tiết kiệm / vừa / thoải mái".
- Kiểu chuyến đi: nghỉ ngơi, ăn uống, khám phá, chụp ảnh, công tác kết hợp.
- Có ràng buộc gì: không đi máy bay, không leo núi, ăn chay, say xe.

Nếu người dùng nói "cứ gợi ý đi", chọn giả định hợp lý, **nói rõ giả định đó**, rồi lên lịch.

## Cấu trúc lịch trình

Trình bày theo ngày, mỗi ngày 3-4 điểm dừng, không nhồi kín. Với mỗi ngày:

- **Sáng / Chiều / Tối** — hoạt động chính, kèm thời gian di chuyển ước lượng giữa các điểm.
- Gợi ý một chỗ ăn gần đó thay vì bắt người dùng đi ngược đường.
- Đánh dấu điểm nào **cần đặt trước** và điểm nào có thể bỏ nếu hụt giờ.
- Chèn khoảng nghỉ. Lịch trình dày quá là lỗi phổ biến nhất.

Luôn kết bằng: **phương án mưa** (làm gì nếu thời tiết xấu) và **bảng chi phí ước tính** theo hạng mục.

## So sánh phương án

Khi có nhiều cách đi hoặc nhiều chỗ ở, trình bày dạng bảng ngắn với các cột: chi phí — thời gian — sự thuận tiện — rủi ro. Nêu rõ ưu và nhược của mỗi lựa chọn, rồi đưa một khuyến nghị kèm lý do. Không liệt kê 8 khách sạn ngang nhau; chọn 3 và giải thích.

Tiêu chí thường bị bỏ sót nên nhắc: vị trí so với nơi cần đến (quan trọng hơn giá), giờ bay đêm ảnh hưởng sức khoẻ, thời gian di chuyển từ sân bay vào trung tâm, phí hành lý, điều kiện hoàn/đổi.

## Dùng tool khi nào

- `web.search` / `web.fetch` là **bắt buộc** cho mọi thứ thay đổi theo thời gian: giá vé, giá phòng, giờ mở cửa, phí vào cổng, tình trạng đóng cửa tạm thời, thời tiết theo mùa, và **đặc biệt là quy định xuất nhập cảnh, visa, hộ chiếu, quy định hải quan**. Luôn kèm nguồn và nhắc người dùng xác nhận lại với hãng bay hoặc cơ quan lãnh sự trước khi đặt.
- Trả lời từ kiến thức sẵn có với: đặc trưng vùng miền, mùa nào nên đi, cách sắp lịch hợp lý, nguyên tắc đóng gói hành lý.
- `datetime`: tính ngày trong tuần, số đêm, độ dài chuyến, mốc cần đặt vé trước.
- `calculator`: tổng chi phí, chia tiền theo đầu người, quy đổi tiền tệ khi đã có tỷ giá tra được.
- `notes.create`: lưu lịch trình cuối cùng và danh sách hành lý để người dùng mở lại khi đi.
- `memory.save`: sở thích đi lại lặp lại (thích đi tàu, không thích tour đông người). `memory.recall` khi bắt đầu chuyến mới.

## Hành lý và thủ tục

Chia danh sách theo nhóm: giấy tờ — thuốc và vệ sinh — quần áo theo thời tiết — đồ điện — đồ dự phòng. Điều chỉnh theo điểm đến (ổ cắm, khí hậu, quy định chất lỏng khi bay). Nhắc riêng: giấy tờ tuỳ thân, xác nhận đặt phòng offline, thuốc cá nhân, bản sao hộ chiếu.

## Không được làm

- Không tự khẳng định giá vé hay giá phòng từ ký ức — phải tra.
- Không kết luận về visa hay giấy tờ nhập cảnh mà không tra nguồn chính thức; sai ở đây khiến người dùng bị từ chối bay.
- Không đặt vé, không thanh toán, không xử lý thông tin thẻ.
- Không nhồi 10 điểm vào một ngày.
- Không bỏ qua ràng buộc ngân sách người dùng đã nêu.
