import { render, screen } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { beforeEach, describe, expect, it } from "vitest";
import i18n from "@/i18n";
import { Markdown } from "./Markdown";

const CODE_CONTENT = "```ts\nconst a = 1;\n```";

const renderMarkdown = () =>
  render(
    <I18nextProvider i18n={i18n}>
      <Markdown content={CODE_CONTENT} />
    </I18nextProvider>,
  );

describe("Markdown code block copy button", () => {
  beforeEach(async () => {
    await i18n.changeLanguage("vi");
  });

  it("shows the Vietnamese copy label sourced from the common namespace", () => {
    renderMarkdown();
    expect(screen.getByText("Sao chép")).toBeInTheDocument();
  });

  it("shows the English copy label when locale is en", async () => {
    await i18n.changeLanguage("en");
    renderMarkdown();
    expect(screen.getByText("Copy")).toBeInTheDocument();
  });

  it("renders Mermaid diagram block for mermaid code block", () => {
    const MERMAID_CONTENT = "```mermaid\ngraph TD\nA --> B\n```";
    render(
      <I18nextProvider i18n={i18n}>
        <Markdown content={MERMAID_CONTENT} />
      </I18nextProvider>,
    );
    expect(screen.getByText(/Sơ đồ kiến trúc/i)).toBeInTheDocument();
  });
});
