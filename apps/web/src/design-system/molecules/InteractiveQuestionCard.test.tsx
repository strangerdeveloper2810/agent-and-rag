import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import i18n from "@/i18n";
import InteractiveQuestionCard from "./InteractiveQuestionCard";
import type { ClarifyQuestion } from "@/types";

describe("InteractiveQuestionCard", () => {
  beforeEach(async () => {
    await i18n.changeLanguage("vi");
  });

  it("renders single-select question with options and custom input", () => {
    const questions: ClarifyQuestion[] = [
      {
        prompt: "Chọn framework mục tiêu?",
        header: "Tech Stack",
        options: [
          {
            label: "React / Next.js",
            description: "Dành cho Web",
            recommended: true,
          },
          { label: "Flutter", description: "Dành cho Mobile" },
        ],
        multiSelect: false,
      },
    ];

    const onSubmit = vi.fn();
    render(
      <InteractiveQuestionCard questions={questions} onSubmit={onSubmit} />,
    );

    expect(screen.getByText("Tech Stack")).toBeInTheDocument();
    expect(screen.getByText("Chọn framework mục tiêu?")).toBeInTheDocument();
    expect(screen.getByText("React / Next.js")).toBeInTheDocument();
    expect(screen.getByText("Khuyến nghị")).toBeInTheDocument();

    // Click on option
    fireEvent.click(screen.getByText("React / Next.js"));
    expect(onSubmit).toHaveBeenCalledWith(
      "Q: Chọn framework mục tiêu?\nA: React / Next.js",
    );
  });

  it("supports write-in custom text input", () => {
    const questions: ClarifyQuestion[] = [
      {
        prompt: "Chọn ngôn ngữ?",
        options: [{ label: "Go" }, { label: "Rust" }],
      },
    ];

    const onSubmit = vi.fn();
    render(
      <InteractiveQuestionCard questions={questions} onSubmit={onSubmit} />,
    );

    const input = screen.getByPlaceholderText(/Hoặc nhập phương án/i);
    fireEvent.change(input, { target: { value: "Python FastAPI" } });
    fireEvent.click(screen.getByText("Gửi"));

    expect(onSubmit).toHaveBeenCalledWith(
      "Q: Chọn ngôn ngữ?\nA: Python FastAPI",
    );
  });

  it("handles multi-select questions with checkboxes", () => {
    const questions: ClarifyQuestion[] = [
      {
        prompt: "Chọn các tính năng cần có?",
        options: [
          { label: "Auth / Login" },
          { label: "Payment Gateway" },
          { label: "Push Notification" },
        ],
        multiSelect: true,
      },
    ];

    const onSubmit = vi.fn();
    render(
      <InteractiveQuestionCard questions={questions} onSubmit={onSubmit} />,
    );

    expect(screen.getByText("Chọn nhiều")).toBeInTheDocument();

    // Toggle 2 options
    fireEvent.click(screen.getByText("Auth / Login"));
    fireEvent.click(screen.getByText("Payment Gateway"));

    // Submit button
    const submitBtn = screen.getByText("Hoàn tất lựa chọn");
    fireEvent.click(submitBtn);

    expect(onSubmit).toHaveBeenCalledWith(
      "Q: Chọn các tính năng cần có?\nA: Auth / Login, Payment Gateway",
    );
  });

  it("handles multi-step wizard questions with Back and Next", () => {
    const questions: ClarifyQuestion[] = [
      {
        prompt: "Bước 1: Chọn OS?",
        header: "OS",
        options: [{ label: "Linux" }, { label: "Windows" }],
      },
      {
        prompt: "Bước 2: Chọn DB?",
        header: "Database",
        options: [{ label: "Postgres" }, { label: "Mongo" }],
      },
    ];

    const onSubmit = vi.fn();
    render(
      <InteractiveQuestionCard questions={questions} onSubmit={onSubmit} />,
    );

    expect(screen.getByText("Câu 1 / 2")).toBeInTheDocument();
    expect(screen.getByText("Bước 1: Chọn OS?")).toBeInTheDocument();

    // Pick Step 1
    fireEvent.click(screen.getByText("Linux"));

    // Should transition to Step 2
    expect(screen.getByText("Câu 2 / 2")).toBeInTheDocument();
    expect(screen.getByText("Bước 2: Chọn DB?")).toBeInTheDocument();

    // Pick Step 2
    fireEvent.click(screen.getByText("Postgres"));

    expect(onSubmit).toHaveBeenCalledWith(
      "Q: Bước 1: Chọn OS?\nA: Linux\n\nQ: Bước 2: Chọn DB?\nA: Postgres",
    );
  });

  it("renders in English when language is switched to en", async () => {
    await i18n.changeLanguage("en");
    const questions: ClarifyQuestion[] = [
      {
        prompt: "Select cloud provider?",
        header: "Infrastructure",
        options: [
          {
            label: "AWS",
            description: "Amazon Web Services",
            recommended: true,
          },
          { label: "GCP" },
        ],
        multiSelect: false,
      },
    ];

    const onSubmit = vi.fn();
    render(
      <InteractiveQuestionCard questions={questions} onSubmit={onSubmit} />,
    );

    expect(screen.getByText("Infrastructure")).toBeInTheDocument();
    expect(screen.getByText("Select cloud provider?")).toBeInTheDocument();
    expect(screen.getByText("Recommended")).toBeInTheDocument();
    expect(
      screen.getByPlaceholderText(/Or type your custom answer/i),
    ).toBeInTheDocument();
    expect(screen.getByText("Send")).toBeInTheDocument();
  });
});
