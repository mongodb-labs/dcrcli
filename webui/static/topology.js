// Topology diagram: routers on top, shards in the middle, config servers at the
// bottom, this machine above them all. Replication flows primary -> secondary,
// and during collection a stream runs from the active node back to dcrcli.
(function () {
  const NS = "http://www.w3.org/2000/svg";
  const R = 24;
  const SPACING = 132;
  const PAD = 26;
  const HEADER = 40;
  const ARC = 50;
  const LABEL = 58;
  const ROW_GAP = 82;
  const GROUP_GAP = 30;
  const MARGIN = 36;
  const GROUP_H = HEADER + ARC + 2 * R + LABEL;
  const CLIENT_W = 230;
  const CLIENT_H = 46;
  const ROLES = ["PRIMARY", "SECONDARY", "MONGOS", "ARBITER", "UNKNOWN"];
  const GLYPH = { PRIMARY: "P", SECONDARY: "S", MONGOS: "R", ARBITER: "A", UNKNOWN: "?" };
  const BADGE = { done: "✓", partial: "~", failed: "!", pending: "", active: "" };
  let instance = 0;

  function el(tag, attrs, parent) {
    const node = document.createElementNS(NS, tag);
    for (const [k, v] of Object.entries(attrs || {})) node.setAttribute(k, v);
    if (parent) parent.appendChild(node);
    return node;
  }

  function keyOf(n) {
    return n.hostname + ":" + n.port;
  }

  function roleOf(n) {
    const r = String(n.role || "UNKNOWN").toUpperCase();
    return ROLES.includes(r) ? r : "UNKNOWN";
  }

  function byHost(a, b) {
    return a.hostname === b.hostname ? a.port - b.port : a.hostname < b.hostname ? -1 : 1;
  }

  // Primary in the middle so replication arcs fan out on both sides.
  function centerPrimary(members) {
    const sorted = members.slice().sort(byHost);
    const primary = sorted.find((m) => roleOf(m) === "PRIMARY");
    if (!primary) return sorted;
    const others = sorted.filter((m) => m !== primary);
    others.splice(Math.floor(others.length / 2), 0, primary);
    return others;
  }

  function buildGroups(nodes) {
    const mongos = [];
    const config = [];
    const plain = [];
    const shards = new Map();
    for (const n of nodes) {
      const shard = String(n.shard_role || "").trim();
      if (roleOf(n) === "MONGOS") mongos.push(n);
      else if (shard.toLowerCase() === "config") config.push(n);
      else if (shard) {
        if (!shards.has(shard)) shards.set(shard, []);
        shards.get(shard).push(n);
      } else plain.push(n);
    }
    const sharded = mongos.length > 0 || config.length > 0 || shards.size > 0;
    const groups = [];
    if (mongos.length) {
      groups.push({ kind: "mongos", row: 0, title: "Query routers", subtitle: "mongos", members: mongos.sort(byHost) });
    }
    for (const name of [...shards.keys()].sort()) {
      const members = shards.get(name);
      groups.push({ kind: "shard", row: 1, title: name, subtitle: "shard · " + members.length + " member" + (members.length === 1 ? "" : "s"), members: centerPrimary(members) });
    }
    if (plain.length) {
      const standalone = !sharded && plain.length === 1 && roleOf(plain[0]) !== "SECONDARY";
      groups.push({
        kind: standalone ? "standalone" : "replset",
        row: 1,
        title: standalone ? "Standalone" : sharded ? "Other members" : "Replica set",
        subtitle: standalone ? "single mongod" : plain.length + " member" + (plain.length === 1 ? "" : "s"),
        members: centerPrimary(plain),
      });
    }
    if (config.length) {
      groups.push({ kind: "config", row: 2, title: "Config servers", subtitle: "CSRS · cluster metadata", members: centerPrimary(config) });
    }
    return { groups, sharded };
  }

  function summarize(nodes) {
    const { groups, sharded } = buildGroups(nodes);
    const count = (role) => nodes.filter((n) => roleOf(n) === role).length;
    let type = "Replica set";
    if (sharded) type = "Sharded cluster";
    else if (groups.length && groups[0].kind === "standalone") type = "Standalone";
    return {
      type,
      sharded,
      members: nodes.length,
      primaries: count("PRIMARY"),
      secondaries: count("SECONDARY"),
      routers: count("MONGOS"),
      shards: groups.filter((g) => g.kind === "shard").length,
      arbiters: count("ARBITER"),
    };
  }

  function layout(groups) {
    const widthOf = (g) => Math.max(1, g.members.length) * SPACING + PAD * 2;
    const maxW = Math.max(980, ...groups.map(widthOf));
    const rows = [];
    for (const rowIdx of [0, 1, 2]) {
      let cur = [];
      let w = 0;
      for (const g of groups.filter((x) => x.row === rowIdx)) {
        const gw = widthOf(g);
        if (cur.length && w + GROUP_GAP + gw > maxW) {
          rows.push({ groups: cur, width: w });
          cur = [];
          w = 0;
        }
        w += (cur.length ? GROUP_GAP : 0) + gw;
        cur.push(g);
      }
      if (cur.length) rows.push({ groups: cur, width: w });
    }
    const W = Math.max(CLIENT_W, ...rows.map((r) => r.width)) + MARGIN * 2;
    let y = MARGIN;
    const client = { x: W / 2, y, w: CLIENT_W, h: CLIENT_H };
    y += CLIENT_H + ROW_GAP * 0.85;
    for (const row of rows) {
      let x = (W - row.width) / 2;
      for (const g of row.groups) {
        g.x = x;
        g.y = y;
        g.w = widthOf(g);
        g.h = GROUP_H;
        g.members.forEach((m, i) => {
          m._cx = x + PAD + SPACING / 2 + i * SPACING;
          m._cy = y + HEADER + ARC + R;
        });
        x += g.w + GROUP_GAP;
      }
      y += GROUP_H + ROW_GAP;
    }
    return { W, H: y - ROW_GAP + MARGIN, client };
  }

  function curve(x1, y1, x2, y2) {
    const dy = Math.max(30, Math.abs(y2 - y1) * 0.5);
    return `M ${x1} ${y1} C ${x1} ${y1 + dy}, ${x2} ${y2 - dy}, ${x2} ${y2}`;
  }

  // Same S-curve as curve(), drawn bottom-up so the flow moves toward (x2, y2).
  function upward(x1, y1, x2, y2) {
    const dy = Math.max(30, Math.abs(y1 - y2) * 0.5);
    return `M ${x1} ${y1} C ${x1} ${y1 - dy}, ${x2} ${y2 + dy}, ${x2} ${y2}`;
  }

  function edge(parent, d, cls, delay) {
    const g = el("g", { class: "tedge " + cls, style: `--d:${delay}ms` }, parent);
    el("path", { d, class: "edge-base", pathLength: "1" }, g);
    el("path", { d, class: "edge-flow" }, g);
    return g;
  }

  function findSeed(nodes, seed) {
    if (!seed || !seed.host) return null;
    const host = seed.host.toLowerCase();
    const port = Number(seed.port || 27017);
    const local = ["localhost", "127.0.0.1", "::1"].includes(host);
    return (
      nodes.find((n) => n.hostname.toLowerCase() === host && n.port === port) ||
      (local ? nodes.find((n) => n.port === port) : null) ||
      nodes.find((n) => n.hostname.toLowerCase() === host) ||
      null
    );
  }

  function truncate(s, n) {
    return s.length > n ? s.slice(0, n - 1) + "…" : s;
  }

  function shortHost(host) {
    if (/^[\d.]+$/.test(host) || host.includes(":")) return host;
    return host.split(".")[0];
  }

  function render(container, nodes, opts) {
    opts = opts || {};
    const signature = JSON.stringify(nodes.map((n) => [keyOf(n), n.role, n.shard_role])) + "|" + JSON.stringify(opts.seed || {});
    if (container._topo && container._topo.signature === signature) return;
    container.replaceChildren();
    instance += 1;
    const id = "t" + instance;

    const work = nodes.map((n) => Object.assign({}, n));
    const { groups } = buildGroups(work);
    const { W, H, client } = layout(groups);

    const svg = el("svg", { viewBox: `0 0 ${W} ${H}`, class: "topo-svg", role: "img", "aria-label": "Cluster topology diagram" });
    const defs = el("defs", {}, svg);
    const grads = {
      PRIMARY: ["#FFE07A", "#F2A900"],
      SECONDARY: ["#4BE39A", "#00A35C"],
      MONGOS: ["#6FB0FF", "#016BF8"],
      ARBITER: ["#D8DEDD", "#889397"],
      UNKNOWN: ["#B8C4CA", "#5C6C75"],
    };
    for (const [role, [a, b]] of Object.entries(grads)) {
      const lg = el("radialGradient", { id: `${id}-g-${role}`, cx: "35%", cy: "30%", r: "75%" }, defs);
      el("stop", { offset: "0%", "stop-color": a }, lg);
      el("stop", { offset: "100%", "stop-color": b }, lg);
    }
    const glow = el("filter", { id: `${id}-glow`, x: "-50%", y: "-50%", width: "200%", height: "200%" }, defs);
    el("feGaussianBlur", { stdDeviation: "6", result: "b" }, glow);
    const merge = el("feMerge", {}, glow);
    el("feMergeNode", { in: "b" }, merge);
    el("feMergeNode", { in: "SourceGraphic" }, merge);
    const dots = el("pattern", { id: `${id}-dots`, width: "22", height: "22", patternUnits: "userSpaceOnUse" }, defs);
    el("circle", { cx: "1.5", cy: "1.5", r: "1.2", class: "dot" }, dots);
    el("rect", { x: 0, y: 0, width: W, height: H, fill: `url(#${id}-dots)`, class: "topo-bg" }, svg);

    const routeLayer = el("g", { class: "layer-routes" }, svg);
    const groupLayer = el("g", { class: "layer-groups" }, svg);
    const replLayer = el("g", { class: "layer-repl" }, svg);
    const nodeLayer = el("g", { class: "layer-nodes" }, svg);
    const topLayer = el("g", { class: "layer-top" }, svg);

    const routers = groups.find((g) => g.kind === "mongos");
    const targets = groups.filter((g) => g.kind !== "mongos");
    if (routers) {
      let d = 200;
      const many = routers.members.length > 3;
      for (const t of targets) {
        const tx = t.x + t.w / 2;
        if (many) {
          edge(routeLayer, curve(routers.x + routers.w / 2, routers.y + routers.h, tx, t.y), t.kind === "config" ? "route meta" : "route", (d += 60));
          continue;
        }
        for (const m of routers.members) {
          edge(routeLayer, curve(m._cx, routers.y + routers.h - 6, tx, t.y), t.kind === "config" ? "route meta" : "route", (d += 40));
        }
      }
    }

    const groupEls = [];
    groups.forEach((g, gi) => {
      const ge = el("g", { class: "tgroup kind-" + g.kind, style: `--i:${gi}` }, groupLayer);
      el("rect", { x: g.x, y: g.y, width: g.w, height: g.h, rx: 18, class: "group-box" }, ge);
      el("rect", { x: g.x, y: g.y, width: g.w, height: 4, rx: 2, class: "group-accent" }, ge);
      const title = el("text", { x: g.x + 18, y: g.y + 26, class: "group-title" }, ge);
      title.textContent = g.title;
      const sub = el("text", { x: g.x + g.w - 18, y: g.y + 26, class: "group-sub", "text-anchor": "end" }, ge);
      sub.textContent = g.subtitle;
      groupEls.push({ el: ge, keys: g.members.map(keyOf) });
    });

    groups.forEach((g, gi) => {
      if (g.kind === "mongos") return;
      const primary = g.members.find((m) => roleOf(m) === "PRIMARY");
      if (!primary) return;
      const pi = g.members.indexOf(primary);
      g.members.forEach((m, i) => {
        if (m === primary) return;
        const lift = 20 + Math.min(30, Math.abs(i - pi) * 10);
        const y0 = primary._cy - R - 4;
        const d = `M ${primary._cx} ${y0} Q ${(primary._cx + m._cx) / 2} ${y0 - lift * 1.6}, ${m._cx} ${y0}`;
        edge(replLayer, d, roleOf(m) === "ARBITER" ? "heartbeat" : "repl", 500 + gi * 120 + i * 80);
      });
    });

    const nodeEls = new Map();
    const info = new Map();
    let ni = 0;
    for (const g of groups) {
      for (const m of g.members) {
        const role = roleOf(m);
        const key = keyOf(m);
        const ne = el("g", {
          class: "tnode role-" + role,
          "data-key": key,
          tabindex: "0",
          style: `--i:${ni++}`,
          transform: `translate(${m._cx} ${m._cy})`,
        }, nodeLayer);
        const body = el("g", { class: "node-body" }, ne);
        el("circle", { r: R + 12, class: "halo", filter: `url(#${id}-glow)` }, body);
        el("circle", { r: R + 6, class: "ring" }, body);
        if (role === "PRIMARY") el("circle", { r: R + 3, class: "beat" }, body);
        el("circle", { r: R, class: "core", fill: `url(#${id}-g-${role})` }, body);
        el("circle", { r: R - 7, class: "core-shine" }, body);
        const glyph = el("text", { class: "glyph", "text-anchor": "middle", dy: "0.36em" }, body);
        glyph.textContent = GLYPH[role];
        const badge = el("g", { class: "badge", transform: `translate(${R * 0.78} ${-R * 0.78})` }, ne);
        el("circle", { r: 10 }, badge);
        el("text", { "text-anchor": "middle", dy: "0.36em" }, badge);
        const name = el("text", { y: R + 22, class: "node-name", "text-anchor": "middle" }, ne);
        name.textContent = truncate(shortHost(m.hostname), 16);
        const meta = el("text", { y: R + 40, class: "node-meta", "text-anchor": "middle" }, ne);
        meta.textContent = ":" + m.port + " · " + role.toLowerCase();
        nodeEls.set(key, ne);
        info.set(key, { node: m, group: g });
      }
    }

    const seedNode = findSeed(work, opts.seed);
    const clientG = el("g", { class: "tclient" }, topLayer);
    el("rect", { x: client.x - client.w / 2, y: client.y, width: client.w, height: client.h, rx: 14, class: "client-box" }, clientG);
    const ct = el("text", { x: client.x, y: client.y + 20, class: "client-title", "text-anchor": "middle" }, clientG);
    ct.textContent = "Data Collector · this machine";
    const cs = el("text", { x: client.x, y: client.y + 36, class: "client-sub", "text-anchor": "middle" }, clientG);
    cs.textContent = "read-only collector";
    if (seedNode) {
      edge(topLayer, curve(client.x, client.y + client.h, seedNode._cx, seedNode._cy - R - 8), "seed", 150);
      const tag = el("g", { class: "seed-tag", transform: `translate(${seedNode._cx - R - 8} ${seedNode._cy - R - 8})` }, topLayer);
      el("rect", { x: -36, y: -11, width: 40, height: 18, rx: 9 }, tag);
      const tt = el("text", { x: -16, y: 2, "text-anchor": "middle" }, tag);
      tt.textContent = "seed";
    }
    const beamLayer = el("g", { class: "layer-beam" }, svg);

    const wrap = document.createElement("div");
    wrap.className = "topo-wrap";
    wrap.appendChild(svg);
    const tip = document.createElement("div");
    tip.className = "topo-tip";
    tip.hidden = true;
    wrap.appendChild(tip);
    container.appendChild(wrap);

    const s = { signature, svg, nodeEls, groupEls, info, client, beamLayer, tip, targets: null, states: null };
    container._topo = s;

    function showTip(key, evt) {
      const data = info.get(key);
      if (!data) return;
      tip.replaceChildren();
      const h = document.createElement("strong");
      h.textContent = key;
      const role = document.createElement("span");
      role.className = "role " + roleOf(data.node);
      role.textContent = roleOf(data.node);
      const group = document.createElement("div");
      group.className = "tip-group";
      group.textContent = data.group.title + (data.group.kind === "shard" ? " · shard" : "");
      tip.append(h, role, group);
      if (s.targets) {
        const sel = document.createElement("div");
        sel.className = s.targets.has(key) ? "tip-yes" : "tip-no";
        sel.textContent = s.targets.has(key) ? "Selected for collection" : "Not in this scope";
        tip.append(sel);
      }
      const st = s.states && s.states.get(key);
      if (st && st.tasks) {
        const list = document.createElement("div");
        list.className = "tip-tasks";
        for (const t of st.tasks) {
          if (t.state === "omitted") continue;
          const row = document.createElement("div");
          row.className = "task " + t.state;
          const mark = { ok: "✓ ", failed: "! ", skipped: "− ", running: "› ", pending: "· " }[t.state] || "";
          row.textContent = mark + t.name + (t.note ? " — " + t.note : "");
          list.append(row);
        }
        tip.append(list);
      }
      tip.hidden = false;
      moveTip(evt);
    }
    function moveTip(evt) {
      const box = wrap.getBoundingClientRect();
      let x = evt.clientX - box.left + 16;
      let y = evt.clientY - box.top + 16;
      if (x + 280 > box.width) x = Math.max(8, x - 300);
      tip.style.left = x + "px";
      tip.style.top = y + "px";
    }
    function hideTip() {
      tip.hidden = true;
    }
    function nodeFrom(target) {
      return target && target.closest ? target.closest(".tnode") : null;
    }
    svg.addEventListener("mouseover", (e) => {
      const node = nodeFrom(e.target);
      if (!node) return;
      showTip(node.getAttribute("data-key"), e);
    });
    svg.addEventListener("mousemove", (e) => {
      if (tip.hidden) return;
      if (!nodeFrom(e.target)) hideTip();
      else moveTip(e);
    });
    svg.addEventListener("mouseout", (e) => {
      const node = nodeFrom(e.target);
      if (!node) return;
      if (nodeFrom(e.relatedTarget) === node) return;
      hideTip();
    });
    wrap.addEventListener("mouseleave", hideTip);
    for (const [key, ne] of nodeEls) {
      ne.addEventListener("focus", () => {
        const r = ne.getBoundingClientRect();
        showTip(key, { clientX: r.right, clientY: r.top });
      });
      ne.addEventListener("blur", hideTip);
    }
  }

  function update(container, view) {
    const s = container._topo;
    if (!s) return;
    view = view || {};
    s.targets = view.targets || null;
    s.states = view.states || null;
    s.svg.classList.toggle("has-targets", Boolean(s.targets));
    s.svg.classList.toggle("is-live", Boolean(s.states));
    let active = null;
    for (const [key, ne] of s.nodeEls) {
      const isTarget = Boolean(s.targets && s.targets.has(key));
      ne.classList.toggle("is-target", isTarget);
      ne.classList.toggle("is-dim", Boolean(s.targets) && !isTarget);
      for (const c of [...ne.classList]) if (c.startsWith("st-")) ne.classList.remove(c);
      const st = s.states && s.states.get(key);
      const label = ne.querySelector(".badge text");
      label.textContent = "";
      if (st) {
        ne.classList.add("st-" + st.state);
        label.textContent = BADGE[st.state] || "";
        if (st.state === "active") active = key;
      }
    }
    for (const g of s.groupEls) {
      g.el.classList.toggle("is-dim", Boolean(s.targets) && !g.keys.some((k) => s.targets.has(k)));
    }
    if (s.activeBeam !== active) {
      s.beamLayer.replaceChildren();
      s.activeBeam = active;
      const data = active && s.info.get(active);
      if (data) {
        const m = data.node;
        edge(s.beamLayer, upward(m._cx, m._cy - R - 10, s.client.x, s.client.y + s.client.h), "beam", 0);
      }
    }
  }

  window.Topology = { render, update, summarize, keyOf, roleOf };
})();
