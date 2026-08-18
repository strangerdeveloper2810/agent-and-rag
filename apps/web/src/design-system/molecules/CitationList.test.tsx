import { render, screen } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import i18n from "@/i18n";
import { CitationList } from "./CitationList";

describe("CitationList", () => {
  beforeEach(async () => {
    await i18n.changeLanguage("vi");
  });

  it("shows the singular Source label", () => {
    render(
      <I18nextProvider i18n={i18n}>
        <CitationList citations={[{ title: "Doc A" }]} />
      </I18nextProvider>,
    );
    expect(screen.getByText("Source")).toBeInTheDocument();
  });

  it("shows the plural Sources label for multiple citations", () => {
    render(
      <I18nextProvider i18n={i18n}>
        <CitationList citations={[{ title: "Doc A" }, { title: "Doc B" }]} />
      </I18nextProvider>,
    );
    expect(screen.getByText("Sources")).toBeInTheDocument();
  });

  describe("i18n wiring (not a hardcoded literal)", () => {
    afterEach(() => {
      // Restore the real bundle so this override doesn't leak into other tests.
      i18n.addResourceBundle(
        "vi",
        "chat",
        { citationList: { source: "Source", sources: "Sources" } },
        true,
        true,
      );
    });

    it("reads the label from the chat i18n resource bundle at runtime", () => {
      i18n.addResourceBundle(
        "vi",
        "chat",
        { citationList: { source: "NGUON_KIEM_TRA" } },
        true,
        true,
      );

      render(
        <I18nextProvider i18n={i18n}>
          <CitationList citations={[{ title: "Doc A" }]} />
        </I18nextProvider>,
      );

      expect(screen.getByText("NGUON_KIEM_TRA")).toBeInTheDocument();
    });
  });
});
