import { render, screen, fireEvent } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { beforeEach, describe, expect, it, vi, type Mock } from "vitest";
import i18n from "@/i18n";
import { MessageBubble } from "./MessageBubble";

vi.mock("@/stores/user.store", () => ({
  useUserStore: vi.fn(),
}));

import { useUserStore } from "@/stores/user.store";

const mockUseUserStore = useUserStore as unknown as Mock;

describe("MessageBubble", () => {
  beforeEach(async () => {
    mockUseUserStore.mockReturnValue(null);
    await i18n.changeLanguage("vi");
  });

  it("shows the Vietnamese retry label for a user message", () => {
    render(
      <I18nextProvider i18n={i18n}>
        <MessageBubble
          message={{ role: "user", content: "Xin chào" }}
          onRetryUser={() => {}}
        />
      </I18nextProvider>,
    );
    expect(
      screen.getByRole("button", { name: "Thử lại tin nhắn này" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Thử lại")).toBeInTheDocument();
  });

  it("shows the English retry label for a user message when locale is en", async () => {
    await i18n.changeLanguage("en");
    render(
      <I18nextProvider i18n={i18n}>
        <MessageBubble
          message={{ role: "user", content: "Hi there" }}
          onRetryUser={() => {}}
        />
      </I18nextProvider>,
    );
    expect(
      screen.getByRole("button", { name: "Retry this message" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Retry")).toBeInTheDocument();
  });

  it("shows Vietnamese assistant action bar, banners and usage footer", () => {
    render(
      <I18nextProvider i18n={i18n}>
        <MessageBubble
          message={{ role: "assistant", content: "Đây là câu trả lời" }}
          hasError
          truncated
          usage={{ inputTokens: 120, outputTokens: 340 }}
          onRegenerate={() => {}}
          onContinue={() => {}}
        />
      </I18nextProvider>,
    );

    // Error banner
    expect(
      screen.getByText("Phản hồi gặp sự cố hoặc quá trình xử lý bị gián đoạn."),
    ).toBeInTheDocument();
    expect(screen.getByText("Thử lại ngay")).toBeInTheDocument();

    // Truncated banner
    expect(
      screen.getByText("Câu trả lời bị cắt do chạm giới hạn độ dài tối đa."),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", {
        name: "Yêu cầu agent viết tiếp câu trả lời",
      }),
    ).toBeInTheDocument();
    expect(screen.getByText("Tiếp tục")).toBeInTheDocument();

    // Action bar
    expect(
      screen.getByRole("button", { name: "Sao chép nội dung" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Sao chép")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Đọc câu trả lời" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Đọc tiếng")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Tạo lại câu trả lời" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Tạo lại")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Hài lòng" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Chưa hài lòng" }),
    ).toBeInTheDocument();

    // Usage footer
    expect(screen.getByText("Input: 120 tokens")).toBeInTheDocument();
    expect(screen.getByText("Output: 340 tokens")).toBeInTheDocument();
    expect(screen.getByText("Tổng: 460 tokens")).toBeInTheDocument();
  });

  it("shows English assistant action bar, banners and usage footer when locale is en", async () => {
    await i18n.changeLanguage("en");
    render(
      <I18nextProvider i18n={i18n}>
        <MessageBubble
          message={{ role: "assistant", content: "Here is the answer" }}
          hasError
          truncated
          usage={{ inputTokens: 120, outputTokens: 340 }}
          onRegenerate={() => {}}
          onContinue={() => {}}
        />
      </I18nextProvider>,
    );

    expect(
      screen.getByText(
        "The response ran into an issue or processing was interrupted.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("Retry now")).toBeInTheDocument();

    expect(
      screen.getByText(
        "The response was cut off due to the maximum length limit.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", {
        name: "Ask the agent to continue the response",
      }),
    ).toBeInTheDocument();
    expect(screen.getByText("Continue")).toBeInTheDocument();

    expect(
      screen.getByRole("button", { name: "Copy content" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Copy")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Read the response aloud" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Read aloud")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Regenerate the response" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Regenerate")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Satisfied" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Not satisfied" }),
    ).toBeInTheDocument();

    expect(screen.getByText("Input: 120 tokens")).toBeInTheDocument();
    expect(screen.getByText("Output: 340 tokens")).toBeInTheDocument();
    expect(screen.getByText("Total: 460 tokens")).toBeInTheDocument();
  });

  it("shows the configured agent avatar image for assistant messages", () => {
    mockUseUserStore.mockReturnValue("https://cdn.example.com/jarvis.png");
    render(
      <I18nextProvider i18n={i18n}>
        <MessageBubble message={{ role: "assistant", content: "Xin chào" }} />
      </I18nextProvider>,
    );

    const img = screen.getByAltText("J.A.R.V.I.S.") as HTMLImageElement;
    expect(img).toBeInTheDocument();
    expect(img.src).toBe("https://cdn.example.com/jarvis.png");
  });

  it("falls back to the default bot icon when the agent avatar image fails to load", () => {
    mockUseUserStore.mockReturnValue("https://cdn.example.com/broken.png");
    render(
      <I18nextProvider i18n={i18n}>
        <MessageBubble message={{ role: "assistant", content: "Xin chào" }} />
      </I18nextProvider>,
    );

    const img = screen.getByAltText("J.A.R.V.I.S.");
    fireEvent.error(img);

    expect(screen.queryByAltText("J.A.R.V.I.S.")).not.toBeInTheDocument();
  });

  it("falls back to the default bot icon when agent_avatar_url is not configured", () => {
    mockUseUserStore.mockReturnValue(null);
    render(
      <I18nextProvider i18n={i18n}>
        <MessageBubble message={{ role: "assistant", content: "Xin chào" }} />
      </I18nextProvider>,
    );

    expect(screen.queryByAltText("J.A.R.V.I.S.")).not.toBeInTheDocument();
  });
});
