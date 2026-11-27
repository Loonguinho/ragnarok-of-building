// Drives a headless Chrome over the DevTools protocol to test the web build
// (Node 24+, no packages: uses the built-in WebSocket).
//
//   google-chrome --headless=new --no-first-run --user-data-dir=/tmp/rob-chrome \
//     --enable-unsafe-swiftshader --use-angle=swiftshader --remote-debugging-port=9333 about:blank &
//   make web-serve &
//   node tools/webtest/shot.mjs 9333 http://localhost:8080/ /tmp/rob '[{"wait":15},{"shot":"start"}]'
//
// Steps (JSON list, run in order; each key is one action):
//   {"preload":"js"}        run js before the page's own scripts (put first)
//   {"throttle":{"down":bytesPerSec,"latency":ms}}  slow the network (applied before loading)
//   {"nocache":true}        disable the browser cache (a first visit)
//   {"wait":s}              sleep s seconds (the app takes ~15 s to load headless)
//   {"shot":"name"}         screenshot to <out-prefix>-name.png
//   {"eval":"js"}           evaluate in the page and print the result
//   {"click":[x,y]}         left click (1440x900 viewport)
//   {"key":{"key":"d","code":"KeyD","alt":true}}   key press (vk optional)
//   {"type":"text"}         insert text into the focused field
//   {"reload":true} / {"nav":"url"}
//   {"intercept":true}      catch file pickers instead of showing them
//   {"choose":["/abs/file"]}  answer the last caught picker with these files
// Console messages, exceptions and wasm/gz responses are printed as they come.
//
// Coordinates at 1440x900: Otimizador (85,212), Calcular (601,219),
// FOR field (228,196), Compartilhar menu (80,388), Importar replay (1383,880).
const [port, url, out, stepsJson] = process.argv.slice(2);
const steps = JSON.parse(stepsJson || "[]");
const fs = await import("node:fs");
const t = await (await fetch(`http://127.0.0.1:${port}/json/new?${encodeURIComponent("about:blank")}`, { method: "PUT" })).json();
const ws = new WebSocket(t.webSocketDebuggerUrl);
await new Promise(r => ws.onopen = r);
let id = 0; const chooser = []; const pending = new Map();
ws.onmessage = (m) => {
  const d = JSON.parse(m.data);
  if (d.id && pending.has(d.id)) { pending.get(d.id)(d); pending.delete(d.id); }
  if (d.method === "Runtime.consoleAPICalled") console.log("[console." + d.params.type + "]", d.params.args.map(a => a.value ?? a.description).join(" ").slice(0, 400));
  if (d.method === "Network.responseReceived" && /\.(wasm|gz)/.test(d.params.response.url)) console.log("[net]", d.params.response.status, d.params.response.url.replace(/^.*\//, ""), d.params.response.headers["Content-Type"] || d.params.response.headers["content-type"]);
  if (d.method === "Page.fileChooserOpened") { chooser.push(d.params); }
  if (d.method === "Runtime.exceptionThrown") console.log("[exception]", d.params.exceptionDetails.exception?.description?.slice(0, 600));
};
const send = (method, params = {}) => new Promise(r => { const i = ++id; pending.set(i, r); ws.send(JSON.stringify({ id: i, method, params })); });
await send("Runtime.enable"); await send("Page.enable");
// In front: a background tab gets its timers throttled, and Go in wasm
// switches goroutines through them.
await send("Page.bringToFront");
await send("Emulation.setFocusEmulationEnabled", { enabled: true });
await send("Emulation.setDeviceMetricsOverride", { width: 1440, height: 900, deviceScaleFactor: 1, mobile: false });
for (const s of steps) if (s.preload) await send("Page.addScriptToEvaluateOnNewDocument", { source: s.preload });
await send("Network.enable");
for (const s of steps) {
  if (s.throttle) await send("Network.emulateNetworkConditions", { offline: false, latency: s.throttle.latency || 0, downloadThroughput: s.throttle.down || -1, uploadThroughput: s.throttle.up || -1 });
  if (s.nocache) await send("Network.setCacheDisabled", { cacheDisabled: true });
}
await send("Page.navigate", { url });
const sleep = s => new Promise(r => setTimeout(r, s * 1000));
for (const s of steps) {
  if (s.wait) await sleep(s.wait);
  if (s.shot) { const r = await send("Page.captureScreenshot", { format: "png" }); fs.writeFileSync(`${out}-${s.shot}.png`, Buffer.from(r.result.data, "base64")); console.log("shot", s.shot); }
  if (s.eval) { const r = await send("Runtime.evaluate", { expression: s.eval, awaitPromise: true, returnByValue: true }); console.log("eval:", JSON.stringify(r.result?.result?.value ?? r.result)); }
  if (s.click) { for (const type of ["mouseMoved", "mousePressed", "mouseReleased"]) await send("Input.dispatchMouseEvent", { type, x: s.click[0], y: s.click[1], button: "left", clickCount: 1 }); await sleep(0.3); }
  if (s.key) { const k = s.key; const mods = k.alt ? 1 : 0; await send("Input.dispatchKeyEvent", { type: "keyDown", key: k.key, code: k.code, modifiers: mods, windowsVirtualKeyCode: k.vk || k.key.toUpperCase().charCodeAt(0) }); await send("Input.dispatchKeyEvent", { type: "keyUp", key: k.key, code: k.code, modifiers: mods, windowsVirtualKeyCode: k.vk || k.key.toUpperCase().charCodeAt(0) }); await sleep(0.3); }
  if (s.type) { await send("Input.insertText", { text: s.type }); await sleep(0.3); }
  if (s.reload) { await send("Page.reload"); }
  if (s.intercept !== undefined) { await send("Page.setInterceptFileChooserDialog", { enabled: s.intercept }); }
  if (s.choose) { // answer the last intercepted picker with these files ([] = nothing chosen)
    const c = chooser.pop(); console.log("chooser:", c ? "opened" : "NOT opened");
    if (c) { const r = await send("DOM.setFileInputFiles", { files: s.choose, backendNodeId: c.backendNodeId }); console.log("setFiles:", JSON.stringify(r.error || "ok")); }
  }
  if (s.nav) { await send("Page.navigate", { url: s.nav }); }
}
// Close the tab: one left open keeps running the program and slows the next runs.
await fetch(`http://127.0.0.1:${port}/json/close/${t.id}`).catch(() => {});
ws.close(); process.exit(0);
