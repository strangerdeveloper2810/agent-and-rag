import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { I18nextProvider } from "react-i18next";
import { beforeEach, describe, expect, it } from "vitest";
import i18n from "@/i18n";
import type { ToolCallState } from "@/modules/chat/chat.api";
import { ToolCallGroup } from "./ToolCallCard";

const tools: ToolCallState[] = [
  { name: "rag.search", status: "done", result: "3 kết quả" },
];

const renderGroup = () =>
  render(
    <I18nextProvider i18n={i18n}>
      <ToolCallGroup tools={tools} />
    </I18nextProvider>,
  );

describe("ToolCallGroup", () => {
  beforeEach(async () => {
    await i18n.changeLanguage("vi");
  });

  it("shows Vietnamese header + tool meta when expanded", async () => {
    renderGroup();
    expect(
      screen.getByText("Đã hoàn thành 1 bước thực thi"),
    ).toBeInTheDocument();

    await userEvent.click(
      screen.getByRole("button", { name: "Danh sách tool call" }),
    );

    expect(screen.getByText("Tra cứu tài liệu RAG")).toBeInTheDocument();
    expect(
      screen.getByText("Tìm thông tin trong cơ sở tri thức local"),
    ).toBeInTheDocument();
  });

  it("shows English header + tool meta when locale is en", async () => {
    await i18n.changeLanguage("en");
    renderGroup();
    expect(
      screen.getByText("Completed 1 execution steps"),
    ).toBeInTheDocument();

    await userEvent.click(
      screen.getByRole("button", { name: "Tool call list" }),
    );

    expect(screen.getByText("Search RAG documents")).toBeInTheDocument();
    expect(
      screen.getByText("Find information in the local knowledge base"),
    ).toBeInTheDocument();
  });
});
