import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import type { QueryClient } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import i18n from "@/i18n";
import { queryKeys } from "@/lib/query-keys";
import { createTestQueryClient, withQueryClient } from "@/test/query";
import { EmptyState } from "./EmptyState";

let queryClient: QueryClient;

const renderEmptyState = () =>
  render(
    withQueryClient(
      <I18nextProvider i18n={i18n}>
        <EmptyState onPick={() => {}} />
      </I18nextProvider>,
      queryClient,
    ),
  );

/**
 * Giả lập GET /api/suggestions (một lượt gọi LLM ở agent-go, qua BFF).
 *
 * Stub trùm lên global fetch nên spy nhận CẢ /api/user/settings (component còn
 * đọc avatar agent) — vì vậy phải đếm riêng theo URL, xem suggestionCalls().
 */
const mockSuggestionsFetch = (
  response: {
    ok: boolean;
    suggestions?: { text: string; category?: string }[];
  } = { ok: true },
) => {
  const spy = vi.fn((url: string) =>
    Promise.resolve({
      ok: String(url).includes("/suggestions") ? response.ok : false,
      json: () => Promise.resolve({ suggestions: response.suggestions }),
    } as Response),
  );
  vi.stubGlobal("fetch", spy);
  return spy;
};

/** Số lần thực sự gọi tới /suggestions (bỏ qua các request khác). */
const suggestionCalls = (spy: ReturnType<typeof mockSuggestionsFetch>) =>
  spy.mock.calls.filter(([url]) => String(url).includes("/suggestions")).length;

describe("EmptyState", () => {
  beforeEach(async () => {
    queryClient = createTestQueryClient();
    await i18n.changeLanguage("vi");
    mockSuggestionsFetch({ ok: false });
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("shows Vietnamese header, subtitle and category tabs", async () => {
    renderEmptyState();

    expect(
      screen.getByText("Tôi có thể giúp gì cho bạn hôm nay?"),
    ).toBeInTheDocument();
    expect(screen.getByText("Ý tưởng & Kế hoạch")).toBeInTheDocument();
    expect(
      screen.getByTitle("Đổi bộ gợi ý ngẫu nhiên mới"),
    ).toBeInTheDocument();

    expect(
      await screen.findByText("Gợi ý thông minh từ AI Agent"),
    ).toBeInTheDocument();
  });

  it("shows English header, subtitle and category tabs", async () => {
    await i18n.changeLanguage("en");
    renderEmptyState();

    expect(screen.getByText("How can I help you today?")).toBeInTheDocument();
    expect(screen.getByText("Ideas & Planning")).toBeInTheDocument();
    expect(screen.getByTitle("Refresh random suggestions")).toBeInTheDocument();

    expect(
      await screen.findByText("Smart Suggestions from AI Agent"),
    ).toBeInTheDocument();
  });

  // /suggestions là một lượt gọi LLM nên phải cache: mount lại component KHÔNG
  // được gọi lại. Đây chính là khoản tốn kém mà bản Zustand cũ không chặn được.
  describe("gợi ý từ agent", () => {
    it("hiện gợi ý agent trả về", async () => {
      mockSuggestionsFetch({
        ok: true,
        suggestions: [{ text: "Tóm tắt tài liệu này" }],
      });

      renderEmptyState();

      expect(
        await screen.findByText("Tóm tắt tài liệu này"),
      ).toBeInTheDocument();
    });

    it("chỉ gọi /suggestions một lần dù component mount lại", async () => {
      const spy = mockSuggestionsFetch({
        ok: true,
        suggestions: [{ text: "Gợi ý đã cache" }],
      });

      const first = renderEmptyState();
      await screen.findByText("Gợi ý đã cache");
      expect(suggestionCalls(spy)).toBe(1);

      first.unmount();
      renderEmptyState();
      await screen.findByText("Gợi ý đã cache");

      // Vẫn 1 — lần mount thứ hai đọc từ cache theo queryKey ["suggestions"].
      expect(suggestionCalls(spy)).toBe(1);
    });

    // Trước fix: mục "Gợi ý thông minh từ AI Agent" lặp lại y hệt dù đổi tab
    // (không phụ thuộc activeTab). Giờ agent-go trả 1 lô kèm category, FE lọc
    // theo tab đang chọn ở client — KHÔNG gọi lại LLM mỗi lần đổi tab.
    it("đổi tab thì lọc gợi ý agent theo category, không gọi lại LLM", async () => {
      const spy = mockSuggestionsFetch({
        ok: true,
        suggestions: [
          { text: "Gợi ý lập trình", category: "dev" },
          { text: "Gợi ý tra cứu", category: "search" },
        ],
      });

      renderEmptyState();
      // Tab mặc định "Ý tưởng & Kế hoạch" (creative) không khớp category nào
      // trong lô trả về → hiện đủ (base), không để trống cả mục.
      await screen.findByText("Gợi ý lập trình");
      expect(screen.getByText("Gợi ý tra cứu")).toBeInTheDocument();

      fireEvent.click(screen.getByText("Lập trình & Clean Code"));

      await waitFor(() => {
        expect(screen.getByText("Gợi ý lập trình")).toBeInTheDocument();
        expect(screen.queryByText("Gợi ý tra cứu")).not.toBeInTheDocument();
      });

      // Vẫn 1 lần gọi — đổi tab chỉ lọc lại dữ liệu đã cache, không tốn quota LLM.
      expect(suggestionCalls(spy)).toBe(1);
    });

    it("agent lỗi thì vẫn hiện gợi ý dự phòng từ file i18n", async () => {
      mockSuggestionsFetch({ ok: false });

      renderEmptyState();

      expect(
        await screen.findByText("Gợi ý thông minh từ AI Agent"),
      ).toBeInTheDocument();
    });

    it("bấm nút đổi gợi ý thì gọi lại agent", async () => {
      const spy = mockSuggestionsFetch({
        ok: true,
        suggestions: [{ text: "Gợi ý ban đầu" }],
      });

      renderEmptyState();
      await screen.findByText("Gợi ý ban đầu");
      expect(suggestionCalls(spy)).toBe(1);

      fireEvent.click(screen.getByTitle("Đổi bộ gợi ý ngẫu nhiên mới"));

      await waitFor(() => expect(suggestionCalls(spy)).toBe(2));
    });
  });

  // User upload avatar cho agent nhưng khung chat không đổi gì — vì lúc chưa có
  // tin nhắn nào thì MessageBubble chưa render, mà hero của EmptyState lại
  // hardcode icon. Nhóm test này khoá lại hành vi đó.
  describe("avatar agent ở hero", () => {
    const seedAgentAvatar = (url: string) =>
      queryClient.setQueryData(queryKeys.settings(), {
        agent_avatar_url: url,
      } as never);

    it("hiện icon mặc định khi user chưa cấu hình avatar agent", () => {
      renderEmptyState();
      expect(screen.queryByRole("img")).not.toBeInTheDocument();
    });

    it("hiện ảnh thật khi settings có agent_avatar_url", () => {
      seedAgentAvatar("https://cdn.example.com/agent.png");

      renderEmptyState();

      const img = screen.getByRole("img");
      expect(img).toHaveAttribute("src", "https://cdn.example.com/agent.png");
    });

    it("ảnh lỗi 404 thì fallback về icon, không để khung trống", () => {
      seedAgentAvatar("https://cdn.example.com/hong.png");

      renderEmptyState();
      fireEvent.error(screen.getByRole("img"));

      expect(screen.queryByRole("img")).not.toBeInTheDocument();
    });
  });
});
