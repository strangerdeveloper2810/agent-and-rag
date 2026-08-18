import { act, render, screen } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { beforeEach, describe, expect, it } from "vitest";
import i18n from "@/i18n";
import { ToastProvider, useToast } from "./Toast";

const Trigger: React.FC = () => {
  const toast = useToast();
  return <button onClick={() => toast.info("hello")}>fire</button>;
};

const renderToast = () =>
  render(
    <I18nextProvider i18n={i18n}>
      <ToastProvider>
        <Trigger />
      </ToastProvider>
    </I18nextProvider>,
  );

describe("Toast", () => {
  beforeEach(async () => {
    await i18n.changeLanguage("vi");
  });

  it("uses the Vietnamese dismiss label", () => {
    renderToast();
    act(() => {
      screen.getByText("fire").click();
    });
    expect(
      screen.getByRole("button", { name: "Đóng thông báo" }),
    ).toBeInTheDocument();
  });

  it("uses the English dismiss label", async () => {
    await i18n.changeLanguage("en");
    renderToast();
    act(() => {
      screen.getByText("fire").click();
    });
    expect(screen.getByRole("button", { name: "Dismiss" })).toBeInTheDocument();
  });
});
