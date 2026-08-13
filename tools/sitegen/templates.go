package main

const sharedHead = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<style>
:root {
  --bg: #0d1117;
  --fg: #e6edf3;
  --muted: #8b949e;
  --accent: #58a6ff;
  --accent2: #3fb950;
  --border: #30363d;
  --card-bg: #161b22;
  --code-bg: #1c2128;
}
* { margin: 0; padding: 0; box-sizing: border-box; }
body {
  background: var(--bg);
  color: var(--fg);
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif;
  line-height: 1.6;
  font-size: 16px;
}
a { color: var(--accent); text-decoration: none; }
a:hover { text-decoration: underline; }
.container { max-width: 960px; margin: 0 auto; padding: 0 2rem; }

/* Nav */
nav {
  border-bottom: 1px solid var(--border);
  padding: 1rem 0;
  position: sticky;
  top: 0;
  background: var(--bg);
  z-index: 10;
}
nav .container { display: flex; align-items: center; gap: 2rem; }
nav .logo { font-weight: 700; font-size: 1.2rem; color: var(--fg); }
nav .links { display: flex; gap: 1.5rem; }
nav .links a { color: var(--muted); font-size: 0.9rem; }
nav .links a:hover, nav .links a.active { color: var(--fg); text-decoration: none; }

