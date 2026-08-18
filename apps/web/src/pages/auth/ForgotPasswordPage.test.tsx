import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { I18nextProvider } from "react-i18next";
import { beforeEach, describe, expect, it, vi } from "vitest";
import i18n from "@/i18n";
import { ToastProvider } from "@/design-system/molecules/Toast";
import { useAuthStore } from "@/stores/auth.store";
import { ForgotPasswordPage } from "./ForgotPasswordPage";

vi.mock("@/stores/auth.store", () => ({
  useAuthStore: vi.fn(),
}));

const mockedUseAuthStore = vi.mocked(useAuthStore);

const renderPage = () =>
  render(
    <I18nextProvider i18n={i18n}>
      <MemoryRouter>
        <ToastProvider>
          <ForgotPasswordPage />
        </ToastProvider>
      </MemoryRouter>
    </I18nextProvider>,
  );

describe("ForgotPasswordPage", () => {
  const forgotPassword = vi.fn();
  const resetPassword = vi.fn();

  beforeEach(async () => {
    forgotPassword.mockReset();
    resetPassword.mockReset();
    mockedUseAuthStore.mockReturnValue({
      forgotPassword,
      resetPassword,
      isLoading: false,
    } as unknown as ReturnType<typeof useAuthStore>);
    await i18n.changeLanguage("vi");
  });

  it("renders Step 1 email input properly", () => {
    renderPage();
    expect(screen.getByPlaceholderText("name@company.com")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /Gửi mã xác thực OTP/i }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: /Quay lại đăng nhập/i }),
    ).toBeInTheDocument();
  });

  it("submits email and transitions to Step 2 OTP verification", async () => {
    const user = userEvent.setup();
    forgotPassword.mockResolvedValueOnce(undefined);

    renderPage();
    const emailInput = screen.getByPlaceholderText("name@company.com");
    await user.type(emailInput, "test@example.com");

    const submitBtn = screen.getByRole("button", {
      name: /Gửi mã xác thực OTP/i,
    });
    await user.click(submitBtn);

    await waitFor(() => {
      expect(forgotPassword).toHaveBeenCalledWith("test@example.com");
      expect(screen.getByPlaceholderText("000000")).toBeInTheDocument();
      expect(
        screen.getByPlaceholderText("Tối thiểu 8 ký tự"),
      ).toBeInTheDocument();
    });
  });
});
