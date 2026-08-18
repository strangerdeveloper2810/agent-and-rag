import { fireEvent, render, screen } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { beforeEach, describe, expect, it } from "vitest";
import i18n from "@/i18n";
import { ToastProvider } from "@/design-system/molecules/Toast";
import { Composer } from "./Composer";

const renderComposer = (disabled: boolean) =>
  render(
    <I18nextProvider i18n={i18n}>
      <ToastProvider>
        <Composer
          value=""
          onChange={() => {}}
          onSend={() => {}}
          disabled={disabled}
          onStop={() => {}}
          attachments={[]}
          onAttachmentsChange={() => {}}
        />
      </ToastProvider>
    </I18nextProvider>,
  );

describe("Composer", () => {
  beforeEach(async () => {
    await i18n.changeLanguage("vi");
  });

  it("shows the Vietnamese placeholder, stop pill and drag overlay", () => {
    const { container } = renderComposer(true);

    expect(
      screen.getByPlaceholderText("Hỏi J.A.R.V.I.S... (gõ / để xem lệnh nhanh)"),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/J\.A\.R\.V\.I\.S\. đang suy luận \(0s\)\.\.\./),
    ).toBeInTheDocument();
    expect(screen.getByText("Dừng lại")).toBeInTheDocument();

    fireEvent.dragOver(container.firstElementChild as Element);
    expect(
      screen.getByText("Thả file vào đây để đính kèm"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Hỗ trợ Hình ảnh, PDF, Word, Excel, Markdown"),
    ).toBeInTheDocument();
  });

  it("shows the English placeholder, stop pill and drag overlay", async () => {
    await i18n.changeLanguage("en");
    const { container } = renderComposer(true);

    expect(
      screen.getByPlaceholderText(
        "Ask J.A.R.V.I.S... (type / to see quick commands)",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/J\.A\.R\.V\.I\.S\. is thinking \(0s\)\.\.\./),
    ).toBeInTheDocument();
    expect(screen.getByText("Stop")).toBeInTheDocument();

    fireEvent.dragOver(container.firstElementChild as Element);
    expect(screen.getByText("Drop files here to attach")).toBeInTheDocument();
    expect(
      screen.getByText("Supports Images, PDF, Word, Excel, Markdown"),
    ).toBeInTheDocument();
  });
});
