import { expect, test } from "@playwright/test";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { configDir } from "../playwright.config";

test.describe.configure({ mode: "serial" });

test("setup, servers, users, tokens, and login", async ({ page }) => {
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto("/");
  await expect(page).toHaveURL(/\/setup$/);
  const token = readFileSync(join(configDir, "setup-token"), "utf8").trim();
  await page.getByLabel("Setup token").fill(token);
  await page.getByLabel("Username").fill("owner");
  await page.getByLabel("Password", { exact: true }).fill("ownerpass1");
  await page.getByLabel("Confirm password").fill("ownerpass1");
  await page.getByRole("button", { name: "Create admin account" }).click();

  await expect(page.getByText("Create your first server")).toBeVisible();

  await page.getByRole("link", { name: "New server" }).first().click();
  await expect(page.getByRole("heading", { name: "New server" })).toBeVisible();
  await expect(page.getByText(/GB RAM, \d+ CPU cores/)).toBeVisible({ timeout: 15_000 });
  const version = page.getByRole("combobox", { name: "Minecraft version" });
  await expect(version).toHaveText(/Latest/, { timeout: 15_000 });
  await version.click();
  await expect(page.getByRole("listbox")).toBeVisible();
  await expect(page.getByRole("option", { selected: true })).toHaveText(/Latest/);
  await page.keyboard.press("ArrowDown");
  await page.keyboard.press("ArrowDown");
  await page.keyboard.press("Enter");
  await expect(page.getByRole("listbox")).toHaveCount(0);
  await expect(version).not.toHaveText(/Latest/);
  await version.press("Escape");

  await page.getByLabel("Server name").fill("");
  await page.getByLabel("Server name").pressSequentially("My Server-1!");
  await expect(page.getByLabel("Server name")).toHaveValue("my-server-1-");
  await page.getByLabel("Server name").blur();
  await expect(page.getByLabel("Server name")).toHaveValue("my-server-1");

  await page.getByRole("link", { name: "People" }).click();
  await page.getByLabel("Username").fill("friend");
  await page.getByLabel("Password").fill("friendpass1");
  await page.getByRole("button", { name: "Add person" }).click();
  await expect(page.getByText("friend", { exact: true })).toBeVisible();

  // Row menus render above the page, so a card can't clip them.
  await page.getByRole("button", { name: "Actions for friend" }).click();
  const reset = page.getByRole("menuitem", { name: "Reset password" });
  await expect(reset).toBeVisible();
  const box = (await reset.boundingBox())!;
  const onTop = await page.evaluate(
    ({ x, y }) => document.elementFromPoint(x, y)?.closest('[role="menuitem"]')?.textContent?.trim(),
    { x: box.x + box.width / 2, y: box.y + box.height / 2 },
  );
  expect(onTop).toBe("Reset password");
  // Outside every card, so overflow-hidden on a short card can't cut it off.
  expect(await reset.evaluate((el) => el.closest('[role="menu"]')?.parentElement === document.body)).toBe(true);
  await reset.click();
  await expect(page.getByRole("dialog")).toContainText("New password for friend");
  await page.getByRole("dialog").getByRole("button", { name: "Cancel" }).click();

  await page.getByRole("link", { name: "Settings" }).click();
  await page.getByLabel("Token name").fill("ci");
  await page.getByRole("button", { name: "Create" }).click();
  await expect(page.getByText(/^mct_/)).toBeVisible();

  await page.getByRole("link", { name: "Activity" }).click();
  await expect(page.getByText("user.create")).toBeVisible();

  // Logging out asks first; cancelling keeps you in.
  await page.getByRole("button", { name: "Log out" }).click();
  await page.getByRole("dialog").getByRole("button", { name: "Cancel" }).click();
  await expect(page).not.toHaveURL(/\/login/);
  await page.getByRole("button", { name: "Log out" }).click();
  await page.getByRole("dialog").getByRole("button", { name: "Log out" }).click();
  await expect(page).toHaveURL(/\/login/);
  await page.getByLabel("Username").fill("friend");
  await page.getByLabel("Password").fill("wrong-password");
  await page.getByRole("button", { name: "Log in" }).click();
  await expect(page.getByText("wrong username or password")).toBeVisible();
  await page.getByLabel("Password").fill("friendpass1");
  await page.getByRole("button", { name: "Log in" }).click();
  await expect(page.getByText("No servers shared with you yet")).toBeVisible();
  await expect(page.getByRole("link", { name: "People" })).toHaveCount(0);

  expect(errors).toEqual([]);
});
