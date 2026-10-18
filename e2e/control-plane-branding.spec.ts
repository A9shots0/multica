import { test, expect, type Page } from "@playwright/test";
import { mkdirSync } from "node:fs";
import { resolve } from "node:path";
import { loginAsDefault } from "./helpers";

// Rendered verification of the secondary product identity. The sidebar is
// user-resizable between 200px and 360px, so those are the real minimum and
// maximum content widths; 256px is the shipped default. Phone width is checked
// separately because the sidebar becomes an off-canvas sheet there.
const SIDEBAR_WIDTHS = [
  { name: "minimum", sidebar: 200 },
  { name: "default", sidebar: 256 },
  { name: "maximum", sidebar: 360 },
] as const;

const CONTROL_PLANE_NAME = "Silver Mediums Agent Control Plane";
const BUILT_ON = "Built on Multica";
const ABOUT_TRIGGER = "About this control plane";
const ABOUT_POINTS = [
  "Honest status",
  "Controlled routing",
  "Shared instructions",
  "Bounded permissions",
  "Independent review",
  "Release and runtime checks",
  "Recovery and rollback",
  "Your Inbox",
  "Usage awareness",
];

const SHOT_DIR = resolve(
  process.cwd(),
  ".qualification-evidence/adoption-20260916/branding-screenshots",
);
mkdirSync(SHOT_DIR, { recursive: true });

async function setSidebarWidth(page: Page, width: number) {
  await page.evaluate((value) => {
    window.localStorage.setItem("sidebar_width", String(value));
  }, width);
  await page.reload({ waitUntil: "domcontentloaded" });
  await expect(page.getByText(CONTROL_PLANE_NAME)).toBeVisible({
    timeout: 30000,
  });
}

// A truncated label still has to be the label: assert the element is clipped
// rather than overflowing its container or pushing the trigger out of view.
async function assertNoHorizontalOverflow(page: Page) {
  const overflow = await page.evaluate(() => {
    const doc = document.documentElement;
    return doc.scrollWidth - doc.clientWidth;
  });
  expect(overflow).toBeLessThanOrEqual(1);
}

