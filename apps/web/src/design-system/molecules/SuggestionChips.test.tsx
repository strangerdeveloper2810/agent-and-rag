import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import SuggestionChips from "./SuggestionChips";

describe("SuggestionChips", () => {
  it("renders suggestions and calls onSelect when clicked", () => {
    const suggestions = ["Chi tiết kiến trúc DB", "Thiết kế RESTful API"];

    const onSelect = vi.fn();
    render(<SuggestionChips suggestions={suggestions} onSelect={onSelect} />);

    expect(screen.getByText("Gợi ý bước tiếp theo:")).toBeInTheDocument();
    expect(screen.getByText("Chi tiết kiến trúc DB")).toBeInTheDocument();
    expect(screen.getByText("Thiết kế RESTful API")).toBeInTheDocument();

    fireEvent.click(screen.getByText("Chi tiết kiến trúc DB"));
    expect(onSelect).toHaveBeenCalledWith("Chi tiết kiến trúc DB");
  });

  it("renders null when suggestions is empty", () => {
    const { container } = render(
      <SuggestionChips suggestions={[]} onSelect={vi.fn()} />,
    );
    expect(container.firstChild).toBeNull();
  });
});
