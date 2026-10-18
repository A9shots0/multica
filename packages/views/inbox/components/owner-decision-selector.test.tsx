/** @vitest-environment jsdom */
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import enInbox from "../../locales/en/inbox.json";
import { OwnerDecisionSelector } from "./owner-decision-selector";

vi.mock("../../i18n", () => ({
  useT: () => ({
    t: (selector: (resource: typeof enInbox) => string) => selector(enInbox),
  }),
}));

const choices = [
  { label: "Rebuild the review gate", directive: "correct" as const, send: "Rebuild it.", primary: true },
  { label: "Inspect more evidence", directive: "request_changes" as const, send: "Inspect more." },
  { label: "Stop this request", directive: "cancel" as const, send: "Stop it.", danger: true },
];

describe("OwnerDecisionSelector", () => {
  it("selects a row, accepts optional context, and continues only after explicit confirmation", () => {
    const onContinue = vi.fn();
    const onNoteChange = vi.fn();
    render(
      <OwnerDecisionSelector
        choices={choices}
        descriptions={["Starts the rebuild.", "Keeps it paused.", "Closes the request."]}
        disabled={false}
        note=""
        onNoteChange={onNoteChange}
        onContinue={onContinue}
      />,
    );

    const recommended = screen.getByRole("radio", { name: /Rebuild the review gate/ });
    const inspect = screen.getByRole("radio", { name: /Inspect more evidence/ });
    const continueButton = screen.getByRole("button", { name: "Continue" });
    expect(recommended).toHaveAttribute("aria-checked", "false");
    expect(continueButton).toBeDisabled();
    expect(screen.queryByRole("textbox", { name: "Add a comment (optional)" })).not.toBeInTheDocument();

    fireEvent.click(inspect);
    expect(inspect).toHaveAttribute("aria-checked", "true");
    expect(recommended).toHaveAttribute("aria-checked", "false");
    expect(onContinue).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: "Add a comment (optional)" }));
    fireEvent.change(screen.getByRole("textbox", { name: "Add a comment (optional)" }), {
      target: { value: "Keep the request paused while checking the visual standard." },
    });
    expect(onNoteChange).toHaveBeenCalledWith("Keep the request paused while checking the visual standard.");

    fireEvent.click(continueButton);
    expect(onContinue).toHaveBeenCalledTimes(1);
    expect(onContinue).toHaveBeenCalledWith(choices[1]);
  });
});
