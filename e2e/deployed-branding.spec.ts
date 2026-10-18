import { test, expect, type Page } from "@playwright/test";
import { mkdirSync } from "node:fs";
import { resolve } from "node:path";

// Post-deployment verification against the running private instance. It is
// deliberately read-only and signed-out: it creates no account, no session and
// no data. The signed-in sidebar surface is verified separately against the
// byte-identical frontend image in the candidate preview.
const CONTROL_PLANE_NAME = "Silver Mediums Agent Control Plane";
const BUILT_ON = "Built on Multica";

const SHOT_DIR = resolve(
  process.cwd(),
  ".qualification-evidence/production-adoption-20260916/deployed-screenshots",
);
mkdirSync(SHOT_DIR, { recursive: true });

async function assertNoHorizontalOverflow(page: Page) {
  const overflow = await page.evaluate(
    () => document.documentElement.scrollWidth - document.documentElement.clientWidth,
  );
  expect(overflow).toBeLessThanOrEqual(1);
}

const WIDTHS = [
  { name: "phone", width: 390, height: 844 },
  { name: "default", width: 1280, height: 720 },
  { name: "wide", width: 1920, height: 1080 },
] as const;

test.describe("deployed private instance branding", () => {
  test("shows both identities on the sign-in screen at every credible width", async ({
    page,
  }) => {
    for (const { name, width, height } of WIDTHS) {
      await page.setViewportSize({ width, height });
      await page.goto("/login", { waitUntil: "domcontentloaded" });

      // Multica's own product name stays exactly where it was.
      await expect(page.getByText("Sign in to Multica")).toBeVisible({
        timeout: 30000,
      });
      await expect(page.getByText(CONTROL_PLANE_NAME)).toBeVisible();
      await expect(page.getByText(BUILT_ON)).toBeVisible();
      await expect(
        page.getByRole("button", { name: "About this control plane" }),
      ).toBeVisible();

      await assertNoHorizontalOverflow(page);
      await page.screenshot({
        path: resolve(SHOT_DIR, `login-${name}-${width}px.png`),
        fullPage: false,
      });
    }
  });

  test("the deployed About surface carries the Multica notices", async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 720 });
    await page.goto("/login", { waitUntil: "domcontentloaded" });
    await expect(page.getByText(CONTROL_PLANE_NAME)).toBeVisible({
      timeout: 30000,
    });

    await page.getByRole("button", { name: "About this control plane" }).click();
    await expect(
      page.getByRole("heading", { name: CONTROL_PLANE_NAME }),
    ).toBeInViewport();
    for (const point of [
      "Honest status",
      "Controlled routing",
      "Shared instructions",
      "Bounded permissions",
      "Independent review",
      "Release and runtime checks",
      "Recovery and rollback",
      "Your Inbox",
      "Usage awareness",
    ]) {
      await expect(page.getByText(point, { exact: true })).toBeVisible();
    }
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
      path: resolve(SHOT_DIR, "login-about-open.png"),
      fullPage: false,
    });
  });

  test("still reports the reviewed build and a healthy runtime", async ({
    request,
  }) => {
    const health = await request.get("http://127.0.0.1:8080/health");
    expect(health.ok()).toBe(true);
    const version = await health.json();
    expect(version.commit).toBe("60fea34b9770-local-24b0121ef7a0");

    const readiness = await request.get("http://127.0.0.1:8080/healthz");
    expect(readiness.ok()).toBe(true);
    expect(await readiness.json()).toMatchObject({
      status: "ok",
      checks: { db: "ok", migrations: "ok" },
    });
  });
});