test.describe("Silver Mediums control plane branding", () => {
  test("renders beside the Multica mark at minimum, default and maximum widths", async ({
    page,
  }) => {
    const slug = await loginAsDefault(page);
    await page.goto(`/${slug}/issues`, { waitUntil: "domcontentloaded" });

    for (const { name, sidebar } of SIDEBAR_WIDTHS) {
      await setSidebarWidth(page, sidebar);

      await expect(page.getByText(CONTROL_PLANE_NAME)).toBeVisible();
      await expect(page.getByText(BUILT_ON)).toBeVisible();

      const trigger = page.getByRole("button", { name: ABOUT_TRIGGER });
      await expect(trigger).toBeVisible();

      // The info trigger must stay inside the sidebar at every width.
      const triggerBox = await trigger.boundingBox();
      expect(triggerBox).not.toBeNull();
      expect(triggerBox!.x + triggerBox!.width).toBeLessThanOrEqual(sidebar + 24);

      await assertNoHorizontalOverflow(page);
      await page.screenshot({
        path: resolve(SHOT_DIR, `sidebar-${name}-${sidebar}px.png`),
        fullPage: false,
      });
    }
  });

  test("the About surface explains the control plane and keeps the Multica notices", async ({
    page,
  }) => {
    const slug = await loginAsDefault(page);
    await page.goto(`/${slug}/issues`, { waitUntil: "domcontentloaded" });
    await expect(page.getByText(CONTROL_PLANE_NAME)).toBeVisible({
      timeout: 30000,
    });

    await page.getByRole("button", { name: ABOUT_TRIGGER }).click();

    // The panel is taller than its scroll box, so it has to open at the top:
    // the heading and intro must be on screen before anything else is read.
    await expect(
      page.getByRole("heading", { name: CONTROL_PLANE_NAME }),
    ).toBeInViewport();
    await expect(
      page.getByText(
        "Multica is the product underneath. The control plane is the Silver Mediums layer that decides how agents are allowed to work on it.",
      ),
    ).toBeInViewport();

    for (const point of ABOUT_POINTS) {
      await expect(page.getByText(point, { exact: true })).toBeVisible();
    }
    await expect(
      page.getByText(
        "Built on Multica. The Multica name, logo and notices are shown unchanged.",
      ),
    ).toBeVisible();
    await expect(
      page.getByText(
        "Multica © 2025-2026 Multica, Inc. Licensed under the Multica License.",
      ),
    ).toBeVisible();
    await expect(
      page.getByRole("link", { name: "https://github.com/multica-ai/multica" }),
    ).toHaveAttribute("href", "https://github.com/multica-ai/multica");

    await assertNoHorizontalOverflow(page);
    await page.screenshot({
      path: resolve(SHOT_DIR, "about-popover-open.png"),
      fullPage: false,
    });
  });

  test("the About surface opens from the keyboard alone", async ({ page }) => {
    const slug = await loginAsDefault(page);
    await page.goto(`/${slug}/issues`, { waitUntil: "domcontentloaded" });
    const trigger = page.getByRole("button", { name: ABOUT_TRIGGER });
    await expect(trigger).toBeVisible({ timeout: 30000 });

    await expect(page.getByText("Honest status")).toHaveCount(0);
    await trigger.focus();
    await expect(trigger).toBeFocused();
    await page.keyboard.press("Enter");
    await expect(page.getByText("Honest status")).toBeVisible();

    await page.keyboard.press("Escape");
    await expect(page.getByText("Honest status")).toHaveCount(0);
  });

  test("fits a phone viewport without horizontal scrolling", async ({ page }) => {
    // Sign in at desktop width: below the md breakpoint the sidebar is an
    // off-canvas sheet, so the dashboard's own landmarks are not on screen.
    const slug = await loginAsDefault(page);
    await page.goto(`/${slug}/issues`, { waitUntil: "domcontentloaded" });
    await expect(page.getByText(CONTROL_PLANE_NAME)).toBeVisible({
      timeout: 30000,
    });
    await page.setViewportSize({ width: 390, height: 844 });
    await page.waitForLoadState("domcontentloaded");
    await assertNoHorizontalOverflow(page);
    await page.screenshot({
      path: resolve(SHOT_DIR, "phone-390px.png"),
      fullPage: false,
    });
  });

  test("shows the identity on the signed-out screen without altering the Multica title", async ({
    page,
  }) => {
    await page.goto("/login", { waitUntil: "domcontentloaded" });
    await expect(page.getByText("Sign in to Multica")).toBeVisible({
      timeout: 30000,
    });
    await expect(page.getByText(CONTROL_PLANE_NAME)).toBeVisible();
    await expect(page.getByText(BUILT_ON)).toBeVisible();
    await assertNoHorizontalOverflow(page);
    await page.screenshot({
      path: resolve(SHOT_DIR, "login-default.png"),
      fullPage: false,
    });

    await page.setViewportSize({ width: 390, height: 844 });
    await page.reload({ waitUntil: "domcontentloaded" });
    await expect(page.getByText(CONTROL_PLANE_NAME)).toBeVisible({
      timeout: 30000,
    });
    await assertNoHorizontalOverflow(page);
    await page.screenshot({
      path: resolve(SHOT_DIR, "login-phone-390px.png"),
      fullPage: false,
    });

    await page.setViewportSize({ width: 1920, height: 1080 });
    await page.reload({ waitUntil: "domcontentloaded" });
    await expect(page.getByText(CONTROL_PLANE_NAME)).toBeVisible({
      timeout: 30000,
    });
    await assertNoHorizontalOverflow(page);
    await page.screenshot({
      path: resolve(SHOT_DIR, "login-wide-1920px.png"),
      fullPage: false,
    });
  });
});
