import { I18nextProvider, useTranslation } from "react-i18next";
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";
import i18n from "@/i18n";
import { ApiError } from "./http";
import { translateApiError } from "./errors";

const Probe: React.FC<{
  err: unknown;
  fallback: string;
  values?: Record<string, unknown>;
}> = ({ err, fallback, values }) => {
  const { t } = useTranslation();
  return <span>{translateApiError(err, t, fallback, values)}</span>;
};

const renderProbe = (
  err: unknown,
  fallback: string,
  values?: Record<string, unknown>,
) =>
  render(
    <I18nextProvider i18n={i18n}>
      <Probe err={err} fallback={fallback} values={values} />
    </I18nextProvider>,
  );

describe("translateApiError", () => {
  beforeEach(async () => {
    await i18n.changeLanguage("vi");
  });

  it("translates a known error code according to the current locale", async () => {
    const vi_ = renderProbe(
      new ApiError(401, "raw backend message", "UNAUTHORIZED"),
      "fallback",
    );
    expect(
      await screen.findByText("Email hoặc mật khẩu không đúng."),
    ).toBeInTheDocument();
    vi_.unmount();

    await i18n.changeLanguage("en");
    renderProbe(
      new ApiError(401, "raw backend message", "UNAUTHORIZED"),
      "fallback",
    );
    expect(
      await screen.findByText("Incorrect email or password."),
    ).toBeInTheDocument();
  });

  it("falls back to the provided string when the code is unknown", () => {
    renderProbe(
      new ApiError(400, "raw", "SOME_UNMAPPED_CODE"),
      "Trang chủ fallback",
    );
    expect(screen.getByText("Trang chủ fallback")).toBeInTheDocument();
  });

  it("falls back to the provided string for a non-ApiError", () => {
    renderProbe(new Error("boom"), "generic fallback");
    expect(screen.getByText("generic fallback")).toBeInTheDocument();
  });

  it("never surfaces the raw backend message", () => {
    renderProbe(
      new ApiError(500, "raw backend message", "INTERNAL"),
      "fallback",
    );
    expect(screen.queryByText("raw backend message")).not.toBeInTheDocument();
  });

  it("supports interpolation values together with the error code", async () => {
    renderProbe(new ApiError(429, "raw", "RATE_LIMITED"), "fallback", {
      seconds: 42,
    });
    expect(
      await screen.findByText(
        "Quá nhiều yêu cầu. Vui lòng thử lại sau 42 giây.",
      ),
    ).toBeInTheDocument();
  });
});
