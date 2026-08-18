---
name: data-analysis
description: Data analysis — clean, explore, find patterns, test hypotheses, and turn numbers into actionable insight
when_to_use: When the user has data to analyse, wants statistics explained, needs pattern discovery, or asks what the numbers mean
triggers: [phân tích dữ liệu, phan tich du lieu, thống kê, thong ke, dữ liệu, du lieu, csv, biểu đồ, bieu do, xu hướng, xu huong, tương quan, tuong quan, outlier, data analysis]
tools: [file.read, shell.exec, calculator]
---

# Data Analysis Skill

Con số không phải câu trả lời — Ý NGHĨA của nó mới là.

## 1. Hiểu dữ liệu trước

Trả lời trước khi phân tích: nguồn ở đâu · mỗi cột nghĩa là gì (hỏi cho rõ, không
đoán) · khoảng thời gian nào, snapshot hay time series · vấn đề đã biết (thiếu
dữ liệu, outlier, sai số đo) · **đang trả lời câu hỏi nào**. Phân tích không có
câu hỏi thì chỉ là làm toán.

Đọc bằng `file.read` với file nhỏ; `shell.exec` với công cụ phù hợp (jq cho JSON,
awk cho CSV, script Python/Go cho tập lớn).

## 2. Làm sạch và kiểm tra

- Thiếu bao nhiêu giá trị, thiếu có theo quy luật nào không?
- Kiểu dữ liệu: số bị lưu thành chuỗi? ngày bị lưu thành timestamp?
- Outlier: giá trị bất khả thi về mặt vật lý hay cực trị về mặt thống kê?
- Giá trị có nằm trong biên mong đợi?
- Có dòng trùng?

Báo vấn đề chất lượng dữ liệu cho user **trước khi** phân tích, kèm lựa chọn:
"15% số đo nhiệt độ ngày thứ Ba bị thiếu — loại ngày đó hay nội suy?"

## 3. Khám phá

**Thống kê mô tả** cho mỗi biến số: count · mean và median (lệch nhau nhiều nghĩa
là phân phối lệch) · std dev · min/max · Q1/Q3 · IQR (thước đo độ phân tán ít bị
outlier ảnh hưởng).

**Phân phối:** chuẩn, lệch, hai đỉnh, hay đều? Có cụm hoặc khoảng trống bất
thường? Nếu user không xem được biểu đồ thì mô tả hình dạng bằng lời.

**Tương quan:** tính tương quan từng cặp biến số, nêu cặp mạnh (|r| > 0,7). **Luôn
nói rõ tương quan không phải nhân quả.**

**Chuỗi thời gian:** xu hướng chung (tăng/giảm/ổn định/theo chu kỳ) · tính mùa
(ngày/tuần/tháng) · điểm đổi hành vi ("hiệu năng tụt hẳn sau bản cập nhật ngày
15/3") · dùng rolling average để bớt nhiễu.

## 4. Tìm pattern

Cụm/phân khúc tự nhiên · điểm bất thường (nêu kèm mức lệch: "cao hơn bình thường
3 độ lệch chuẩn") · quan hệ phi tuyến, ngưỡng, tác động tương tác · phễu/chuỗi
bước: người dùng rơi ở bước nào.

## 5. Kiểm định giả thuyết

Nêu H0 và H1 → chọn test phù hợp (t-test so sánh trung bình, chi-squared cho biến
phân loại, test tương quan) → đặt mức ý nghĩa (thường α = 0,05) → tính test
statistic và p-value → diễn giải bằng lời thường, kèm điều kiện.

## 6. Biến phân tích thành insight

**Dở:** "mean 42,3, std dev 5,7."
**Tốt:** "thiết kế mới cho 42,3 kN — cải thiện 23% so với bản cũ, nhưng phương sai
lớn hơn (bản tệ nhất vẫn hơn bản tốt nhất cũ 8%)."

Mỗi insight trình bày 4 phần: **Finding** (dữ liệu cho thấy gì) · **Context** (vì
sao nó quan trọng với mục tiêu) · **Action** (nên làm gì) · **Confidence**
(cao/trung bình/thấp, kèm lý do).

## Chọn biểu đồ

| Loại dữ liệu | Biểu đồ | Vì sao |
|---|---|---|
| Phân phối | histogram, density | thấy hình dạng, độ lệch, số đỉnh |
| So sánh nhóm | bar, box plot | đặt cạnh nhau |
| Theo thời gian | line | thấy quỹ đạo |
| Quan hệ | scatter | thấy tương quan |
| Thành phần | stacked bar, treemap (tránh pie) | phần trong tổng thể |
| Xếp hạng | bar ngang đã sort | dễ đọc |
| Địa lý | map, heatmap | pattern không gian |

Vẽ thật qua `shell.exec` khi được (matplotlib/seaborn, gonum/plot).

## Lệnh nhanh

```bash
awk '{s+=$1; q+=$1*$1; n++} END {print "mean:", s/n, "sd:", sqrt(q/n-(s/n)^2)}' data.csv
jq '[.[].value] | add/length' data.json
```

Phân tích nặng thì viết script Python/Go rồi chạy.

## Anti-pattern

- **Phân tích không có câu hỏi** — hỏi "chúng ta đang muốn biết điều gì?" trước.
- **Chọn lọc kết quả** — báo TẤT CẢ phát hiện, không chỉ cái ủng hộ kết luận mong muốn.
- **Nói quá độ chắc chắn** — n = 12 thì kết quả là gợi ý, không phải kết luận.
- **P-hacking** — thử 20 giả thuyết rồi báo cái p < 0,05; phải hiệu chỉnh cho
  nhiều phép so sánh.
- **Bỏ qua chất lượng dữ liệu** — rác vào thì rác ra.
- **Phức tạp để cho oai** — một phép trung bình trả lời được câu hỏi thắng một mô
  hình tinh vi không trả lời được.
