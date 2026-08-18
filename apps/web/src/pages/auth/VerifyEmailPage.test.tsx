import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { I18nextProvider } from "react-i18next";
import { beforeEach, describe, expect, it, vi } from "vitest";
import i18n from "@/i18n";
import { ToastProvider } from "@/design-system/molecules/Toast";
import { useAuthStore } from "@/stores/auth.store";
import { VerifyEmailPage } from "./VerifyEmailPage";

vi.mock("@/stores/auth.store", () => ({
  useAuthStore: vi.fn(),
}));

const mockedUseAuthStore = vi.mocked(useAuthStore);

const renderPage = (email = "user@example.com") =>
  render(
    <I18nextProvider i18n={i18n}>
      <MemoryRouter initialEntries={[`/verify-email?email=${email}`]}>
        <ToastProvider>
          <VerifyEmailPage />
        </ToastProvider>
      </MemoryRouter>
    </I18nextProvider>,
  );

describe("VerifyEmailPage", () => {
  const verifyEmail = vi.fn();
  const resendOtp = vi.fn();

  beforeEach(async () => {
    verifyEmail.mockReset();
    resendOtp.mockReset();
    mockedUseAuthStore.mockReturnValue({
      verifyEmail,
      resendOtp,
      isLoading: false,
    } as unknown as ReturnType<typeof useAuthStore>);
    await i18n.changeLanguage("vi");
  });

  it("renders Vietnamese copy by default", () => {
    renderPage();
    expect(
      screen.getByRole("heading", { name: "Xác minh email" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /Gửi lại mã sau \d+s/ }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: /Quay lại đăng nhập/ }),
    ).toBeInTheDocument();
  });

  it("renders English copy when locale is en", async () => {
    await i18n.changeLanguage("en");
    renderPage();
    expect(
      screen.getByRole("heading", { name: "Verify your email" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /Resend code in \d+s/ }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: /Back to sign in/ }),
    ).toBeInTheDocument();
  });

  it("shows the translated fallback toast on verification failure (en)", async () => {
    verifyEmail.mockRejectedValue(undefined);
    await i18n.changeLanguage("en");
    renderPage();

    await userEvent.type(screen.getByRole("textbox"), "123456");
    await userEvent.click(screen.getByRole("button", { name: "Verify" }));

    expect(
      await screen.findByText("Verification failed. Please try again."),
    ).toBeInTheDocument();
  });
});