/* Hero */
.hero {
  padding: 4rem 0 3rem;
  text-align: center;
}
.hero h1 { font-size: 2.5rem; margin-bottom: 1rem; line-height: 1.2; }
.hero .tagline { font-size: 1.2rem; color: var(--muted); max-width: 600px; margin: 0 auto 2rem; }
.hero .badges { display: flex; gap: 0.75rem; justify-content: center; flex-wrap: wrap; }
.badge {
  display: inline-block;
  padding: 0.25rem 0.75rem;
  border-radius: 2rem;
  font-size: 0.8rem;
  font-weight: 600;
  border: 1px solid var(--border);
}
.badge.go { border-color: var(--accent); color: var(--accent); }
.badge.webgpu { border-color: var(--accent2); color: var(--accent2); }
.badge.pure { border-color: #d2a8ff; color: #d2a8ff; }

/* Feature cards */
.features {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 1.5rem;
  padding: 2rem 0;
}
.card {
  background: var(--card-bg);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 1.5rem;
}
.card h3 { margin-bottom: 0.5rem; font-size: 1.1rem; }
.card p { color: var(--muted); font-size: 0.9rem; }

/* Stats */
.stats {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
  gap: 1rem;
  padding: 2rem 0;
  text-align: center;
}
.stat-value { font-size: 2rem; font-weight: 700; color: var(--accent); }
.stat-label { font-size: 0.8rem; color: var(--muted); text-transform: uppercase; letter-spacing: 0.05em; }

/* Article */
.article-content { padding: 2rem 0 4rem; }
.article-content h1 { font-size: 2rem; margin: 2rem 0 1rem; padding-top: 1rem; }
.article-content h2 { font-size: 1.5rem; margin: 2rem 0 0.75rem; padding-top: 1rem; border-top: 1px solid var(--border); }
.article-content h3 { font-size: 1.2rem; margin: 1.5rem 0 0.5rem; }
.article-content h4 { font-size: 1rem; margin: 1rem 0 0.5rem; }
.article-content p { margin-bottom: 1rem; }
.article-content pre {
  background: var(--code-bg);
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 1rem;
  overflow-x: auto;
  margin-bottom: 1rem;
  font-size: 0.85rem;
  line-height: 1.5;
}
.article-content code {
  background: var(--code-bg);
  padding: 0.15em 0.35em;
  border-radius: 3px;
  font-size: 0.9em;
}
.article-content pre code { background: none; padding: 0; }
.article-content blockquote {
  border-left: 3px solid var(--accent);
  padding-left: 1rem;
  color: var(--muted);
  margin-bottom: 1rem;
}
.article-content table {
  width: 100%;
  border-collapse: collapse;
  margin-bottom: 1rem;
  font-size: 0.9rem;
}
.article-content th, .article-content td {
  border: 1px solid var(--border);
  padding: 0.5rem 0.75rem;
  text-align: left;
}
.article-content th { background: var(--card-bg); font-weight: 600; }
.article-content hr { border: none; border-top: 1px solid var(--border); margin: 2rem 0; }
.article-content ul, .article-content ol { margin: 0 0 1rem 1.5rem; }
.article-content li { margin-bottom: 0.25rem; }

/* TOC sidebar for article */
.article-layout { display: flex; gap: 2rem; }
.article-toc {
  width: 240px;
  flex-shrink: 0;
  position: sticky;
  top: 4rem;
  max-height: calc(100vh - 5rem);
  overflow-y: auto;
  font-size: 0.8rem;
  padding: 1rem 0;
}
.article-toc a { display: block; padding: 0.2rem 0; color: var(--muted); }
.article-toc a:hover { color: var(--accent); text-decoration: none; }
.article-toc .toc-h1 { font-weight: 600; margin-top: 0.5rem; }
.article-toc .toc-h2 { padding-left: 0.75rem; }
.article-body { flex: 1; min-width: 0; }

/* Footer */
footer {
  border-top: 1px solid var(--border);
  padding: 2rem 0;
  text-align: center;
  color: var(--muted);
  font-size: 0.8rem;
}

@media (max-width: 768px) {
  .hero h1 { font-size: 1.8rem; }
  .article-layout { flex-direction: column; }
  .article-toc { width: 100%; position: static; max-height: none; }
  .container { padding: 0 1rem; }
}
</style>
</head>
<body>
`

const indexTemplate = sharedHead + `<title>ironwail-go — Porting Quake to Go</title>
<nav>
<div class="container">
  <a href="index.html" class="logo">ironwail-go</a>
  <div class="links">
    <a href="index.html" class="active">Project</a>
    <a href="article.html">Development Article</a>
    <a href="https://github.com/darkliquid/ironwail-go">GitHub</a>
  </div>
</div>
</nav>

<div class="container">
  <section class="hero">
    <h1>Porting Quake to Go</h1>
    <p class="tagline">A pure-Go port of the Ironwail Quake engine with WebGPU rendering, zero CGO, and an interactive browser walkthrough.</p>
    <div class="badges">
      <span class="badge go">Pure Go</span>
      <span class="badge webgpu">WebGPU</span>
      <span class="badge pure">CGO_ENABLED=0</span>
    </div>
  </section>

  <section class="stats">
    <div><div class="stat-value">53+</div><div class="stat-label">Packages</div></div>
    <div><div class="stat-value">11</div><div class="stat-label">Chapters</div></div>
    <div><div class="stat-value">0</div><div class="stat-label">CGO Dependencies</div></div>
    <div><div class="stat-value">7+</div><div class="stat-label">AI Agents</div></div>
  </section>

  <section class="features">
    <div class="card">
      <h3>WebGPU Renderer</h3>
      <p>Canonical renderer built on gogpu, a pure-Go WebGPU implementation. Supports BSP world, alias models, sprites, particles, decals, sky, liquids, and HUD overlays.</p>
    </div>
    <div class="card">
      <h3>Browser Walkthrough</h3>
      <p>Full engine runs as WebAssembly in the browser with a 7-layer interactive inspection tour: Boot, Console, Host, Server, QuakeC, Client, Renderer.</p>
    </div>
    <div class="card">
      <h3>Parity Verification</h3>
      <p>Deterministic hash-based parity gates replace screenshot comparison. Dumpstate schema, render-record hashing, and message-stream recording prove bit-level C equivalence.</p>
    </div>
    <div class="card">
      <h3>QGo Compiler &amp; Debugger</h3>
      <p>QuakeGo dialect compiler with In-VM bytecode runner, resumable breakpoints, and headless REPL. Mod authors can test gameplay logic without booting the engine.</p>
    </div>
    <div class="card">
      <h3>Educational Codebase</h3>
      <p>Built to be read and learned from. Package doc.go lineage sections, per-subsystem guides, stage-by-stage renderer curriculum, and C citation conventions throughout.</p>
    </div>
    <div class="card">
      <h3>Multi-Agent Experiment</h3>
      <p>Field test of AI-assisted development across Copilot, Claude, GPT, GLM, Gemini, and Qwen under a human-as-architect model codified in AGENTS.md.</p>
    </div>
  </section>

  <section style="padding: 2rem 0;">
    <h2 style="margin-bottom: 1rem;">Architecture</h2>
    <p style="color: var(--muted); margin-bottom: 1rem;">The engine preserves Quake's client-server split even in single-player. The server is authoritative for physics, QuakeC, and entity state; the client parses server messages and presents player-visible state.</p>
    <div class="features" style="grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));">
      <div class="card"><h3>cmd/ironwailgo</h3><p>Entry point. Constructs Game, wires backends, parses flags.</p></div>
      <div class="card"><h3>internal/game</h3><p>Top-level coordinator. Owns all subsystems.</p></div>
      <div class="card"><h3>internal/host</h3><p>Startup/shutdown, command execution, frame timing.</p></div>
      <div class="card"><h3>internal/server</h3><p>Authoritative simulation: physics, collision, QC hooks.</p></div>
      <div class="card"><h3>internal/client</h3><p>Signon state, parsed entities, usercmd generation.</p></div>
      <div class="card"><h3>internal/renderer</h3><p>GoGPU/WebGPU rendering pipeline.</p></div>
      <div class="card"><h3>internal/qc</h3><p>QuakeC bytecode VM loader and executor.</p></div>
      <div class="card"><h3>internal/fs</h3><p>Quake virtual filesystem with pak support.</p></div>
    </div>
  </section>
</div>

<footer>
  <div class="container">
    <p>ironwail-go &mdash; A pure-Go port of the Ironwail Quake engine</p>
    <p style="margin-top: 0.5rem;"><a href="https://github.com/darkliquid/ironwail-go">GitHub</a> &middot; <a href="article.html">Read the full development article</a></p>
  </div>
</footer>
</body>
</html>`

const articleTemplate = sharedHead + `<title>Development Article — ironwail-go</title>
<nav>
<div class="container">
  <a href="index.html" class="logo">ironwail-go</a>
  <div class="links">
    <a href="index.html">Project</a>
    <a href="article.html" class="active">Development Article</a>
    <a href="https://github.com/darkliquid/ironwail-go">GitHub</a>
  </div>
</div>
</nav>

<div class="container">
  <div class="article-layout">
    <aside class="article-toc">
      <a href="#prologue" class="toc-h1">Prologue</a>
      <a href="#chapter-1" class="toc-h1">Ch 1: Quake Engine</a>
      <a href="#chapter-2" class="toc-h1">Ch 2: Go Divergence</a>
      <a href="#chapter-3" class="toc-h1">Ch 3: The Renderer</a>
      <a href="#chapter-4" class="toc-h1">Ch 4: Render Stages</a>
      <a href="#chapter-5" class="toc-h1">Ch 5: QuakeC VM</a>
      <a href="#chapter-6" class="toc-h1">Ch 6: GoGPU</a>
      <a href="#chapter-7" class="toc-h1">Ch 7: Synthesis</a>
      <a href="#chapter-8" class="toc-h1">Ch 8: Hardening Days</a>
      <a href="#chapter-9" class="toc-h1">Ch 9: Browser Frontier</a>
      <a href="#chapter-10" class="toc-h1">Ch 10: Parity Gap</a>
      <a href="#chapter-11" class="toc-h1">Ch 11: Compiler Grows Up</a>
      <a href="#ref-consolidated" class="toc-h1">References</a>
    </aside>
    <main class="article-body article-content">
{{.ArticleHTML}}
    </main>
  </div>
</div>

<footer>
  <div class="container">
    <p>ironwail-go &mdash; A pure-Go port of the Ironwail Quake engine</p>
    <p style="margin-top: 0.5rem;"><a href="index.html">Back to project overview</a> &middot; <a href="https://github.com/darkliquid/ironwail-go">GitHub</a></p>
  </div>
</footer>
</body>
</html>`
