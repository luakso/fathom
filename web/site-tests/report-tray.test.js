import { describe, it, expect, beforeEach } from "vitest";
import { createTray } from "../site/assets/js/lib/report-tray.js";

function trayDom() {
  document.body.innerHTML = `
    <span id="pincount"></span>
    <aside id="tray"><div id="pinlist"></div>
      <textarea id="thread"></textarea><span id="charcount"></span>
      <canvas id="xcard" width="1200" height="675"></canvas>
      <button id="traytoggle"></button><button id="trayclose"></button>
      <button id="regen"></button><button id="copythread"></button>
      <button id="cardprev"></button><button id="cardnext"></button><button id="carddl"></button>
    </aside>`;
}
const meta = { methodology_version: 1, data_through_day: "2026-06-06" };

beforeEach(trayDom);

describe("createTray", () => {
  it("adds a pin and reflects the count", () => {
    const t = createTray({ brand: "payees", meta });
    t.init();
    t.addPin({ title: "TEST", value: "$1.00", context: "ctx", denom: "denom" });
    expect(document.getElementById("pincount").textContent).toBe("1");
    expect(document.getElementById("pinlist").textContent).toContain("TEST");
  });
  it("builds a thread from pins with the brand and through-date in the header", () => {
    const t = createTray({ brand: "payees", meta });
    t.init();
    t.addPin({ title: "T", value: "$1.00", context: "ctx", denom: "d" });
    t.genThread();
    const text = document.getElementById("thread").value;
    expect(text).toContain("payees");
    expect(text).toContain("2026-06-06");
    expect(text).toContain("$1.00");
  });
  it("ignores a null pin (panel had nothing to pin)", () => {
    const t = createTray({ brand: "payees", meta });
    t.init();
    t.addPin(null);
    expect(document.getElementById("pincount").textContent).toBe("0");
  });
  it("renders the X-card without throwing (canvas present)", () => {
    const t = createTray({ brand: "payees", meta });
    t.init();
    t.addPin({ title: "T", value: "$1.00", context: "ctx", denom: "d" });
    expect(() => t.renderCard()).not.toThrow();
  });
});
