// browser_env.ts — browser-walkthrough DOM/canvas polyfill for Deno tests.
//
// The engine wasm needs `document`/`window`/`canvas` for its browser path
// (main_wasm.go + gogpu's browser platform). Deno has none of these — this
// installs minimal stand-ins backed by Deno's real OffscreenCanvas + WebGPU
// so gogpu's browser-surface path can run. It mirrors what a real page
// provides: querySelector("canvas"), createElement, body, title, events,
// requestAnimationFrame (driven manually by the test), matchMedia.

export interface BrowserEnv {
  /** Drive one rAF tick (fire all queued callbacks with a timestamp). */
  tick(): void;
  /** Number of rAF callbacks currently queued. */
  pending(): number;
  /** The fake canvas element (has getContext("webgpu"/"2d")). */
  canvas(): any;
}

export function installBrowserEnv(canvasW = 320, canvasH = 200): BrowserEnv {
  const rafQ: Function[] = [];
  let canvasEl: any = null;

  const makeCanvasElement = () => {
    const c = new OffscreenCanvas(canvasW, canvasH);
    const wgpuCtx = c.getContext("webgpu");
    const style: any = {};
    canvasEl = new Proxy(
      {} as any,
      {
        get(_t, p) {
          switch (p) {
            case "getContext":
              return (type: string) =>
                type === "webgpu" ? wgpuCtx : type === "2d" ? c.getContext("2d") : null;
            case "style":
              return style;
            case "addEventListener":
            case "removeEventListener":
            case "setAttribute":
            case "requestPointerLock":
            case "focus":
              return () => {};
            case "clientWidth":
            case "offsetWidth":
              return c.width;
            case "clientHeight":
            case "offsetHeight":
              return c.height;
            case "width":
              return c.width;
            case "height":
              return c.height;
            case "tabIndex":
              return 0;
            default:
              return Reflect.get(_t, p);
          }
        },
        set(_t, p, v) {
          if (p === "width" || p === "height") {
            c[p] = v;
            return true;
          }
          return Reflect.set(_t, p, v);
        },
      },
    );
    return canvasEl;
  };

  (globalThis as any).document = {
    querySelector: (sel: string) => (sel === "canvas" ? (canvasEl ?? makeCanvasElement()) : null),
    createElement: () => makeCanvasElement(),
    body: { appendChild: () => {} },
    title: "ironwail-harness",
    addEventListener: () => {},
    removeEventListener: () => {},
    exitPointerLock: () => {},
    getElementById: () => null,
  };

  (globalThis as any).window = new Proxy(
    {} as any,
    {
      get(_t, p) {
        switch (p) {
          case "requestAnimationFrame":
            return (cb: Function) => {
              rafQ.push(cb);
              return rafQ.length;
            };
          case "cancelAnimationFrame":
            return () => {};
          case "addEventListener":
          case "removeEventListener":
            return () => {};
          case "matchMedia":
            return () => ({ matches: false });
          default:
            return Reflect.get(_t, p);
        }
      },
    },
  );

  return {
    tick() {
      const q = rafQ.splice(0);
      for (const cb of q) {
        try {
          cb(performance.now());
        } catch {
          // a rAF callback failing must not kill the test driver
        }
      }
    },
    pending: () => rafQ.length,
    canvas: () => canvasEl ?? makeCanvasElement(),
  };
}
