import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { I18nextProvider } from "react-i18next";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import i18n from "@/i18n";
import { ToastProvider } from "@/design-system/molecules/Toast";
import type { ChatEvent } from "@/modules/chat/chat.api";
import { ChatPage } from "./ChatPage";

// jsdom doesn't implement scrollIntoView; ChatPage calls it on every message update.
Element.prototype.scrollIntoView = vi.fn();

const chatApi = vi.hoisted(() => ({
  createConversation: vi.fn(),
  getMessages: vi.fn(),
  streamChat: vi.fn(),
  streamContinue: vi.fn(),
}));

vi.mock("@/modules/chat/chat.api", async (importOriginal) => {
  const actual =
    await importOriginal<typeof import("@/modules/chat/chat.api")>();
  return {
    ...actual,
    createConversation: chatApi.createConversation,
    getMessages: chatApi.getMessages,
    streamChat: chatApi.streamChat,
    streamContinue: chatApi.streamContinue,
  };
});

vi.mock("@/context/ConversationContext", () => ({
  useConversation: () => ({ reloadConversations: vi.fn() }),
}));

const renderChatPage = () =>
  render(
    <I18nextProvider i18n={i18n}>
      <ToastProvider>
        <MemoryRouter initialEntries={["/messages/conv-1"]}>
          <Routes>
            <Route path="/messages/:id" element={<ChatPage />} />
          </Routes>
        </MemoryRouter>
      </ToastProvider>
    </I18nextProvider>,
  );

describe("ChatPage", () => {
  beforeEach(async () => {
    document.head.insertAdjacentHTML(
      "beforeend",
      '<meta name="description" content="placeholder">',
    );
    chatApi.getMessages.mockResolvedValue([]);
    chatApi.createConversation.mockReset();
    await i18n.changeLanguage("vi");
  });

  afterEach(() => {
    document.head
      .querySelectorAll('meta[name="description"]')
      .forEach((el) => el.remove());
    vi.clearAllMocks();
  });

  it("sets the Vietnamese document title/description and shows the interrupt toast", async () => {
    let capturedOnEvent: ((e: ChatEvent) => void) | undefined;
    chatApi.streamChat.mockImplementation(
      (_id: string, _content: string, onEvent: (e: ChatEvent) => void) => {
        capturedOnEvent = onEvent;
        return Promise.resolve();
      },
    );

    renderChatPage();

    await waitFor(() => expect(document.title).toContain("Trò chuyện AI"));
    expect(
      document
        .querySelector('meta[name="description"]')
        ?.getAttribute("content"),
    ).toBe(
      "Trò chuyện và giao việc trực tiếp cho trợ lý AI thông minh J.A.R.V.I.S.",
    );

    const textbox = await screen.findByPlaceholderText(
      "Hỏi J.A.R.V.I.S... (gõ / để xem lệnh nhanh)",
    );
    await userEvent.type(textbox, "Xin chào");
    await userEvent.click(screen.getByRole("button", { name: "Gửi tin nhắn" }));

    await waitFor(() => expect(capturedOnEvent).toBeDefined());
    act(() => {
      capturedOnEvent!({ type: "interrupt", name: "shell.exec" });
    });

    expect(
      await screen.findByText(
        'Đã dừng: công cụ "shell.exec" cần được xác nhận trước khi chạy.',
      ),
    ).toBeInTheDocument();
  });

  it("shows the English service-error toast and context warning banner", async () => {
    await i18n.changeLanguage("en");

    let capturedOnEvent: ((e: ChatEvent) => void) | undefined;
    chatApi.streamChat.mockImplementation(
      (_id: string, _content: string, onEvent: (e: ChatEvent) => void) => {
        capturedOnEvent = onEvent;
        return Promise.resolve();
      },
    );

    renderChatPage();

    await waitFor(() => expect(document.title).toContain("AI Chat"));

    const textbox = await screen.findByPlaceholderText(
      "Ask J.A.R.V.I.S... (type / to see quick commands)",
    );
    await userEvent.type(textbox, "Hello");
    await userEvent.click(screen.getByRole("button", { name: "Send message" }));

    await waitFor(() => expect(capturedOnEvent).toBeDefined());
    act(() => {
      capturedOnEvent!({ type: "error", message: "boom" });
    });

    expect(
      await screen.findByText(
        "We're having some trouble with the AI service right now. We'll get it fixed shortly — please try again.",
      ),
    ).toBeInTheDocument();

    act(() => {
      capturedOnEvent!({
        type: "done",
        contextTokens: 90000,
        contextBudget: 100000,
      });
    });

    expect(
      await screen.findByText(
        "This conversation is getting long (90K tokens) — start a new chat for faster, more accurate responses. Key information already learned will still be kept.",
      ),
    ).toBeInTheDocument();
    expect(
      await screen.findByRole("button", { name: "Start a new conversation" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Start new chat")).toBeInTheDocument();
  });

  it("forwards the current UI language to streamChat", async () => {
    await i18n.changeLanguage("en");
    chatApi.streamChat.mockResolvedValue(undefined);

    renderChatPage();

    const textbox = await screen.findByPlaceholderText(
      "Ask J.A.R.V.I.S... (type / to see quick commands)",
    );
    await userEvent.type(textbox, "Hello");
    await userEvent.click(screen.getByRole("button", { name: "Send message" }));

    await waitFor(() => expect(chatApi.streamChat).toHaveBeenCalled());
    expect(chatApi.streamChat).toHaveBeenCalledWith(
      expect.any(String),
      "Hello",
      expect.any(Function),
      expect.anything(),
      undefined,
      "en",
    );
  });
});
