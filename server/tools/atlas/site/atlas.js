// Lumilio Atlas site. All data comes from atlas-data.js (window.ATLAS), which
// `task atlas` regenerates from source; this file only renders it.
(() => {
  "use strict";

  const A = window.ATLAS;
  const $ = (selector, root = document) => root.querySelector(selector);
  const content = $("#content");
  const sidebar = $("#sidebar");

  const KIND_TITLES = {
    architecture: "Architecture",
    sequence: "Sequence",
    dataflow: "Data flow",
    lifecycle: "Lifecycle",
  };
  const SIDE_TITLES = { server: "Server", desktop: "Desktop", web: "Web" };

  const viewsById = new Map(A.views.map((view) => [view.id, view]));
  const modulesById = new Map(A.modules.map((module) => [module.id, module]));
  const groupsById = new Map(A.groups.map((group) => [group.id, group]));
  const staleByView = new Map();
  for (const item of A.stale) {
    const view = A.views.find((candidate) => candidate.source === item.where);
    if (view) staleByView.set(view.id, (staleByView.get(view.id) ?? 0) + 1);
  }

  // ------------------------------------------------------------- utilities

  const escapeHTML = (value) =>
    String(value ?? "").replace(
      /[&<>"']/g,
      (character) =>
        ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[character],
    );

  const store = {
    get(key, fallback) {
      try {
        return localStorage.getItem(key) ?? fallback;
      } catch {
        return fallback;
      }
    },
    set(key, value) {
      try {
        localStorage.setItem(key, value);
      } catch {
        // Storage can be unavailable; the setting simply does not persist.
      }
    },
  };

  let openIn = store.get("atlas-open-in", "vscode");

  function sourceURL(file, line) {
    if (!file) return null;
    if (openIn === "github") {
      return `${A.repo.github}/blob/${A.repo.branch}/${file}${line ? `#L${line}` : ""}`;
    }
    return `vscode://file/${A.repo.root}/${file}${line ? `:${line}` : ""}`;
  }

  function sourceLink(file, line, label) {
    const url = sourceURL(file, line);
    if (!url) return "";
    const text = label ?? `${file}${line ? `:${line}` : ""}`;
    return `<a class="loc" href="${escapeHTML(url)}" title="Open ${escapeHTML(file)}">${escapeHTML(text)}</a>`;
  }

  function moduleLink(id, label) {
    if (!modulesById.has(id)) return `<code>${escapeHTML(label ?? id)}</code>`;
    return `<a href="#/module/${encodeURI(id)}"><code>${escapeHTML(label ?? id)}</code></a>`;
  }

  function anchorHTML(anchor) {
    if (!anchor) return '<span class="muted">—</span>';
    const resolved = A.anchors[anchor];
    const parts = [`<code>${escapeHTML(anchor)}</code>`];
    if (resolved) {
      if (resolved.kind === "group") {
        parts[0] = `<a href="#/view/group-${encodeURI(anchor.slice(6))}"><code>${escapeHTML(anchor)}</code></a>`;
      } else if (resolved.kind === "mod") {
        parts[0] = `<a href="#/module/${encodeURI(resolved.module)}"><code>${escapeHTML(anchor)}</code></a>`;
      }
      if (resolved.file) parts.push(sourceLink(resolved.file, resolved.line));
      if (resolved.module && resolved.kind !== "mod") {
        parts.push(
          `<a class="loc" href="#/module/${encodeURI(resolved.module)}">module</a>`,
        );
      }
      if (resolved.stale) parts.push('<span class="chip stale">re-verify</span>');
    }
    return `<span class="anchor">${parts.join("")}</span>`;
  }

  function kindChip(kind) {
    return `<span class="chip kind-${kind}"><span class="dot ${kind}"></span>${KIND_TITLES[kind] ?? kind}</span>`;
  }

  function parseRoute() {
    const hash = decodeURI(location.hash.replace(/^#/, "")) || "/";
    const [path, query = ""] = hash.split("?");
    const params = new URLSearchParams(query);
    const parts = path.split("/").filter(Boolean);
    return { parts, params, path };
  }

  // --------------------------------------------------------------- sidebar

  function renderSidebar() {
    const { path } = parseRoute();
    const link = (href, label, dot) => {
      const active = `#${path}` === href ? " active" : "";
      return `<a class="${active.trim()}" href="${href}">${dot ? `<span class="dot ${dot}"></span>` : ""}${escapeHTML(label)}</a>`;
    };
    let html = "<h3>Atlas</h3>";
    html += link("#/", "Overview");
    html += link("#/health", `Health${A.problems.length + A.stale.length ? ` (${A.problems.length + A.stale.length})` : ""}`);
    for (const kind of Object.keys(KIND_TITLES)) {
      const views = A.views.filter((view) => view.category === kind && !view.id.startsWith("group-"));
      html += `<h3>${KIND_TITLES[kind]}</h3>`;
      for (const view of views) html += link(`#/view/${view.id}`, view.title, kind);
      if (kind === "architecture") {
        for (const side of Object.keys(SIDE_TITLES)) {
          const groupViews = A.views.filter(
            (view) => view.id.startsWith("group-") && groupsById.get(view.id.slice(6))?.side === side,
          );
          const open = groupViews.some((view) => `#${path}` === `#/view/${view.id}`) ? " open" : "";
          html += `<details${open}><summary>${SIDE_TITLES[side]} groups</summary><div>`;
          for (const view of groupViews) html += link(`#/view/${view.id}`, view.title, "architecture");
          html += "</div></details>";
        }
      }
    }
    html += "<h3>Modules</h3>";
    for (const side of Object.keys(SIDE_TITLES)) {
      const groups = A.groups.filter((group) => group.side === side).sort((a, b) => b.layer - a.layer);
      const activeModule = path.startsWith("/module/") ? path.slice(8) : "";
      const sideOpen = modulesById.get(activeModule)?.side === side ? " open" : "";
      html += `<details${sideOpen}><summary>${SIDE_TITLES[side]}</summary><div>`;
      for (const group of groups) {
        const modules = A.modules.filter((module) => module.group === group.id);
        const groupOpen = modulesById.get(activeModule)?.group === group.id ? " open" : "";
        html += `<details${groupOpen}><summary>${escapeHTML(group.title)}</summary><div>`;
        for (const module of modules) html += link(`#/module/${module.id}`, shortModule(module));
        html += "</div></details>";
      }
      html += "</div></details>";
    }
    sidebar.innerHTML = html;
  }

  function shortModule(module) {
    const parts = module.id.split("/");
    if (module.side === "web") return parts.slice(2).join("/") || module.id;
    if (parts[1] === "internal") return parts.slice(2).join("/");
    return parts.slice(1).join("/") || module.id;
  }

  // ---------------------------------------------------------------- health

  function renderHealthPill() {
    const pill = $("#health-pill");
    const problems = A.problems.length;
    const stale = A.stale.length;
    if (problems + stale === 0) {
      pill.className = "health-pill ok";
      pill.textContent = "✓ in sync";
    } else {
      pill.className = "health-pill bad";
      pill.textContent = `${problems} problems · ${stale} stale`;
    }
  }

  function problemList(items, empty) {
    if (!items.length) return `<div class="empty">${empty}</div>`;
    return items
      .map(
        (item) =>
          `<div class="problem"><code>${escapeHTML(item.where)}</code>${escapeHTML(item.message)}</div>`,
      )
      .join("");
  }

  function renderHealth() {
    return `<div class="page">
      <div class="eyebrow">Atlas health</div>
      <h1>Is the map still the territory?</h1>
      <p class="lede">Problems are broken anchors, undeclared dependencies, or missing package docs. Stale anchors point at code that changed after its view was last verified: re-read the view against the code, fix it if behaviour changed, then run <code>task atlas:lock</code>. CI fails on both.</p>
      <h2>Problems (${A.problems.length})</h2>
      <div class="card">${problemList(A.problems, "No problems. Every anchor resolves and every dependency is declared.")}</div>
      <h2>Stale anchors (${A.stale.length})</h2>
      <div class="card">${problemList(A.stale, "Nothing stale. Every authored view was verified against the current code.")}</div>
    </div>`;
  }

  // -------------------------------------------------------------- overview

  function renderOverview() {
    const authored = A.views.filter((view) => !view.derived);
    const lines = A.modules.reduce((sum, module) => sum + module.lines, 0);
    const tiles = (kind) =>
      A.views
        .filter((view) => view.category === kind && !view.id.startsWith("group-"))
        .map(
          (view) => `<a class="card tile" href="#/view/${view.id}">
            ${kindChip(view.kind)} ${view.derived ? '<span class="chip derived">derived</span>' : ""}
            ${staleByView.has(view.id) ? '<span class="chip stale">re-verify</span>' : ""}
            <h4>${escapeHTML(view.title)}</h4><p>${escapeHTML(view.summary)}</p></a>`,
        )
        .join("");
    let html = `<div class="page">
      <div class="eyebrow">Lumilio Photos</div>
      <h1>Atlas</h1>
      <p class="lede">A map of the codebase that cannot silently drift. Architecture views are derived from the import graph; sequence, data-flow, and lifecycle views are authored, but every element is anchored to a real symbol, API operation, or table, and CI fails when an anchor breaks or its code changes without the view being re-verified.</p>
      <div class="stats">
        <div class="card stat"><b>${A.modules.length}</b><span>modules</span></div>
        <div class="card stat"><b>${A.groups.length}</b><span>groups</span></div>
        <div class="card stat"><b>${authored.length}</b><span>authored views</span></div>
        <div class="card stat"><b>${Object.keys(A.anchors).length}</b><span>anchors</span></div>
        <div class="card stat"><b>${Math.round(lines / 1000)}k</b><span>lines mapped</span></div>
      </div>`;
    for (const kind of Object.keys(KIND_TITLES)) {
      html += `<h2>${KIND_TITLES[kind]}</h2><div class="grid">${tiles(kind)}</div>`;
    }
    return `${html}</div>`;
  }

  // ------------------------------------------------------------------ views

  function elementTable(view) {
    if (view.kind === "sequence") {
      const participants = view.nodes
        .map((node) => `<tr><td>${escapeHTML(node.label)}</td><td>${anchorHTML(node.anchor)}</td></tr>`)
        .join("");
      const steps = (view.rows ?? [])
        .map(
          (row) => `<tr><td>${row.number}</td><td class="muted">${escapeHTML(row.from)} → ${escapeHTML(row.to)}</td>
            <td>${row.block ? `<span class="muted">${escapeHTML(row.block)} — </span>` : ""}${escapeHTML(row.label)}</td>
            <td>${anchorHTML(row.anchor)}</td></tr>`,
        )
        .join("");
      return `<h2>Participants</h2><div class="card table-card"><table><thead><tr><th>Participant</th><th>Anchor</th></tr></thead><tbody>${participants}</tbody></table></div>
        <h2>Steps</h2><div class="card table-card"><table><thead><tr><th>#</th><th>From → To</th><th>Message</th><th>Anchor</th></tr></thead><tbody>${steps}</tbody></table></div>`;
    }
    if (view.kind === "lifecycle") {
      const states = view.nodes
        .map(
          (node) => `<tr data-node="${escapeHTML(node.id)}"><td><b>${escapeHTML(node.label)}</b></td><td>${escapeHTML(node.note ?? "")}</td><td>${anchorHTML(node.anchor)}</td></tr>`,
        )
        .join("");
      const transitions = (view.transitions ?? [])
        .map(
          (transition) => `<tr><td>${escapeHTML(transition.from)}</td><td>${escapeHTML(transition.to)}</td><td>${escapeHTML(transition.label ?? "")}</td><td>${anchorHTML(transition.anchor)}</td></tr>`,
        )
        .join("");
      return `${view.enum ? `<p class="muted">States are exactly the members of ${anchorHTML(view.enum)} — adding a member in code fails CI until this diagram draws it.</p>` : ""}
        <h2>States</h2><div class="card table-card"><table><thead><tr><th>State</th><th>Meaning</th><th>Anchor</th></tr></thead><tbody>${states}</tbody></table></div>
        <h2>Transitions</h2><div class="card table-card"><table><thead><tr><th>From</th><th>To</th><th>Trigger</th><th>Anchor</th></tr></thead><tbody>${transitions}</tbody></table></div>`;
    }
    const rows = view.nodes
      .map(
        (node) => `<tr data-node="${escapeHTML(node.id)}"><td><b>${escapeHTML(node.label)}</b></td><td>${escapeHTML(node.note ?? "")}</td><td>${anchorHTML(node.anchor)}</td></tr>`,
      )
      .join("");
    const anchoredEdges = (view.edges ?? []).filter((edge) => edge.anchor);
    const edgeRows = anchoredEdges
      .map(
        (edge) => `<tr><td class="muted">${escapeHTML(edge.from)} → ${escapeHTML(edge.to)}</td><td>${escapeHTML(edge.label ?? "")}</td><td>${anchorHTML(edge.anchor)}</td></tr>`,
      )
      .join("");
    return `<h2>Elements</h2><div class="card table-card"><table><thead><tr><th>Element</th><th>Description</th><th>Anchor</th></tr></thead><tbody>${rows}</tbody></table></div>
      ${anchoredEdges.length ? `<h2>Anchored flows</h2><div class="card table-card"><table><thead><tr><th>From → To</th><th>Label</th><th>Anchor</th></tr></thead><tbody>${edgeRows}</tbody></table></div>` : ""}`;
  }

  function renderView(id, params) {
    const view = viewsById.get(id);
    if (!view) return notFound(`No view “${id}”.`);
    const notes = (view.notes ?? [])
      .map((note) => `<h2>${escapeHTML(note.title)}</h2><div class="notes">${paragraphs(note.body)}</div>`)
      .join("");
    const related = (view.related ?? [])
      .map((other) => viewsById.get(other))
      .filter(Boolean)
      .map((other) => `<a class="card tile" href="#/view/${other.id}">${kindChip(other.kind)}<h4>${escapeHTML(other.title)}</h4><p>${escapeHTML(other.summary)}</p></a>`)
      .join("");
    const sourceFile = view.derived ? null : view.source;
    const html = `<div class="page">
      <div class="eyebrow">${kindChip(view.kind)}
        ${view.derived ? '<span class="chip derived">derived from source</span>' : ""}
        ${staleByView.has(view.id) ? `<span class="chip stale">${staleByView.get(view.id)} anchor(s) to re-verify</span>` : ""}
        <span>${sourceFile ? sourceLink(sourceFile, 0, sourceFile) : escapeHTML(view.source)}</span>
      </div>
      <h1>${escapeHTML(view.title)}</h1>
      <p class="lede">${escapeHTML(view.summary)}</p>
      ${diagramCard(view.mermaid)}
      ${elementTable(view)}
      ${notes}
      ${related ? `<h2>Related views</h2><div class="grid">${related}</div>` : ""}
    </div>`;
    return { html, after: () => mountDiagrams(params.get("node")) };
  }

  function paragraphs(text) {
    return String(text ?? "")
      .trim()
      .split(/\n\s*\n/)
      .map((paragraph) => {
        const lines = paragraph.split("\n");
        if (lines.every((line) => line.trim().startsWith("- "))) {
          return `<ul>${lines.map((line) => `<li>${inlineMarkdown(line.trim().slice(2))}</li>`).join("")}</ul>`;
        }
        return `<p>${inlineMarkdown(paragraph)}</p>`;
      })
      .join("");
  }

  function inlineMarkdown(text) {
    return escapeHTML(text)
      .replace(/`([^`]+)`/g, "<code>$1</code>")
      .replace(/\*\*([^*]+)\*\*/g, "<b>$1</b>");
  }

  // ---------------------------------------------------------------- modules

  function renderModule(id, params) {
    const module = modulesById.get(id);
    if (!module) return notFound(`No module “${id}”.`);
    const group = groupsById.get(module.group);
    const viewsUsing = (A.usage[`mod:${module.id}`] ?? []).map((viewId) => viewsById.get(viewId)).filter(Boolean);
    const symbols = A.symbols.filter((symbol) => symbol.module === module.id);
    const focus = params.get("sym");
    const neighbourhood = neighbourhoodMermaid(module);
    const html = `<div class="page">
      <div class="eyebrow">
        <span class="chip">${SIDE_TITLES[module.side]}</span>
        ${group ? `<a class="chip" href="#/view/group-${group.id}">${escapeHTML(group.title)}</a>` : ""}
        <span class="chip">${module.kind}</span>
        <span>${module.files} files · ${module.lines.toLocaleString()} lines</span>
        ${module.docFile ? sourceLink(module.docFile, 0, module.docFile) : ""}
      </div>
      <h1><code style="font-size:22px;background:none;padding:0">${escapeHTML(module.id)}</code></h1>
      <div class="card doc">${module.docHtml || `<p class="muted">${escapeHTML(module.synopsis || "No documentation.")}</p>`}</div>
      <h2>Neighbourhood</h2>
      ${diagramCard(neighbourhood, "compact")}
      <div class="columns" style="margin-top:14px">
        <div class="card list-card"><h4>Imports (${module.imports.length})</h4><ul>${module.imports.map((target) => `<li>${moduleLink(target)}</li>`).join("") || '<li class="muted">nothing internal</li>'}</ul></div>
        <div class="card list-card"><h4>Imported by (${module.importedBy.length})</h4><ul>${module.importedBy.map((source) => `<li>${moduleLink(source)}</li>`).join("") || '<li class="muted">no internal importers</li>'}</ul></div>
        <div class="card list-card"><h4>Appears in views (${viewsUsing.length})</h4><ul>${viewsUsing.map((view) => `<li><a href="#/view/${view.id}">${escapeHTML(view.title)}</a></li>`).join("") || '<li class="muted">not referenced by an authored view</li>'}</ul></div>
      </div>
      <h2>Exported symbols (${symbols.length})</h2>
      <input class="symbol-filter" id="symbol-filter" placeholder="Filter symbols" value="${escapeHTML(focus ?? "")}" />
      <div class="card table-card"><table><thead><tr><th>Symbol</th><th>Kind</th><th>Source</th></tr></thead><tbody id="symbol-rows"></tbody></table></div>
    </div>`;
    const after = () => {
      mountDiagrams();
      const input = $("#symbol-filter");
      const rows = $("#symbol-rows");
      const draw = () => {
        const term = input.value.trim().toLowerCase();
        const shown = symbols.filter((symbol) => !term || symbol.name.toLowerCase().includes(term)).slice(0, 400);
        rows.innerHTML =
          shown
            .map(
              (symbol) => `<tr class="${symbol.name === focus ? "highlight" : ""}"><td><code>${escapeHTML(symbol.name)}</code></td><td class="muted">${symbol.kind}</td><td>${sourceLink(symbol.file, symbol.line)}</td></tr>`,
            )
            .join("") || '<tr><td colspan="3" class="muted">No matching symbols.</td></tr>';
      };
      input.addEventListener("input", draw);
      draw();
    };
    return { html, after };
  }

  function mermaidId(id) {
    return `n_${id.replace(/[^A-Za-z0-9_]/g, "_")}`;
  }

  function mermaidLabel(text) {
    return String(text).replace(/"/g, "#quot;");
  }

  function neighbourhoodMermaid(module) {
    const lines = ["flowchart LR"];
    const add = (id, cls) => {
      const other = modulesById.get(id);
      lines.push(`  ${mermaidId(id)}["${mermaidLabel(other ? shortModule(other) : id)}"]`);
      lines.push(`  click ${mermaidId(id)} href "#/module/${id}"`);
      if (cls) lines.push(`  class ${mermaidId(id)} ${cls}`);
    };
    lines.push("  classDef focus stroke-width:3px");
    add(module.id, "focus");
    for (const source of module.importedBy) {
      add(source);
      lines.push(`  ${mermaidId(source)} --> ${mermaidId(module.id)}`);
    }
    for (const target of module.imports) {
      add(target);
      lines.push(`  ${mermaidId(module.id)} --> ${mermaidId(target)}`);
    }
    return lines.join("\n");
  }

  function notFound(message) {
    return `<div class="page"><h1>Not found</h1><p class="lede">${escapeHTML(message)}</p><p><a href="#/">Back to the overview</a></p></div>`;
  }

  // ---------------------------------------------------------------- diagram

  function currentTheme() {
    const forced = document.documentElement.dataset.theme;
    if (forced) return forced;
    return matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
  }

  function initMermaid() {
    mermaid.initialize({
      startOnLoad: false,
      securityLevel: "loose",
      theme: currentTheme() === "dark" ? "dark" : "default",
      maxTextSize: 500000,
      maxEdges: 5000,
      flowchart: { useMaxWidth: false, htmlLabels: true, curve: "basis", nodeSpacing: 36, rankSpacing: 56 },
      sequence: { useMaxWidth: false, showSequenceNumbers: true, wrap: true, width: 180 },
      state: { useMaxWidth: false },
      themeVariables: { fontFamily: getComputedStyle(document.body).fontFamily, fontSize: "14px" },
    });
  }

  let diagramCounter = 0;
  const diagramSources = new Map();
  function diagramCard(source, variant) {
    diagramCounter += 1;
    const id = `diagram-${diagramCounter}`;
    diagramSources.set(id, source);
    const height = variant === "compact" ? ' style="height:min(46vh,460px)"' : "";
    return `<div class="card diagram-card" data-diagram="${id}">
      <div class="diagram-toolbar">
        <button data-act="in" title="Zoom in">+</button>
        <button data-act="out" title="Zoom out">−</button>
        <button data-act="fit" title="Fit to view">Fit</button>
        <button data-act="full" title="Toggle fullscreen">⤢</button>
        <button data-act="copy" title="Copy Mermaid source">Copy</button>
      </div>
      <div class="diagram-viewport"${height}><div class="diagram-stage"></div></div>
      <div class="diagram-hint">Drag to pan · ⌘/Ctrl + scroll or pinch to zoom · click a node to navigate</div>
    </div>`;
  }

  async function mountDiagrams(highlightNode) {
    const cards = [...content.querySelectorAll(".diagram-card")];
    for (const card of cards) {
      const source = diagramSources.get(card.dataset.diagram) ?? "";
      const stage = card.querySelector(".diagram-stage");
      const viewport = card.querySelector(".diagram-viewport");
      try {
        const { svg, bindFunctions } = await mermaid.render(`${card.dataset.diagram}-svg`, source);
        stage.innerHTML = svg;
        bindFunctions?.(stage);
      } catch (error) {
        stage.innerHTML = `<div class="diagram-error">${escapeHTML(error?.message ?? error)}</div>`;
        continue;
      }
      if (highlightNode) highlight(stage, highlightNode);
      attachPanZoom(card, viewport, stage, source);
    }
  }

  function highlight(stage, nodeId) {
    const target = mermaidId(nodeId);
    for (const element of stage.querySelectorAll("g.node, g.stateGroup, g[id]")) {
      if (element.id && element.id.includes(`-${target}-`)) element.classList.add("node-highlight");
      if (element.id && element.id.endsWith(target)) element.classList.add("node-highlight");
    }
    const row = content.querySelector(`tr[data-node="${CSS.escape(nodeId)}"]`);
    if (row) {
      row.classList.add("highlight");
      row.scrollIntoView({ block: "center" });
    }
  }

  function attachPanZoom(card, viewport, stage, source) {
    let scale = 1;
    let x = 0;
    let y = 0;
    const apply = () => {
      stage.style.transform = `translate(${x}px, ${y}px) scale(${scale})`;
    };
    const fit = () => {
      const svg = stage.querySelector("svg");
      if (!svg) return;
      scale = 1;
      x = 0;
      y = 0;
      apply();
      const box = stage.getBoundingClientRect();
      const room = viewport.getBoundingClientRect();
      scale = Math.min(1.25, room.width / box.width, room.height / box.height);
      x = Math.max(0, (room.width - box.width * scale) / 2);
      y = Math.max(0, (room.height - box.height * scale) / 2);
      apply();
    };
    const zoomAt = (factor, clientX, clientY) => {
      const room = viewport.getBoundingClientRect();
      const px = clientX - room.left;
      const py = clientY - room.top;
      const next = Math.min(4, Math.max(0.1, scale * factor));
      x = px - ((px - x) * next) / scale;
      y = py - ((py - y) * next) / scale;
      scale = next;
      apply();
    };
    viewport.addEventListener(
      "wheel",
      (event) => {
        if (!event.ctrlKey && !event.metaKey) return;
        event.preventDefault();
        zoomAt(Math.exp(-event.deltaY * 0.01), event.clientX, event.clientY);
      },
      { passive: false },
    );
    let drag = null;
    viewport.addEventListener("pointerdown", (event) => {
      if (event.button !== 0 || event.target.closest("a")) return;
      drag = { startX: event.clientX - x, startY: event.clientY - y, moved: false };
      viewport.setPointerCapture(event.pointerId);
      viewport.classList.add("dragging");
    });
    viewport.addEventListener("pointermove", (event) => {
      if (!drag) return;
      x = event.clientX - drag.startX;
      y = event.clientY - drag.startY;
      drag.moved = true;
      apply();
    });
    const end = () => {
      drag = null;
      viewport.classList.remove("dragging");
    };
    viewport.addEventListener("pointerup", end);
    viewport.addEventListener("pointercancel", end);
    card.querySelector(".diagram-toolbar").addEventListener("click", (event) => {
      const action = event.target.closest("button")?.dataset.act;
      const room = viewport.getBoundingClientRect();
      const cx = room.left + room.width / 2;
      const cy = room.top + room.height / 2;
      if (action === "in") zoomAt(1.25, cx, cy);
      if (action === "out") zoomAt(0.8, cx, cy);
      if (action === "fit") fit();
      if (action === "full") {
        card.classList.toggle("fullscreen");
        requestAnimationFrame(fit);
      }
      if (action === "copy") navigator.clipboard?.writeText(source);
    });
    requestAnimationFrame(fit);
  }

  // ----------------------------------------------------------------- search

  const searchIndex = [
    ...A.views.map((view) => ({
      kind: KIND_TITLES[view.category] ?? view.category,
      title: view.title,
      detail: view.id,
      href: `#/view/${view.id}`,
      text: `${view.title} ${view.id} ${view.summary}`.toLowerCase(),
      weight: 3,
    })),
    ...A.modules.map((module) => ({
      kind: "Module",
      title: shortModule(module),
      detail: module.id,
      href: `#/module/${module.id}`,
      text: `${module.id} ${module.synopsis}`.toLowerCase(),
      weight: 2,
    })),
    ...A.symbols.map((symbol) => ({
      kind: symbol.key.startsWith("go:") ? "Go" : "TS",
      title: symbol.name,
      detail: symbol.module,
      href: `#/module/${symbol.module}?sym=${encodeURIComponent(symbol.name)}`,
      text: symbol.name.toLowerCase(),
      weight: 1,
    })),
  ];

  function search(term) {
    const query = term.trim().toLowerCase();
    if (!query) return [];
    const scored = [];
    for (const item of searchIndex) {
      const index = item.text.indexOf(query);
      if (index === -1) continue;
      const exact = item.title.toLowerCase() === query ? 10 : 0;
      const prefix = item.title.toLowerCase().startsWith(query) ? 4 : 0;
      scored.push({ item, score: exact + prefix + item.weight - index / 200 });
    }
    return scored
      .sort((a, b) => b.score - a.score)
      .slice(0, 30)
      .map((entry) => entry.item);
  }

  function setupSearch() {
    const input = $("#search");
    const results = $("#search-results");
    let active = 0;
    let items = [];
    const draw = () => {
      items = search(input.value);
      active = 0;
      results.innerHTML = items
        .map(
          (item, index) => `<a href="${item.href}" class="${index === active ? "active" : ""}"><span class="kind">${escapeHTML(item.kind)}</span><span>${escapeHTML(item.title)}</span><span class="detail">${escapeHTML(item.detail)}</span></a>`,
        )
        .join("");
      results.classList.toggle("open", items.length > 0);
    };
    input.addEventListener("input", draw);
    input.addEventListener("keydown", (event) => {
      const links = [...results.querySelectorAll("a")];
      if (event.key === "ArrowDown" || event.key === "ArrowUp") {
        event.preventDefault();
        active = (active + (event.key === "ArrowDown" ? 1 : -1) + links.length) % Math.max(links.length, 1);
        links.forEach((link, index) => link.classList.toggle("active", index === active));
      } else if (event.key === "Enter" && items[active]) {
        location.hash = items[active].href.slice(1);
        results.classList.remove("open");
        input.blur();
      } else if (event.key === "Escape") {
        results.classList.remove("open");
        input.blur();
      }
    });
    results.addEventListener("click", () => results.classList.remove("open"));
    document.addEventListener("click", (event) => {
      if (!event.target.closest(".search")) results.classList.remove("open");
    });
    document.addEventListener("keydown", (event) => {
      if (event.key === "/" && document.activeElement !== input && !event.target.closest("input, textarea")) {
        event.preventDefault();
        input.focus();
        input.select();
      }
    });
  }

  // ----------------------------------------------------------------- router

  function route() {
    const { parts, params } = parseRoute();
    let result;
    if (parts[0] === "view") result = renderView(parts.slice(1).join("/"), params);
    else if (parts[0] === "module") result = renderModule(parts.slice(1).join("/"), params);
    else if (parts[0] === "health") result = renderHealth();
    else if (parts[0] === "source") {
      const url = sourceURL(parts.slice(1).join("/"), 0);
      history.back();
      if (url) window.open(url, "_self");
      return;
    } else result = renderOverview();
    const { html, after } = typeof result === "string" ? { html: result } : result;
    content.innerHTML = html;
    renderSidebar();
    sidebar.classList.remove("open");
    if (!params.get("node") && !params.get("sym")) window.scrollTo(0, 0);
    after?.();
  }

  function setup() {
    initMermaid();
    renderHealthPill();
    setupSearch();
    const select = $("#open-in");
    select.value = openIn;
    select.addEventListener("change", () => {
      openIn = select.value;
      store.set("atlas-open-in", openIn);
      route();
    });
    $("#theme-toggle").addEventListener("click", () => {
      const next = currentTheme() === "dark" ? "light" : "dark";
      document.documentElement.dataset.theme = next;
      store.set("atlas-theme", next);
      initMermaid();
      route();
    });
    $("#nav-toggle").addEventListener("click", () => sidebar.classList.toggle("open"));
    addEventListener("hashchange", route);
    route();
  }

  setup();
})();
