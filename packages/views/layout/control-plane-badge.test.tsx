import type { ReactNode } from "react";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import enLayout from "../locales/en/layout.json";
import jaLayout from "../locales/ja/layout.json";
import koLayout from "../locales/ko/layout.json";
import zhLayout from "../locales/zh-Hans/layout.json";
import { ControlPlaneBadge } from "./control-plane-badge";

// react-i18next isn't initialised in the views test env, so resolve the
// selector against the real en/layout.json and assert on shipped copy.
vi.mock("../i18n", () => ({
  useT: () => ({
    t: (sel: (r: typeof enLayout) => string) => sel(enLayout),
  }),
}));

// Base UI's popover portals its popup; flattening the positioner/portal keeps
// the open-state behaviour (trigger click) while letting jsdom see the content.
vi.mock("@multica/ui/components/ui/popover", async () => {
  const { useState, createContext, useContext } = await import("react");
  const OpenContext = createContext<{
    open: boolean;
    setOpen: (value: boolean) => void;
  }>({ open: false, setOpen: () => {} });
  return {
    Popover: ({ children }: { children: ReactNode }) => {
      const [open, setOpen] = useState(false);
      return (
        <OpenContext.Provider value={{ open, setOpen }}>
          {children}
        </OpenContext.Provider>
      );
    },
    PopoverTrigger: ({
      children,
      ...props
    }: { children: ReactNode } & Record<string, unknown>) => {
      const { open, setOpen } = useContext(OpenContext);
      return (
        <button
          type="button"
          aria-expanded={open}
          onClick={() => setOpen(!open)}
          {...props}
        >
          {children}
        </button>
      );
    },
    PopoverContent: ({ children }: { children: ReactNode }) => {
      const { open } = useContext(OpenContext);
      return open ? <div role="dialog">{children}</div> : null;
    },
    PopoverHeader: ({ children }: { children: ReactNode }) => <div>{children}</div>,
    PopoverTitle: ({ children }: { children: ReactNode }) => <h2>{children}</h2>,
    PopoverDescription: ({ children }: { children: ReactNode }) => <p>{children}</p>,
  };
});

const ABOUT_KEYS = [
  "truthful_states",
  "routing",
  "shared_knowledge",
  "bounded_permissions",
  "independent_review",
  "release_verification",
  "recovery",
  "owner_inbox",
  "usage",
] as const;

describe("ControlPlaneBadge", () => {
  it("shows the secondary identity and the Multica attribution together", () => {
    render(<ControlPlaneBadge />);
    expect(
      screen.getByText("Silver Mediums Agent Control Plane"),
    ).toBeInTheDocument();
    expect(screen.getByText("Built on Multica")).toBeInTheDocument();
  });

  it("explains every capability the control plane adds", async () => {
    const user = userEvent.setup();
    render(<ControlPlaneBadge />);
    await user.click(
      screen.getByRole("button", { name: enLayout.control_plane.about_trigger }),
    );
    const about = screen.getByRole("dialog");
    expect(
      within(about).getByText(enLayout.control_plane.about_intro),
    ).toBeInTheDocument();
    for (const key of ABOUT_KEYS) {
      const point = enLayout.control_plane.points[key];
      expect(within(about).getByText(point.title)).toBeInTheDocument();
      expect(within(about).getByText(point.body)).toBeInTheDocument();
    }
  });

  it("keeps the Multica copyright, attribution and project link in the About surface", async () => {
    const user = userEvent.setup();
    render(<ControlPlaneBadge />);
    await user.click(
      screen.getByRole("button", { name: enLayout.control_plane.about_trigger }),
    );
    const about = screen.getByRole("dialog");
    expect(
      within(about).getByText(enLayout.control_plane.attribution),
    ).toBeInTheDocument();
    expect(
      within(about).getByText(enLayout.control_plane.copyright),
    ).toBeInTheDocument();
    const link = within(about).getByRole("link", {
      name: "https://github.com/multica-ai/multica",
    });
    expect(link).toHaveAttribute("href", "https://github.com/multica-ai/multica");
    expect(link).toHaveAttribute("rel", "noreferrer");
  });

  // Keyboard operation and the popup's real roles are verified against the
  // actual Base UI popover in e2e/control-plane-branding.spec.ts. Asserting
  // them here would only re-test this file's mock.
  it("labels the About trigger for assistive technology", () => {
    render(<ControlPlaneBadge />);
    expect(
      screen.getByRole("button", { name: enLayout.control_plane.about_trigger }),
    ).toHaveAttribute("aria-label", enLayout.control_plane.about_trigger);
  });

  it("keeps the full text of both truncating labels reachable on hover", () => {
    render(<ControlPlaneBadge />);
    expect(screen.getByText("Silver Mediums Agent Control Plane")).toHaveAttribute(
      "title",
      "Silver Mediums Agent Control Plane",
    );
    expect(screen.getByText("Built on Multica")).toHaveAttribute(
      "title",
      "Built on Multica",
    );
  });
});

describe("control plane copy", () => {
  // The Multica License keeps the product name and attribution intact. A
  // translation that drops "Multica" would quietly remove it from that locale.
  it.each([
    ["en", enLayout],
    ["ja", jaLayout],
    ["ko", koLayout],
    ["zh-Hans", zhLayout],
  ])("%s keeps the Multica name in the byline and attribution", (_locale, bundle) => {
    const control = (bundle as typeof enLayout).control_plane;
    expect(control.name).toBe("Silver Mediums Agent Control Plane");
    expect(control.built_on).toContain("Multica");
    expect(control.attribution).toContain("Multica");
    expect(control.copyright).toContain("Multica, Inc.");
    expect(control.copyright).toContain("2025-2026");
    for (const key of ABOUT_KEYS) {
      expect(control.points[key].title.length).toBeGreaterThan(0);
      expect(control.points[key].body.length).toBeGreaterThan(0);
    }
  });

  // Narrow layouts truncate the two label lines, so neither may be long enough
  // to make the 200px sidebar unreadable at its minimum width.
  it("keeps both sidebar labels short enough to read when truncated", () => {
    for (const bundle of [enLayout, jaLayout, koLayout, zhLayout]) {
      expect(bundle.control_plane.built_on.length).toBeLessThanOrEqual(24);
      expect(bundle.control_plane.name.length).toBeLessThanOrEqual(40);
    }
  });
});
