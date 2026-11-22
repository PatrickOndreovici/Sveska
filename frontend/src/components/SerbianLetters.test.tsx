import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useRef } from "react";
import { describe, expect, it, vi } from "vitest";
import { SerbianLetters } from "./SerbianLetters";

function Field({ onChange }: { onChange?: (value: string) => void }) {
  const field = useRef<HTMLInputElement>(null);
  return (
    <>
      <label htmlFor="word">Word</label>
      <input
        id="word"
        ref={field}
        onChange={(event) => {
          onChange?.(event.target.value);
        }}
      />
      <SerbianLetters target={() => field.current} />
      <button type="button">Next</button>
    </>
  );
}

describe("SerbianLetters", () => {
  it("types a letter where the cursor is and keeps the focus in the field", async () => {
    render(<Field />);
    const field = screen.getByLabelText<HTMLInputElement>("Word");
    await userEvent.type(field, "kua");
    field.setSelectionRange(2, 2);

    await userEvent.click(screen.getByRole("button", { name: "Type ć" }));
    expect(field).toHaveValue("kuća");
    expect(field).toHaveFocus();
    expect(field.selectionStart).toBe(3);
  });

  it("types over a selection, and a capital with Shift", async () => {
    render(<Field />);
    const field = screen.getByLabelText<HTMLInputElement>("Word");
    await userEvent.type(field, "x");
    field.setSelectionRange(0, 1);

    fireEvent.click(screen.getByRole("button", { name: "Type đ" }), { shiftKey: true });
    expect(field).toHaveValue("Đ");
  });

  it("tells React about the change", async () => {
    const onChange = vi.fn();
    render(<Field onChange={onChange} />);
    await userEvent.click(screen.getByRole("button", { name: "Type ž" }));
    expect(onChange).toHaveBeenLastCalledWith("ž");
  });

  it("stays out of the tab order", async () => {
    render(<Field />);
    await userEvent.click(screen.getByLabelText("Word"));
    await userEvent.tab();
    expect(screen.getByRole("button", { name: "Next" })).toHaveFocus();
  });
});
