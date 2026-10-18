import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { I18nProvider } from "@multica/core/i18n/react";
import { RESOURCES } from "@multica/views/locales";
import { AuthRecoveryGate } from "./auth-recovery-gate";

const authState = vi.hoisted(() => ({
  status: "recovering" as "recovering" | "authenticated",
  recoveryCause: "database" as "database" | "network" | "service" | null,
  retryAuthentication: vi.fn(),
}));

vi.mock("@multica/core/auth", () => ({
  useAuthStore: (selector: (state: typeof authState) => unknown) =>
    selector(authState),
}));

function renderGate() {
  return render(
    <I18nProvider locale="en" resources={RESOURCES}>
      <AuthRecoveryGate>
        <div>workspace content</div>
      </AuthRecoveryGate>
    </I18nProvider>,
  );
}

describe("AuthRecoveryGate", () => {
  beforeEach(() => {
    authState.status = "recovering";
    authState.recoveryCause = "database";
    authState.retryAuthentication.mockReset();
  });

  it("identifies database unavailability and keeps recovery visible", () => {
    renderGate();

    expect(
      screen.getByRole("heading", { name: "Service temporarily unavailable" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/cannot reach its database right now/i),
    ).toBeInTheDocument();
    screen.getByRole("button", { name: "Try again" }).click();
    expect(authState.retryAuthentication).toHaveBeenCalledOnce();
    expect(screen.queryByText("workspace content")).not.toBeInTheDocument();
  });

  it("renders the application once authentication is healthy", () => {
    authState.status = "authenticated";
    renderGate();
    expect(screen.getByText("workspace content")).toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { name: "Service temporarily unavailable" }),
    ).not.toBeInTheDocument();
  });

  it("does not call a network outage a database outage", () => {
    authState.recoveryCause = "network";
    renderGate();

    expect(
      screen.getByRole("heading", { name: "Connection temporarily unavailable" }),
    ).toBeInTheDocument();
    expect(screen.getByText(/cannot reach the server right now/i)).toBeInTheDocument();
    expect(screen.queryByText(/cannot reach its database right now/i)).not.toBeInTheDocument();
  });
});
