import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { I18nextProvider } from "react-i18next";
import { beforeEach, describe, expect, it, vi } from "vitest";
import i18n from "@/i18n";
import { ToastProvider } from "@/design-system/molecules/Toast";
import { useAuthStore } from "@/stores/auth.store";
import { RegisterPage } from "./RegisterPage";

vi.mock("@/stores/auth.store", () => ({
  useAuthStore: vi.fn(),
}));

const mockedUseAuthStore = vi.mocked(useAuthStore);

const renderPage = () =>
  render(
    <I18nextProvider i18n={i18n}>
      <MemoryRouter>
        <ToastProvider>
          <RegisterPage />
        </ToastProvider>
      </MemoryRouter>
    </I18nextProvider>,
  );

describe("RegisterPage", () => {
  const registerUser = vi.fn();

  beforeEach(async () => {
    registerUser.mockReset();
    mockedUseAuthStore.mockReturnValue({
      register: registerUser,
      isLoading: false,
    } as unknown as ReturnType<typeof useAuthStore>);
    await i18n.changeLanguage("vi");
  });

  it("renders Vietnamese copy by default", () => {
    renderPage();
    expect(
      screen.getByRole("heading", { name: "Tạo tài khoản mới" }),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("Họ và tên")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Tạo tài khoản J.A.R.V.I.S." }),
    ).toBeInTheDocument();
  });

  it("renders English copy when locale is en", async () => {
    await i18n.changeLanguage("en");
    renderPage();
    expect(
      screen.getByRole("heading", { name: "Create a new account" }),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("Full name")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Create J.A.R.V.I.S. account" }),
    ).toBeInTheDocument();
  });

  it("shows the translated password strength label (en)", async () => {
    await i18n.changeLanguage("en");
    renderPage();

    await userEvent.type(screen.getByLabelText("Password"), "abc12345");

    expect(screen.getByText("Password strength:")).toBeInTheDocument();
  });

  it("shows the translated fallback toast on registration failure (en)", async () => {
    registerUser.mockRejectedValue(undefined);
    await i18n.changeLanguage("en");
    renderPage();

    await userEvent.type(screen.getByLabelText("Full name"), "John Doe");
    await userEvent.type(
      screen.getByLabelText("Email address"),
      "user@example.com",
    );
    await userEvent.type(screen.getByLabelText("Password"), "password123");
    await userEvent.click(
      screen.getByRole("button", { name: "Create J.A.R.V.I.S. account" }),
    );

    expect(
      await screen.findByText(
        "Registration failed. This email may already be in use.",
      ),
    ).toBeInTheDocument();
  });
});
