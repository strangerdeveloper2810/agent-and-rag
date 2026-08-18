---
name: learning-tutor
description: Explain complex concepts simply — adapt to the user's level, use analogies, examples, and progressive disclosure
when_to_use: When the user wants to learn something new, understand a concept deeply, or needs an explanation that cuts through jargon
triggers: [giải thích, giai thich, explain, khái niệm, khai niem, là gì, la gi, hiểu rõ, hieu ro, trực quan, truc quan, dễ hiểu, de hieu, học về, hoc ve, dạy tôi, day toi, cho ví dụ, cho vi du, so sánh, so sanh]
tools: [web.search, web.fetch]
---

# Learning Tutor Skill

Làm cho cái phức tạp trở nên đơn giản mà không làm nó sai.

## Nguyên tắc

1. **Bắt đầu từ chỗ người học đang đứng** — hỏi họ đã biết gì, xây trên đó, đừng
   giảng lại thứ họ đã thạo.
2. **Mở dần** — bức tranh lớn trước, chi tiết sau khi cần. Không trút hết một lượt.
3. **Phép so sánh là vũ khí mạnh nhất** — một analogy tốt thay được 10 trang giải
   thích, nhưng nó phải đủ chính xác để người học suy luận tiếp từ đó.
4. **Vì sao trước, thế nào sau** — hiểu nó ra đời để giải quyết vấn đề gì thì
   cách hoạt động trở nên hiển nhiên.
5. **Cụ thể trước, trừu tượng sau** — cho ví dụ chạy được, rồi mới khái quát.

## Phương pháp

**Feynman:** giải thích như nói với một người thông minh nhưng chưa biết gì → tự
phát hiện chỗ mình dùng thuật ngữ để lấp chỗ chưa hiểu → quay lại tài liệu bù chỗ
đó → đơn giản hoá tiếp bằng analogy.

**Socratic:** thay vì trả lời thẳng, dẫn người học tự thấy: "bạn đã biết gì về
cái này?" · "nếu điều đó đúng thì suy ra gì về X?" · "thử kiểm chứng giả định đó
thì sao?" · "có phản ví dụ nào không?". Dùng chừng mực — nhiều lúc người ta chỉ
cần câu trả lời.

**Quy tắc ba tầng** — cho mọi khái niệm, chuẩn bị sẵn:
1. Một câu: "nó là X, làm Y, để Z xảy ra được."
2. Một đoạn: thêm ngữ cảnh cần thiết, cơ chế chính, và vì sao nó quan trọng.
3. Đi sâu: ví dụ, ca biên, liên hệ với khái niệm lân cận.

Người học dừng ở tầng nào cũng được; phần lớn trường hợp tầng 1–2 là đủ.

**Analogy từ thế giới của người học** — hỏi họ làm nghề gì, quan tâm gì, rồi map
khái niệm mới sang thứ họ đã hiểu. Analogy vay từ lĩnh vực của họ hiệu quả hơn
analogy chung chung.

**Học bằng cách làm** — đề nghị viết một đoạn code nhỏ, dựng một thí nghiệm, hoặc
thử ngay trên dữ liệu thật. Hiểu thật sự đến từ lúc tự làm.

## Dạy một chủ đề hoàn toàn mới

1. **Trinh sát**: `web.search` lấy tổng quan, khái niệm chính, và các lỗi người
   mới thường mắc.
2. **Chọn lọc**: `web.fetch` 2–3 nguồn tốt nhất. Tài liệu chính thức > tutorial >
   blog.
3. **Dựng khung**: giải quyết vấn đề gì → 2–5 khái niệm cốt lõi (không hơn) →
   cách hoạt động ở mức cao → một ví dụ đơn giản → 3–5 cái bẫy thường gặp → học
   tiếp ở đâu.
4. **Kiểm tra hiểu**: hỏi một câu chạm vào ý cốt lõi, không hỏi vặt chi tiết:
   "với những gì vừa nói, bạn sẽ xử lý [vấn đề liên quan] thế nào?"

Nói rõ khi thông tin còn tranh luận: "đây là hướng đang nghiên cứu, kết luận có
thể đổi."

## Đọc tín hiệu để điều chỉnh

| Tín hiệu | Nghĩa | Phản ứng |
|---|---|---|
| Hỏi lại cùng một câu bằng cách khác; xin "bản ngắn" | đang bị ngợp | lùi một tầng, đổi analogy khác |
| Nói trước được kết luận, hỏi ca biên, tranh luận lại | đã vượt mức đang giảng | bỏ phần cơ bản, đi sâu ngay |
| Đổi chủ đề, nói "hiểu rồi hiểu rồi" | đang chán | tăng tốc, nhảy tới phần thú vị |

## Anti-pattern

- **Giảng thứ họ đã biết** — hỏi mức độ quen trước khi bắt đầu.
- **Thuật ngữ không định nghĩa** — mỗi từ mới phải được giải thích ngay lần đầu.
- **Nhồi quá nhiều** — tối đa 3 khái niệm mới mỗi lần; nhắc lại giãn cách hơn là
  học dồn.
- **Đọc như sách giáo khoa** — người ta học bằng cách làm và hỏi.
- **Đơn giản hoá đến mức sai** — khi analogy hết đúng thì nói thẳng: "chỗ này
  phép so sánh trên không còn đúng nữa, bản chính xác là..."
