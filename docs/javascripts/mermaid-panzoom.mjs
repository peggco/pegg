/**
 * Pan & zoom for Mermaid diagrams.
 *
 * Zensical renders each diagram into a `<div class="mermaid">` whose SVG lives in a
 * *closed* shadow root, so the SVG itself cannot be reached from the page. We
 * therefore attach @panzoom/panzoom to the host element — transforming the host
 * also transforms its shadow content.
 *
 * Zensical has no native pan/zoom and does not run MkDocs plugins (so
 * `mkdocs-panzoom-plugin` is silently ignored), which is why this is wired up
 * through `extra_javascript`.
 *
 * The panzoom library is vendored under ./vendor/panzoom.mjs
 * (MIT, https://github.com/timmywil/panzoom).
 */

import Panzoom from "./vendor/panzoom.mjs";

const SELECTOR = "div.mermaid";
const MIN_SCALE = 0.4;
const MAX_SCALE = 20;
const HINT = "drag to pan · ctrl/⌘ + scroll to zoom · double-click to reset";

const STYLE = `
.pz-wrapper { position: relative; }
.pz-wrapper > div.mermaid { overflow: hidden; cursor: grab; user-select: none; }
.pz-wrapper > div.mermaid:active { cursor: grabbing; }
.pz-hint {
  position: absolute;
  right: 0.4rem;
  bottom: 0.4rem;
  padding: 0.1rem 0.45rem;
  border-radius: 0.25rem;
  font-size: 0.6rem;
  line-height: 1.5;
  color: var(--md-default-fg-color--light);
  background-color: var(--md-default-bg-color);
  border: 1px solid var(--md-default-fg-color--lightest);
  opacity: 0;
  transition: opacity 150ms ease-in-out;
  pointer-events: none;
  z-index: 1;
}
.pz-wrapper:hover .pz-hint { opacity: 1; }
`;

function injectStyle() {
  if (document.getElementById("mermaid-panzoom-style")) return;
  const style = document.createElement("style");
  style.id = "mermaid-panzoom-style";
  style.textContent = STYLE;
  document.head.append(style);
}

function wrap(el) {
  if (el.parentElement?.classList.contains("pz-wrapper")) return;
  const wrapper = document.createElement("div");
  wrapper.className = "pz-wrapper";
  el.replaceWith(wrapper);
  wrapper.append(el);
  const hint = document.createElement("span");
  hint.className = "pz-hint";
  hint.textContent = HINT;
  wrapper.append(hint);
}

function enhance(el) {
  if (el.dataset.pzReady === "1") return;
  el.dataset.pzReady = "1";

  wrap(el);

  const panzoom = Panzoom(el, {
    minScale: MIN_SCALE,
    maxScale: MAX_SCALE,
    cursor: "grab",
  });

  el.addEventListener(
    "wheel",
    (event) => {
      if (!(event.ctrlKey || event.metaKey)) return;
      event.preventDefault();
      panzoom.zoomWithWheel(event);
    },
    { passive: false }
  );

  el.addEventListener("dblclick", () => panzoom.reset());
}

function scan(root = document) {
  root.querySelectorAll(SELECTOR).forEach(enhance);
}

function boot() {
  injectStyle();
  scan();

  let scheduled = false;
  const observer = new MutationObserver(() => {
    if (scheduled) return;
    scheduled = true;
    requestAnimationFrame(() => {
      scheduled = false;
      scan();
    });
  });
  observer.observe(document.body, { childList: true, subtree: true });

  // Material/Zensical instant navigation, when available.
  if (window.document$ && typeof window.document$.subscribe === "function") {
    window.document$.subscribe(() => scan());
  }
}

if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", boot);
} else {
  boot();
}
