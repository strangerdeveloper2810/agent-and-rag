import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi, type Mock } from "vitest";
import { createTestQueryClient, withQueryClient } from "@/test/query";
import { useSession } from "./useSession";

vi.mock("@/lib/http", () => ({
  default: { get: vi.fn() },
}));

import api from "@/lib/http";

const mockGet = (api as unknown as { get: Mock }).get;

/** Component tối giản chỉ để quan sát kết quả của useSession(). */
const Probe = ({ label }: { label: string }) => {
  const { user, isPending } = useSession();
  if (isPending) return <div>{label}:pending</div>;
  return (
    <div>
      {label}:{user ? user.email : "anonymous"}
    </div>
  );
};

describe("useSession", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("nhiều component cùng mount chỉ tạo MỘT request /api/auth/me", async () => {
    mockGet.mockResolvedValue({ user: { id: "u1", email: "a@b.c" } });

    // Đây chính là lỗi cũ: main.tsx gọi init() ở module scope + mỗi guard gọi
    // init() trong useEffect → reload trang là 3 lần /api/auth/me. Zustand
    // không dedupe được vì nó không biết request nào đang bay.
    render(
      withQueryClient(
        <>
          <Probe label="guard" />
          <Probe label="sidebar" />
          <Probe label="modal" />
        </>,
        createTestQueryClient(),
      ),
    );

    expect(await screen.findByText("guard:a@b.c")).toBeInTheDocument();
    expect(screen.getByText("sidebar:a@b.c")).toBeInTheDocument();
    expect(screen.getByText("modal:a@b.c")).toBeInTheDocument();
    expect(mockGet).toHaveBeenCalledTimes(1);
    expect(mockGet).toHaveBeenCalledWith("/api/auth/me");
  });

  it("chưa đăng nhập (401) là kết quả hợp lệ, không phải lỗi query", async () => {
    mockGet.mockRejectedValue({ status: 401, message: "Unauthorized" });

    render(withQueryClient(<Probe label="guard" />, createTestQueryClient()));

    // Guard chỉ cần biết "đã kiểm tra xong, không có user" để redirect —
    // nếu để query vào trạng thái error thì phải phân biệt thêm lỗi mạng.
    expect(await screen.findByText("guard:anonymous")).toBeInTheDocument();
    expect(mockGet).toHaveBeenCalledTimes(1);
  });

  it("isPending = true ở lần render đầu (guard hiện spinner, không đá về /login)", () => {
    mockGet.mockReturnValue(new Promise(() => {}));

    render(withQueryClient(<Probe label="guard" />, createTestQueryClient()));

    expect(screen.getByText("guard:pending")).toBeInTheDocument();
  });
});
