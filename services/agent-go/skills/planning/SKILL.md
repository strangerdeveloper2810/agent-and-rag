---
name: planning
description: Project planning — break down goals into tasks, estimate effort, identify dependencies, create timeline
when_to_use: When the user needs to plan a project, organize work, estimate timelines, or structure a complex goal into manageable pieces
triggers: [lập kế hoạch, lap ke hoach, kế hoạch, ke hoach, roadmap, chia task, phân task, phan task, lên plan, len plan]
tools: [file.read, file.write, web.search]
---

# Planning Skill

Biến mục tiêu mơ hồ thành kế hoạch chạy được: có phase, milestone, phụ thuộc, và
ước lượng công sức.

## 0. Discovery

1. **Định nghĩa "xong"** — cụ thể và đo được. "Làm app mới" là mơ hồ; "app cho
   phép đặt hàng, thanh toán qua VNPay, xong trước Q4" là mục tiêu.
2. **Ràng buộc** — thời gian, ngân sách, người, công nghệ, phụ thuộc nhóm khác.
3. **Tiêu chí thành công** — 2–3 kết quả đo được.
4. **Điều chưa biết** — liệt kê giả định cần kiểm chứng.
5. `web.search` tham khảo dự án tương tự, best practice, ước lượng thời gian.

## 1. Chia việc (WBS)

- Mỗi task làm xong trong **1–5 ngày**. Dài hơn thì chia nhỏ tiếp.
- Mỗi task có người phụ trách rõ ràng và định nghĩa "xong".
- Task phải cụ thể: "thiết kế schema đơn hàng", không phải "làm phần backend".

Cấu trúc: Epic → Milestone (theo tuần) → Task, mỗi task ghi kèm người phụ trách,
công sức, và phụ thuộc vào task nào.

## 2. Phụ thuộc

Liệt kê mọi phụ thuộc → xác định **critical path** (chuỗi phụ thuộc dài nhất, nó
quyết định thời gian tối thiểu của dự án) → tách riêng phụ thuộc **bên ngoài** (
không kiểm soát được: vendor, phê duyệt đối tác) → chỉ rõ việc nào chạy song song
được, việc nào buộc phải tuần tự.

## 3. Ước lượng

Cỡ áo trước: **S** 1–2 ngày · **M** 3–5 ngày · **L** 1–2 tuần · **XL** 3–4 tuần.

Với L và XL: chia nhỏ tiếp nếu được; nhân buffer 1,5× khi biết là phức tạp, 2×
khi độ bất định cao.

Mỗi task nghĩ 3 mốc: tốt nhất · thường gặp · xấu nhất. Lập kế hoạch theo mốc
"thường gặp", ghi mốc "xấu nhất" vào phần rủi ro.

## 4. Timeline

Bảng theo tuần: milestone · deliverable chính · rủi ro.

- Không task nào kéo quá 2 tuần mà không có deliverable trung gian.
- Mỗi milestone phải có thứ demo được.
- Chừa buffer tối thiểu **20%** tổng thời gian.
- Tuần cuối trước deadline **không bao giờ** dành cho tính năng mới — chỉ test,
  sửa, và hoàn thiện.

## 5. Sổ rủi ro

Mỗi rủi ro ghi: xác suất (thấp/vừa/cao) · tác động · **giảm thiểu** (đang làm gì
để nó ít xảy ra) · **phương án B** (nếu vẫn xảy ra thì làm gì).

## 6. Giao tiếp

Hằng ngày: check-in ngắn 5 phút · hằng tuần: tiến độ so với milestone, vướng gì,
điều chỉnh gì · mỗi milestone: demo hoặc review deliverable · khi rủi ro thành
hiện thực: báo ngay kèm các lựa chọn, đừng chờ tới buổi họp tuần.

## Định dạng bản kế hoạch

Goal (một câu, đo được) · tiêu chí thành công · ràng buộc · bảng milestone và
timeline · WBS · phụ thuộc (critical path + phụ thuộc ngoài) · sổ rủi ro · **next
steps** (việc làm hôm nay, việc làm tuần này).

## Anti-pattern

- **Waterfall mọi thứ**: việc 2 ngày thì bỏ kế hoạch hình thức, làm luôn.
- **Coi nhẹ phần chưa biết**: ước lượng giả định mọi thứ suôn sẻ là ước lượng sai.
- **Bỏ qua yếu tố con người**: người ta bị họp, bị việc khác chen vào, nghỉ phép.
- **Kế hoạch thành sản phẩm**: kế hoạch phục vụ dự án, không phải ngược lại — thực
  tế đổi thì cập nhật kế hoạch.
