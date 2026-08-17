---
name: document-qa
description: Answer questions grounded in the user's own uploaded documents — enumerate what exists, retrieve relevant passages, quote sources, and flag what the documents do not cover
when_to_use: When the user asks what documents they uploaded, or asks a question that should be answered from their own uploaded files rather than general knowledge
triggers: [trong tài liệu, trong tai lieu, tài liệu tôi, tai lieu toi, tài liệu của tôi, tai lieu cua toi, tài liệu đã upload, tai lieu da upload, tài liệu upload, tai lieu upload, liệt kê tài liệu, liet ke tai lieu, danh sách tài liệu, danh sach tai lieu, theo tài liệu, theo tai lieu, tài liệu nào, tai lieu nao, tôi đã upload, toi da upload, đã tải lên, da tai len]
tools: [rag.list, rag.search, rag.read, memory.recall, memory.save]
---

# Kỹ năng Hỏi đáp trên tài liệu

Trả lời dựa trên **tài liệu chính người dùng đã tải lên**, không dựa vào kiến thức chung. Đây là kỹ năng về sự chính xác và truy nguyên: mỗi khẳng định phải chỉ được ra nó đến từ đâu.

## Chọn tool nào

Ba tool khác nhau hẳn về mục đích — chọn sai là nguyên nhân trả lời thiếu:

- **`rag.list`** — liệt kê **toàn bộ** tài liệu (tên file, số chunk, độ dài), không có nội dung. Dùng khi người dùng hỏi "có những tài liệu gì", "tôi đã upload gì", hoặc khi cần biết phạm vi trước khi tìm. `rag.list` không có tham số: gọi là ra hết.
- **`rag.search`** — tìm theo **ngữ nghĩa**, trả về các đoạn khớp nhất kèm điểm và tên file. Dùng cho mọi câu hỏi về nội dung. Lưu ý: nó chỉ trả top kết quả nên **không bao giờ dùng để liệt kê** — muốn liệt kê thì dùng `rag.list`.
- **`rag.read`** — đọc **toàn bộ** nội dung một tài liệu theo `source` (tên file) hoặc `documentId`. Dùng khi cần bối cảnh đầy đủ: tóm tắt cả tài liệu, so sánh hai tài liệu, hoặc khi đoạn `rag.search` trả về bị cắt ngang ý.

Tài liệu nằm trong cơ sở dữ liệu, **không phải file trên đĩa**. Không có đường dẫn hệ thống nào để đọc.

## Quy trình

1. **Xác định phạm vi.** Nếu câu hỏi mơ hồ về việc "tài liệu nào", gọi `rag.list` trước để biết đang có gì, rồi mới tìm.
2. **Tìm bằng `rag.search`** với truy vấn sát ý người dùng. Nếu kết quả yếu hoặc lệch, **thử lại 2-3 lần với cách diễn đạt khác** — đồng nghĩa, thuật ngữ chuyên môn, và cả tiếng Anh nếu tài liệu có thể là tiếng Anh. Đừng bỏ cuộc sau một lượt tìm.
3. **Đọc đủ trước khi kết luận.** Nếu đoạn trích không đủ để trả lời chắc chắn, gọi `rag.read` trên tài liệu liên quan nhất.
4. **Trả lời kèm nguồn.** Mỗi ý chính ghi rõ đến từ tài liệu nào. Trích nguyên văn câu quan trọng thay vì diễn giải lại, nhất là với số liệu, điều khoản, ngày tháng, tên riêng.
5. **Nói rõ khoảng trống.** Nếu tài liệu chỉ trả lời được một phần, tách bạch: phần này có trong tài liệu, phần kia không có.

## Nguyên tắc bám nguồn

- **Không trộn kiến thức chung vào câu trả lời mà không đánh dấu.** Nếu cần bổ sung ngoài tài liệu, tách thành một đoạn riêng và nói rõ đây là kiến thức chung, không phải từ tài liệu người dùng.
- **Không suy diễn quá xa** khỏi những gì tài liệu nói. Tài liệu nói A, đừng kết luận B chỉ vì thường thì A dẫn tới B.
- **Khi tài liệu tự mâu thuẫn** (hai bản khác phiên bản chẳng hạn): nêu cả hai, chỉ ra chúng khác nhau ở đâu, và nếu có dấu hiệu về thời điểm thì nêu ra. Không tự chọn một bên rồi im lặng.
- **Khi không tìm thấy gì**: nói thẳng là tài liệu hiện có không đề cập, cho biết mình đã tìm bằng những từ khoá nào, và liệt kê tên các tài liệu đang có để người dùng biết cần upload thêm gì. Không bịa nội dung cho có.

## Trình bày

- **Câu hỏi cụ thể**: trả lời trực tiếp trước, dẫn chứng sau.
- **Tóm tắt một tài liệu**: `rag.read` toàn bộ, rồi tóm theo cấu trúc — nội dung chính, điểm quan trọng, chỗ cần chú ý. Giữ nguyên thuật ngữ tài liệu dùng.
- **Liệt kê**: `rag.list` rồi trình bày danh sách tên tài liệu kèm độ lớn.

Dùng `memory.recall` khi tiếp nối chủ đề tài liệu đã bàn trước đó; `memory.save` cho kết luận ổn định rút ra từ tài liệu, kèm tên nguồn.

## Không được làm

- Không dùng `rag.search` để cố liệt kê đủ tài liệu — sai công cụ, sẽ thiếu.
- Không trả lời từ kiến thức chung khi người dùng đã chỉ rõ "trong tài liệu của tôi".
- Không bịa tên tài liệu, số trang, hay câu trích dẫn.
- Không khẳng định "tài liệu không có" khi chỉ mới tìm một lần bằng một từ khoá.
- Không tóm tắt dài hơn mức cần. Người dùng hỏi một chi tiết thì trả chi tiết đó.
