// Copyright (c) the go-numerics authors.
// SPDX-License-Identifier: BSD-3-Clause
//
// Browser proof of the go-numerics playground, driven by browser_test.go.
//
// It loads the BUILT page (Hugo layout + the wasm compiled from this module)
// in headless Chrome, with the colour scheme FORCED, and drives it the way a
// visitor does: real mouse clicks at widget rectangles, real key presses, a
// real pointer over the plot. It checks what the app reports (numericsDebug)
// AND what is on the screen (canvas pixels), because a flag saying "shown" is
// not a picture.
//
// Usage: node driver.cjs <url> <chrome> <captureDir>
// Prints one line "RESULT <json>" and exits non-zero on the first failure.
"use strict";
const puppeteer = require("puppeteer-core");
const path = require("path");

const [url, chrome, captureDir] = process.argv.slice(2);
const results = [];
function check(name, ok, detail) {
  results.push({ name, ok: !!ok, detail: detail === undefined ? "" : String(detail) });
  if (!ok) throw new Error("FAILED: " + name + (detail !== undefined ? " — " + detail : ""));
}

// Theme backgrounds the canvas must paint (app.go Theme), as on the landing,
// and the accent the first curve is drawn in.
const BG = { light: [0xff, 0xff, 0xff], dark: [0x0b, 0x0e, 0x14] };
const ACCENT = { light: [0x74, 0x45, 0xad], dark: [0xb7, 0x94, 0xe4] };

const DESKTOP = { width: 1400, height: 900 };
const PHONE = { width: 390, height: 844, isMobile: true, hasTouch: true, name: "phone" };

