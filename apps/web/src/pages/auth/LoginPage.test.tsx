import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { I18nextProvider } from "react-i18next";
import { beforeEach, describe, expect, it, vi } from "vitest";
import i18n from "@/i18n";
import { ToastProvider } from "@/design-system/molecules/Toast";
import { useAuthStore } from "@/stores/auth.store";
import { LoginPage } from "./LoginPage";

vi.mock("@/stores/auth.store", () => ({
  useAuthStore: vi.fn(),
}));

const mockedUseAuthStore = vi.mocked(useAuthStore);

const renderPage = () =>
  render(
    <I18nextProvider i18n={i18n}>
      <MemoryRouter>
        <ToastProvider>
          <LoginPage />
        </ToastProvider>
      </MemoryRouter>
    </I18nextProvider>,
  );

describe("LoginPage", () => {
  const login = vi.fn();

  beforeEach(async () => {
    login.mockReset();
    mockedUseAuthStore.mockReturnValue({
      login,
      isLoading: false,
    } as unknown as ReturnType<typeof useAuthStore>);
    await i18n.changeLanguage("vi");
  });

  it("renders Vietnamese copy by default", () => {
    renderPage();
    expect(
      screen.getByRole("button", {
        name: "Đăng nhập nhanh với tài khoản Demo",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByPlaceholderText("name@company.com"),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: /Đăng ký ngay/ }),
    ).toBeInTheDocument();
  });

  it("renders English copy when locale is en", async () => {
    await i18n.changeLanguage("en");
    renderPage();
    expect(
      screen.getByRole("button", { name: "Quick sign-in with Demo account" }),
    ).toBeInTheDocument();
    expect(
      screen.getByPlaceholderText("name@company.com"),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: /Sign up now/ }),
    ).toBeInTheDocument();
  });

  it("shows the translated fallback toast when demo login fails (en)", async () => {
    login.mockRejectedValue(undefined);
    await i18n.changeLanguage("en");
    renderPage();

    await userEvent.click(
      screen.getByRole("button", { name: "Quick sign-in with Demo account" }),
    );

    expect(
      await screen.findByText(
        "Couldn't sign in automatically with the demo account. Please try entering your credentials manually.",
      ),
    ).toBeInTheDocument();
  });

  it("shows the translated fallback toast on generic login failure (vi)", async () => {
    login.mockRejectedValue(undefined);
    renderPage();

    await userEvent.type(
      screen.getByLabelText("Địa chỉ Email"),
      "user@example.com",
    );
    await userEvent.type(screen.getByLabelText("Mật khẩu"), "password123");
    await userEvent.click(
      screen.getByRole("button", { name: "Đăng nhập ngay" }),
    );

    expect(
      await screen.findByText(
        "Đăng nhập thất bại. Vui lòng kiểm tra lại email/mật khẩu.",
      ),
    ).toBeInTheDocument();
  });
});
