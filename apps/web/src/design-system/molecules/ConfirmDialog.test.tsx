import { render, screen } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { beforeEach, describe, expect, it, vi } from "vitest";
import i18n from "@/i18n";
import { ConfirmDialog } from "./ConfirmDialog";

const renderDialog = (
  props: Partial<React.ComponentProps<typeof ConfirmDialog>> = {},
) =>
  render(
    <I18nextProvider i18n={i18n}>
      <ConfirmDialog
        open
        title="Xóa mục này?"
        onConfirm={vi.fn()}
        onCancel={vi.fn()}
        {...props}
      />
    </I18nextProvider>,
  );

describe("ConfirmDialog", () => {
  beforeEach(async () => {
    await i18n.changeLanguage("vi");
  });

  it("falls back to translated Vietnamese confirm/cancel labels when none are provided", () => {
    renderDialog();
    expect(screen.getByRole("button", { name: "Hủy" })).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Xác nhận" }),
    ).toBeInTheDocument();
  });

  it("falls back to translated English confirm/cancel labels when none are provided", async () => {
    await i18n.changeLanguage("en");
    renderDialog();
    expect(screen.getByRole("button", { name: "Cancel" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Confirm" })).toBeInTheDocument();
  });

  it("still honors explicit confirmLabel/cancelLabel overrides", () => {
    renderDialog({ confirmLabel: "Xóa vĩnh viễn", cancelLabel: "Thôi" });
    expect(screen.getByRole("button", { name: "Thôi" })).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Xóa vĩnh viễn" }),
    ).toBeInTheDocument();
  });
});