async function run(scheme, dpr, vp = DESKTOP, opts = {}) {
  const browser = await puppeteer.launch({
    executablePath: chrome, headless: true,
    args: ["--no-sandbox", "--disable-gpu", "--blink-settings=preferredColorScheme=" + (scheme === "dark" ? 0 : 1)],
  });
  try {
    const page = await browser.newPage();
    const errors = [];
    page.on("pageerror", (e) => errors.push(String(e)));
    page.on("console", (m) => { if (m.type() === "error") errors.push(m.text()); });
    await page.setViewport({ width: vp.width, height: vp.height, deviceScaleFactor: dpr, isMobile: !!vp.isMobile, hasTouch: !!vp.hasTouch });
    await page.emulateMediaFeatures([{ name: "prefers-color-scheme", value: scheme }]);
    await page.evaluateOnNewDocument(() => { try { localStorage.removeItem("theme"); } catch (e) {} });
    await page.goto(url, { waitUntil: "load" });
    await page.waitForFunction(() => globalThis.numericsPlaygroundReady === true &&
      document.querySelector(".stage").classList.contains("ready"), { timeout: 60000 });
    const tag = scheme + "@" + dpr + "x" + (vp.name ? " " + vp.name : "") + ": ";
    const shot = (what) => path.join(captureDir, what + "-" + scheme + "-" + dpr + "x" + (vp.name ? "-" + vp.name : "") + ".png");
    const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

    const debug = async () => { const raw = await page.evaluate(() => numericsDebug()); try { return JSON.parse(raw); } catch (e) { throw new Error("numericsDebug returned no JSON (" + e.message + "): " + String(raw).slice(0, 200)); } };
    const boxOf = async () => page.evaluate(() => { const r = document.getElementById("num-canvas").getBoundingClientRect(); return { x: r.left, y: r.top, w: r.width, h: r.height }; });
    let canvasBox = await boxOf();
    // On a phone the canvas is taller than the window and the stage scrolls
    // (State.MinHeight): scroll a widget into view before clicking it, as a
    // visitor would, then re-read where the canvas now is.
    const reveal = async (r) => {
      await page.evaluate((y, h) => {
        const s = document.querySelector(".stage");
        if (y < s.scrollTop || y + h > s.scrollTop + s.clientHeight) s.scrollTop = Math.max(0, y + h / 2 - s.clientHeight / 2);
      }, r.y, r.h);
      canvasBox = await boxOf();
    };
    const rects = async () => JSON.parse(await page.evaluate(() => numericsRects()));
    const center = (r) => [canvasBox.x + r.x + r.w / 2, canvasBox.y + r.y + r.h / 2];
    const clickAt = async (x, y) => { await page.mouse.click(x, y); await sleep(60); };
    const click = async (name) => { const r = (await rects())[name]; await reveal(r); const [x, y] = center(r); await clickAt(x, y); };
    // Pixels of the canvas inside a CSS rectangle (relative to the canvas).
    const pixels = async (r) => page.evaluate((r, dpr) => {
      const c = document.getElementById("num-canvas");
      const ctx = c.getContext("2d");
      const d = ctx.getImageData(Math.round(r.x * dpr), Math.round(r.y * dpr), Math.max(1, Math.round(r.w * dpr)), Math.max(1, Math.round(r.h * dpr))).data;
      return Array.from(d);
    }, r, dpr);
    const near = (p, i, c, tol) => Math.abs(p[i] - c[0]) <= tol && Math.abs(p[i + 1] - c[1]) <= tol && Math.abs(p[i + 2] - c[2]) <= tol;
    const count = (p, c, tol) => { let n = 0; for (let i = 0; i < p.length; i += 4) if (near(p, i, c, tol)) n++; return n; };
    // Pick an option of a dropdown: open it, then click its row in the popover.
    const pick = async (dd, option) => {
      const d = await debug();
      const list = { families: d.families, cases: d.cases, methods: d.methods, plots: d.plotNames }[dd];
      const idx = list.indexOf(option);
      check(tag + "option " + option + " exists in " + dd, idx >= 0, list.join(","));
      await click(dd);
      const pop = (await rects())[dd + "Popover"];
      await reveal(pop);
      const visible = Math.min(list.length, 12);
      const rowH = pop.h / visible;
      await page.mouse.move(canvasBox.x + pop.x + pop.w / 2, canvasBox.y + pop.y + pop.h / 2);
      for (let i = 0; i < list.length; i++) await page.mouse.wheel({ deltaY: -40 });
      await sleep(40);
      let first = 0;
      while (idx >= first + visible) {
        await page.mouse.move(canvasBox.x + pop.x + pop.w / 2, canvasBox.y + pop.y + pop.h / 2);
        await page.mouse.wheel({ deltaY: 40 });
        await sleep(40);
        first++;
      }
      await clickAt(canvasBox.x + pop.x + pop.w / 2, canvasBox.y + pop.y + (idx - first + 0.5) * rowH);
    };
    // Replace the parameters: click in the box, delete what is there, type.
    const typeArgs = async (text) => {
      const before = (await debug()).args.length;
      await click("args");
      await page.keyboard.press("End");
      for (let i = 0; i < before + 5; i++) await page.keyboard.press("Backspace");
      await page.keyboard.type(text);
    };
    const num = (s) => parseFloat(s);
    const rowOf = (d, name) => { const r = d.rows.find((x) => x.Name === name); return r ? r.Value : undefined; };

    // 0. A capture of the opened page first, so a failure below still leaves
    //    the picture of what was on screen.
    await page.screenshot({ path: shot("loaded") });
    const fit = await page.evaluate(() => {
      const c = document.getElementById("num-canvas");
      const r = c.getBoundingClientRect();
      return { iw: innerWidth, dpr: devicePixelRatio, left: r.left, right: r.right, cw: c.width,
        scroll: document.documentElement.scrollWidth, client: document.documentElement.clientWidth };
    });
    check(tag + "the canvas fits the viewport width", fit.left >= 0 && fit.right <= vp.width + 0.5, fit.left + ".." + fit.right + " of " + vp.width);
    check(tag + "the page does not scroll horizontally", fit.scroll <= vp.width && fit.client <= vp.width, "scrollWidth " + fit.scroll + ", clientWidth " + fit.client + " of " + vp.width);
    check(tag + "the viewport is " + vp.width + " CSS px at " + dpr + "x", fit.iw === vp.width && fit.dpr === dpr, fit.iw + " @ " + fit.dpr);
    check(tag + "the canvas backing store matches the device pixels", Math.abs(fit.cw - Math.round((fit.right - fit.left) * dpr)) <= dpr, fit.cw);
    const all = await pixels({ x: 0, y: 0, w: canvasBox.w, h: Math.min(canvasBox.h, 400) });
    const painted = all.length / 4 - count(all, BG[scheme], 6);
    check(tag + "the app rendered something (non-background pixels)", painted > 1000, painted + " px");

    // 1. The page opened on the first problem, in the forced theme, already run.
    let d = await debug();
    check(tag + "opens on the scalar root cos x = x with Brentq", d.family === "Scalar root" && d.case === "cos x = x" && d.method === "Brentq", d.family + "/" + d.case + "/" + d.method);
    check(tag + "the canvas follows the forced theme", d.dark === (scheme === "dark"), d.dark);
    const root0 = rowOf(d, "root");
    check(tag + "the root is the Dottie number within Brent's xtol of 2e-12", Math.abs(num(root0) - 0.7390851332151607) < 2e-12, root0);
    check(tag + "the check against mathematics is on the page", Number(rowOf(d, "|root − reference|")) < 2e-12 && rowOf(d, "reference root") === "0.7390851332151607", rowOf(d, "|root − reference|"));
    check(tag + "the verdict is the library's", /^converged/.test(d.verdict), d.verdict);
    check(tag + "the time of one call is reported with the wasm caveat", /^one call: .* \(mean of \d+\), uninstrumented — WebAssembly, one thread$/.test(d.timing), d.timing);
    check(tag + "the footer says nothing is computed in JavaScript", /Nothing is computed in JavaScript/.test(d.status), d.status);
    check(tag + "GOMAXPROCS is 1 under js/wasm", d.maxProcs === 1, d.maxProcs);
    const corner = await pixels({ x: 2, y: 2, w: 1, h: 1 });
    check(tag + "canvas background is the theme's", near(corner, 0, BG[scheme], 2), corner.slice(0, 3));
    let rc = await rects();
    reveal(rc.plot);
    rc = await rects();
    const plotPx = await pixels(rc.plot);
    check(tag + "the curve is drawn in the accent colour", count(plotPx, ACCENT[scheme], 40) > 100, count(plotPx, ACCENT[scheme], 40) + " px");
    await page.screenshot({ path: shot("open") });

    // 2. The pointer over the plot: the exact value the library computed.
    await reveal(rc.plot);
    const pr = (await rects()).plot;
    await page.mouse.move(canvasBox.x + pr.x + pr.w * 0.4, canvasBox.y + pr.y + pr.h / 2);
    await sleep(80);
    d = await debug();
    const m = /^x = (\S+)\s+f = (\S+)/.exec(d.readout);
    check(tag + "hovering the plot gives a readout of x and f", d.hover && !!m, d.readout);
    const hx = num(m[1]), hf = num(m[2]);
    check(tag + "the readout's f is cos x − x at that x (to 1e-15)", Math.abs(hf - (Math.cos(hx) - hx)) < 1e-15, hx + " " + hf);
    check(tag + "the readout is the plotted point itself", d.series[0].x[d.hoverAt] === hx, d.hoverAt);
    await page.screenshot({ path: shot("hover") });
    const tb = (await rects()).table; // another widget: the pointer has left the plot
    await reveal(tb);
    await page.mouse.move(...center(tb));
    await sleep(60);
    d = await debug();
    check(tag + "moving off the plot clears the readout", !d.hover && /^Move the pointer/.test(d.readout), d.readout);

    // 3. Edit the parameters and Run: a loose tolerance, a looser root.
    await typeArgs("a=0\nb=2\nxtol=1e-3\nrtol=1e-3");
    await click("run");
    d = await debug();
    check(tag + "Run solved with the typed tolerance (a different root, close to the first)",
      rowOf(d, "root") !== root0 && Math.abs(num(rowOf(d, "root")) - 0.7390851332151607) < 2e-3, rowOf(d, "root"));
    check(tag + "the check against mathematics shows the larger error", Number(rowOf(d, "|root − reference|")) > 1e-12, rowOf(d, "|root − reference|"));

    // 4. The library's own error: a bracket without a sign change.
    await typeArgs("a=2\nb=3");
    await click("run");
    d = await debug();
    check(tag + "the library's error is shown as the library says it", /^the library says: .*sign/.test(d.error), d.error);
    const ep = await pixels((await rects()).error);
    const red = scheme === "dark" ? [0xf7, 0x8b, 0x8b] : [0xb4, 0x23, 0x18];
    check(tag + "the error is on screen in red", count(ep, red, 40) > 20, count(ep, red, 40) + " px");
    // The page's own error: a parameter that does not exist.
    await typeArgs("nonsense=1");
    await click("run");
    d = await debug();
    check(tag + "an unknown parameter is refused by name", /unknown parameter "nonsense"/.test(d.error) && d.rows === null, d.error);
    await click("defaults");
    await click("run");
    d = await debug();
    check(tag + "Defaults and Run bring the first result back, to the bit", rowOf(d, "root") === root0 && d.error === "", d.error);

    // 5. Another family through the menu: an ODE with an invariant to check.
    await pick("families", "ODE");
    d = await debug();
    check(tag + "the ODE family opened on Lotka–Volterra with RK45", d.family === "ODE" && d.case === "Lotka–Volterra" && d.method === "RK45", d.family + "/" + d.case + "/" + d.method);
    check(tag + "the invariant is conserved to the tolerance", Number(rowOf(d, "max drift of V")) < 1e-3, rowOf(d, "max drift of V"));
    check(tag + "two components are plotted", d.series.length === 2 && d.series[0].n > 20, JSON.stringify(d.series.map((s) => s.label + ":" + s.n)));
    await pick("plots", "phase portrait");
    d = await debug();
    check(tag + "the plot menu switched to the phase portrait", d.plotIndex === 1 && d.series[0].label === "orbit", d.plotIndex);
    await pick("methods", "DOP853");
    d = await debug();
    check(tag + "DOP853 ran the same problem", d.method === "DOP853" && /DOP853/.test(d.title), d.title);
    await page.screenshot({ path: shot("ode") });

    // 6. The code panel: Go program, then the SciPy equivalent, then Copy.
    check(tag + "the Go program is shown", d.code.startsWith("package main") && d.code.includes("integrate.SolveIVP"), d.code.slice(0, 40));
    const lang = (await rects()).lang;
    await reveal(lang);
    await clickAt(canvasBox.x + lang.x + lang.w * 3 / 4, canvasBox.y + lang.y + lang.h / 2); // Go | SciPy
    d = await debug();
    check(tag + "the SciPy tab shows SciPy", d.code.startsWith("import numpy as np") && d.code.includes("solve_ivp") && d.code.includes('method="DOP853"'), d.code.slice(0, 40));
    await browser.defaultBrowserContext().overridePermissions(new URL(url).origin, ["clipboard-read", "clipboard-write", "clipboard-sanitized-write"]);
    await click("copy");
    const clip = await page.evaluate(() => navigator.clipboard.readText());
    check(tag + "Copy copies the program shown", clip === d.code, clip.slice(0, 40));

    // 7. Special functions: a sweep, timed per point, with its identity check.
    await pick("families", "Special functions");
    await pick("cases", "J0(x)");
    d = await debug();
    check(tag + "a special function reports a sweep with a time per point", /^one sweep of 400 points: .* per point/.test(d.timing), d.timing);
    check(tag + "the Wronskian identity holds to rounding", Number(rowOf(d, d.rows.map((r) => r.Name).find((n) => /^max residual/.test(n)))) < 1e-13, JSON.stringify(d.rows.slice(-3)));
    await page.screenshot({ path: shot("special") });

    // 8. The page's theme toggle recolours the canvas (System -> Light -> Dark).
    await page.click("#theme-toggle"); // light
    await sleep(100);
    let px = await pixels({ x: 2, y: 2, w: 1, h: 1 });
    check(tag + "toggle to Light paints the light background", near(px, 0, BG.light, 2) && (await debug()).dark === false, px.slice(0, 3));
    await page.click("#theme-toggle"); // dark
    await sleep(100);
    px = await pixels({ x: 2, y: 2, w: 1, h: 1 });
    check(tag + "toggle to Dark paints the dark background", near(px, 0, BG.dark, 2) && (await debug()).dark === true, px.slice(0, 3));

    check(tag + "no page or console error", errors.length === 0, errors.join(" | "));
  } finally {
    await browser.close();
  }
}

(async () => {
  try {
    await run("light", 1, DESKTOP);
    await run("dark", 1);
    await run("light", 2);
    await run("dark", 2);
    await run("light", 3, PHONE);
    console.log("RESULT " + JSON.stringify({ ok: true, results }));
  } catch (e) {
    console.log("RESULT " + JSON.stringify({ ok: false, error: String(e.message || e), results }));
    process.exit(1);
  }
})();
