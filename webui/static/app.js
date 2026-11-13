const state = {
  step: "connect",
  discovered: false,
  running: false,
  finished: false,
  nodes: [],
  modes: [],
  scope: "one-secondary",
  outputDir: "",
  hasPassword: false,
  savedUsername: "",
  seed: null,
  snapshot: null,
  events: null,
  view: "diagram",
  prompts: [],
  activePrompt: null,
  promptReturnFocus: null,
};

const steps = ["guide", "connect", "topology", "scope", "collect", "results"];
const demoKind = new URLSearchParams(location.search).get("demo");

function $(id) {
  return document.getElementById(id);
}

function h(tag, cls, text) {
  const node = document.createElement(tag);
  if (cls) node.className = cls;
  if (text !== undefined && text !== null) node.textContent = text;
  return node;
}

const ICONS = {
  error: '<svg viewBox="0 0 24 24" width="24" height="24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round"><circle cx="12" cy="12" r="9"/><path d="M12 7.5v5.5M12 16.5v.01"/></svg>',
  key: '<svg viewBox="0 0 24 24" width="24" height="24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><circle cx="8" cy="15" r="4"/><path d="M11 12l8-8M16 7l3 3M14 9l2 2"/></svg>',
  plug: '<svg viewBox="0 0 24 24" width="24" height="24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M9 7V3M15 7V3M7 7h10v4a5 5 0 0 1-10 0V7zM12 16v5"/></svg>',
  globe: '<svg viewBox="0 0 24 24" width="24" height="24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round"><circle cx="12" cy="12" r="9"/><path d="M3 12h18M12 3a14 14 0 0 1 0 18M12 3a14 14 0 0 0 0 18"/></svg>',
  clock: '<svg viewBox="0 0 24 24" width="24" height="24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round"><circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/></svg>',
  lock: '<svg viewBox="0 0 24 24" width="26" height="26" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><rect x="5" y="11" width="14" height="10" rx="2"/><path d="M8 11V7a4 4 0 0 1 8 0v4"/></svg>',
  shield: '<svg viewBox="0 0 24 24" width="26" height="26" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 3l8 3v6c0 4.5-3.4 8.3-8 9-4.6-.7-8-4.5-8-9V6l8-3z"/><path d="M12 8v4M12 15.5v.01"/></svg>',
};

const PROBLEM_ICON = {
  password_missing: "key",
  auth_failed: "key",
  auth_required: "key",
  connection_refused: "plug",
  host_not_found: "globe",
  unresolved_members: "globe",
  timeout: "clock",
  tls: "lock",
};
const WARN_CODES = new Set(["password_missing", "unresolved_members"]);

// ---------- API ----------

class ApiError extends Error {
  constructor(message, problem, status) {
    super(message);
    this.problem = problem || null;
    this.status = status;
  }
}

async function api(path, body) {
  if (demoKind) return Demo.api(path, body);
  const opts = body === undefined
    ? { method: "GET" }
    : { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) };
  const res = await fetch(path, opts);
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new ApiError(data.error || res.statusText || "request failed", data.problem, res.status);
  return data;
}

// ---------- Problems ----------

function clearFieldErrors() {
  document.querySelectorAll("label.invalid").forEach((label) => {
    label.classList.remove("invalid");
    const msg = label.querySelector(".field-msg");
    if (msg) msg.remove();
  });
}

function markField(field, text) {
  const label = document.querySelector('label[data-field="' + field + '"]');
  if (!label) return null;
  label.classList.remove("invalid");
  void label.offsetWidth;
  label.classList.add("invalid");
  let msg = label.querySelector(".field-msg");
  if (!msg) {
    msg = h("span", "field-msg");
    label.append(msg);
  }
  msg.textContent = text;
  return label.querySelector("input");
}

function clearProblem() {
  $("problem-slot").replaceChildren();
  clearFieldErrors();
}

function showProblem(p, actions) {
  clearProblem();
  if (!p) return;
  if (typeof p === "string") p = { title: "Something went wrong", message: p };
  const card = h("div", "problem" + (WARN_CODES.has(p.code) ? " warn" : ""));
  card.setAttribute("role", "alert");
  const icon = h("div", "problem-icon");
  icon.innerHTML = ICONS[PROBLEM_ICON[p.code] || "error"];
  const body = h("div", "problem-body");
  const head = h("div", "problem-head");
  const close = h("button", "problem-close", "×");
  close.type = "button";
  close.setAttribute("aria-label", "Dismiss");
  close.addEventListener("click", clearProblem);
  head.append(h("h3", "", p.title || "Something went wrong"), close);
  body.append(head);
  if (p.message) body.append(h("p", "", p.message));

  if (p.hosts && p.hosts.length) {
    const hosts = h("div", "problem-hosts");
    for (const host of p.hosts) hosts.append(h("code", "", host));
    body.append(hosts);
  }
  if (p.hints && p.hints.length) {
    const hints = h("div", "problem-hints");
    hints.append(h("strong", "", "Try this"));
    const ul = h("ul");
    for (const hint of p.hints) ul.append(h("li", "", hint));
    hints.append(ul);
    body.append(hints);
  }
  if (actions && actions.length) {
    const row = h("div", "problem-actions");
    actions.forEach((a, i) => {
      const btn = h("button", "btn" + (i === 0 ? " primary" : ""), a.label);
      btn.type = "button";
      btn.addEventListener("click", a.run);
      row.append(btn);
    });
    body.append(row);
  }
  if (p.details) {
    const det = h("details");
    det.append(h("summary", "", "Technical details"));
    det.append(h("pre", "", p.details));
    const copy = h("button", "btn copy-btn", "Copy details");
    copy.type = "button";
    copy.addEventListener("click", async () => {
      try {
        await navigator.clipboard.writeText(p.details);
        copy.textContent = "Copied";
      } catch (_) {
        copy.textContent = "Copy failed";
      }
      setTimeout(() => (copy.textContent = "Copy details"), 1500);
    });
    det.append(copy);
    body.append(det);
  }
  card.append(icon, body);
  $("problem-slot").append(card);

  if (p.field) {
    const input = markField(p.field, p.title || p.message);
    if (input && state.step === "connect") input.focus({ preventScroll: true });
  }
  card.scrollIntoView({ behavior: "smooth", block: "nearest" });
}

function problemFromError(err) {
  if (err && err.problem) return err.problem;
  return { title: "Request failed", message: (err && err.message) || String(err) };
}

// ---------- Navigation ----------

function show(step) {
  state.step = step;
  for (const name of steps) {
    $("step-" + name).hidden = name !== step;
    const btn = document.querySelector('.step-btn[data-step="' + name + '"]');
    btn.setAttribute("aria-current", name === step ? "step" : "false");
    btn.disabled = !canOpen(name);
  }
  clearProblem();
  if (step === "topology") renderTopology();
  if (step === "scope") renderScopeDiagram();
  if (step === "collect") renderCollectDiagram();
  if (step === "results") renderResults();
}

function canOpen(step) {
  if (step === "guide") return true;
  if (state.running && step !== "collect") return false;
  if (step === "connect") return true;
  if (step === "topology" || step === "scope") return state.discovered;
  if (step === "collect") return Boolean(state.running || state.finished || state.snapshot);
  if (step === "results") return state.finished;
  return false;
}

function roleName(role) {
  return Topology.roleOf({ role });
}

function rolePill(role) {
  const name = roleName(role);
  return h("span", "role " + name, name);
}

// ---------- Topology ----------

function animateNumber(el, to) {
  const start = performance.now();
  const dur = 700;
  function tick(now) {
    const t = Math.min(1, (now - start) / dur);
    el.textContent = String(Math.round(to * (1 - Math.pow(1 - t, 3))));
    if (t < 1) requestAnimationFrame(tick);
  }
  requestAnimationFrame(tick);
}

function renderStats() {
  const s = Topology.summarize(state.nodes);
  const wrap = $("topo-stats");
  wrap.replaceChildren();
  const items = [
    { cls: "type", label: "Topology", text: s.type },
    { cls: "", label: "Members", n: s.members },
  ];
  if (s.sharded) items.push({ cls: "", label: "Shards", n: s.shards });
  if (s.routers) items.push({ cls: "router", label: "mongos", n: s.routers });
  items.push({ cls: "primary", label: "Primaries", n: s.primaries });
  items.push({ cls: "secondary", label: "Secondaries", n: s.secondaries });
  if (s.arbiters) items.push({ cls: "", label: "Arbiters", n: s.arbiters });
  items.forEach((item, i) => {
    const box = h("div", "stat " + item.cls);
    box.style.setProperty("--i", i);
    const b = h("b", "", item.text !== undefined ? item.text : "0");
    box.append(b, h("span", "", item.label));
    wrap.append(box);
    if (item.n !== undefined) animateNumber(b, item.n);
  });
  return s;
}

function renderTopology() {
  const rows = $("node-rows");
  rows.replaceChildren();
  for (const node of state.nodes) {
    const tr = h("tr");
    const role = h("td");
    role.append(rolePill(node.role));
    tr.append(h("td", "", node.hostname), h("td", "", String(node.port)), role, h("td", "", node.shard_role || "—"));
    rows.append(tr);
  }
  const s = renderStats();
  const n = state.nodes.length;
  $("topology-summary").textContent = s.type + " with " + n + " member" + (n === 1 ? "" : "s") + ". Hover a node for details, then choose what to collect.";
  setView(state.view);
  if (state.step === "topology") {
    Topology.render($("topo-diagram"), state.nodes, { seed: state.seed });
    Topology.update($("topo-diagram"), {});
  }
}

function setView(view) {
  state.view = view;
  $("view-diagram").setAttribute("aria-pressed", String(view === "diagram"));
  $("view-table").setAttribute("aria-pressed", String(view === "table"));
  $("topo-diagram-wrap").hidden = view !== "diagram";
  $("topo-table").hidden = view !== "table";
}

function targetKeys(mode) {
  if (!mode || !mode.targets) return new Set();
  if (mode.error && !(mode.standalone && $("allow-standalone").checked)) return new Set();
  return new Set(mode.targets.map(Topology.keyOf));
}

function renderScopeDiagram() {
  if (state.step !== "scope" || !state.nodes.length) return;
  Topology.render($("scope-diagram"), state.nodes, { seed: state.seed });
  Topology.update($("scope-diagram"), { targets: targetKeys(selectedMode()) });
}

function renderCollectDiagram() {
  if (state.step !== "collect" || !state.nodes.length) return;
  Topology.render($("collect-diagram"), state.nodes, { seed: state.seed });
  const snap = state.snapshot;
  const states = new Map();
  const targets = new Set();
  for (const n of (snap && snap.nodes) || []) {
    const key = Topology.keyOf(n);
    targets.add(key);
    states.set(key, { state: n.state || "pending", tasks: n.tasks || [] });
  }
  Topology.update($("collect-diagram"), snap ? { targets, states } : { targets: targetKeys(selectedMode()) });
}

// ---------- Scope ----------

function selectedMode() {
  return state.modes.find((mode) => mode.id === state.scope) || null;
}

function renderModes() {
  const wrap = $("mode-cards");
  wrap.replaceChildren();
  $("standalone-callout").hidden = !state.modes.some((mode) => mode.standalone);
  for (const mode of state.modes) {
    const label = h("label", "mode-card");
    const input = document.createElement("input");
    input.type = "radio";
    input.name = "collect_nodes";
    input.value = mode.id;
    input.checked = mode.id === state.scope;
    input.addEventListener("change", () => {
      state.scope = mode.id;
      renderTargets();
    });
    const count = mode.targets ? mode.targets.length : 0;
    label.append(input, h("strong", "", mode.label + " · " + count + " node" + (count === 1 ? "" : "s")), h("small", "", mode.description));
    wrap.append(label);
  }
  renderTargets();
  updateSSHField();
}

function renderTargets() {
  const box = $("target-box");
  const mode = selectedMode();
  box.replaceChildren();
  if (!mode) {
    box.textContent = "Choose a scope.";
    updateStartButton();
    return;
  }
  box.append(h("strong", "", mode.label));
  if (mode.error) {
    box.append(h("p", "", mode.standalone
      ? "No secondary is available. Confirm the standalone primary above, or choose all nodes."
      : mode.error));
  }
  if ((mode.targets || []).length) {
    const list = h("ul");
    for (const node of mode.targets) list.append(h("li", "", node.hostname + ":" + node.port + " (" + roleName(node.role) + ")"));
    box.append(list);
  }
  updateStartButton();
  renderScopeDiagram();
}

function collectDataValue() {
  const parts = [];
  if ($("art-gmd").checked) parts.push("getmongodata");
  if ($("art-ftdc").checked) parts.push("ftdc");
  if ($("art-logs").checked) parts.push("logs");
  if ($("art-commands").checked) parts.push("commands");
  if (parts.length === 4) return "all";
  return parts.join(",");
}

function needsSSH() {
  return $("art-commands").checked || $("art-ftdc").checked || $("art-logs").checked;
}

function updateSSHField() {
  $("ssh-field").hidden = !needsSSH();
  $("ssh-hint").hidden = !needsSSH();
}

function updateStartButton() {
  const mode = selectedMode();
  const data = collectDataValue();
  let ok = Boolean(mode && data && mode.targets && mode.targets.length);
  if (mode && mode.error) {
    ok = Boolean(mode.standalone && $("allow-standalone").checked && mode.targets.length && data);
  }
  $("start-collect").disabled = !ok || state.running;
}

// ---------- Collection ----------

function appendLog(ev) {
  const li = h("li", ev.level || "info", (ev.time ? ev.time.slice(11, 19) + "  " : "") + (ev.message || ""));
  const log = $("activity-log");
  log.append(li);
  while (log.children.length > 200) log.firstChild.remove();
  log.scrollTop = log.scrollHeight;
}

function collectMessage() {
  const ev = state.snapshot;
  if (state.prompts.length && state.running) {
    const p = state.prompts[0];
    return "Waiting for you to answer an SSH prompt" + (p.context ? " (" + p.context + ")" : "") + ".";
  }
  if (!ev) return state.running ? "Starting collection…" : "Waiting to start.";
  return ev.message || (ev.status === "running" ? "Collecting…" : "");
}

const STATE_LABEL = { done: "Collected", partial: "Partial", failed: "Failed", pending: "Waiting", active: "Collecting" };

function renderNodeCard(node) {
  const st = node.state || "pending";
  const card = h("article", "node-card " + st);
  const header = h("header");
  const id = h("div", "node-id");
  id.append(h("strong", "", node.hostname + ":" + node.port), rolePill(node.role));
  header.append(id, h("span", "state-pill " + st, STATE_LABEL[st] || st));
  const tasks = h("div", "tasks");
  const notes = h("div", "task-notes");
  const seenNotes = new Set();
  for (const task of node.tasks || []) {
    if (task.state === "omitted") continue;
    const mark = { ok: "✓ ", failed: "! ", skipped: "− ", running: "› ", pending: "· " }[task.state] || "";
    const pill = h("span", "task " + task.state, mark + task.name);
    if (task.note) pill.title = task.note;
    tasks.append(pill);
    if (task.state === "failed" && task.note && !seenNotes.has(task.note)) {
      seenNotes.add(task.note);
      notes.append(h("p", "task-note", task.name + ": " + task.note));
    }
  }
  card.append(header, tasks);
  if (notes.childElementCount) card.append(notes);
  return card;
}

function renderSnapshot(ev) {
  state.snapshot = ev;
  if (ev.output_dir) state.outputDir = ev.output_dir;
  $("collect-message").textContent = collectMessage();
  $("bar-fill").style.width = (ev.percent || 0) + "%";
  $("bar-label").textContent = (ev.percent || 0) + "%";
  const wrap = $("progress-nodes");
  wrap.replaceChildren();
  for (const node of ev.nodes || []) wrap.append(renderNodeCard(node));
  renderCollectDiagram();
  if (ev.status === "done" || ev.status === "error") {
    state.running = false;
    state.finished = true;
    if (state.step === "collect") show("results");
    else if (state.step === "results") renderResults();
  }
}

function resultCounts(nodes) {
  const counts = { done: 0, partial: 0, failed: 0, artifacts: 0, failedTasks: 0 };
  for (const node of nodes) {
    const st = node.state || "pending";
    if (counts[st] !== undefined) counts[st] += 1;
    for (const task of node.tasks || []) {
      if (task.state === "ok") counts.artifacts += 1;
      if (task.state === "failed") counts.failedTasks += 1;
    }
  }
  return counts;
}

function renderResultStats(nodes, failed) {
  const c = resultCounts(nodes);
  const wrap = $("results-stats");
  wrap.replaceChildren();
  const items = [{ cls: "", label: "Nodes", n: nodes.length }];
  if (c.done) items.push({ cls: "secondary", label: "Collected", n: c.done });
  if (c.partial) items.push({ cls: "primary", label: "Partial", n: c.partial });
  if (c.failed) items.push({ cls: "failed", label: "Failed", n: c.failed });
  items.push({ cls: "", label: failed ? "Tasks saved" : "Artifacts", n: c.artifacts });
  items.forEach((item, i) => {
    const box = h("div", "stat " + item.cls);
    box.style.setProperty("--i", i);
    const b = h("b", "", "0");
    box.append(b, h("span", "", item.label));
    wrap.append(box);
    animateNumber(b, item.n);
  });
  return c;
}

function renderResultsDiagram(nodes) {
  const box = $("results-diagram");
  if (!state.nodes.length) {
    box.hidden = true;
    return;
  }
  box.hidden = false;
  Topology.render(box, state.nodes, { seed: state.seed });
  const states = new Map();
  const targets = new Set();
  for (const n of nodes) {
    const key = Topology.keyOf(n);
    targets.add(key);
    states.set(key, { state: n.state || "pending", tasks: n.tasks || [] });
  }
  Topology.update(box, { targets, states });
}

function renderResults() {
  const snap = state.snapshot;
  const nodes = (snap && snap.nodes) || [];
  const failed = Boolean(snap && snap.status === "error");
  const c = nodes.length ? resultCounts(nodes) : { done: 0, partial: 0, failed: 0 };
  const stopped = failed || c.failed === nodes.length && nodes.length > 0;
  const mixed = !stopped && (c.partial > 0 || c.failed > 0);
  $("results-title").textContent = stopped ? "Collection stopped" : mixed ? "Collection finished with gaps" : "Collection finished";
  $("results-message").textContent = (snap && snap.message) || (nodes.length ? "Every selected node was collected." : "");
  $("results-path").textContent = state.outputDir || "(no output directory)";
  $("copy-path").hidden = !state.outputDir;
  const canAnalyze = Boolean(state.outputDir) && !stopped;
  $("analyze-mra").hidden = !canAnalyze;
  $("analyze-mra-btn").hidden = !canAnalyze;
  const pathBox = $("results-path").closest(".result-path");
  pathBox.classList.toggle("bad", stopped);
  pathBox.classList.toggle("partial", mixed);
  if (nodes.length) renderResultStats(nodes, stopped);
  else $("results-stats").replaceChildren();
  renderResultsDiagram(nodes);
  const wrap = $("results-nodes");
  wrap.replaceChildren();
  for (const node of nodes) wrap.append(renderNodeCard(node));
}

function handleEvent(ev) {
  if (ev.type === "log") appendLog(ev);
  else if (ev.type === "snapshot") renderSnapshot(ev);
  else if (ev.type === "ssh_prompt" && ev.prompt) addPrompt(ev.prompt);
  else if (ev.type === "ssh_prompt_done" && ev.prompt) removePrompt(ev.prompt.id);
}

function openEvents() {
  if (demoKind) return;
  if (state.events) state.events.close();
  $("activity-log").replaceChildren();
  const source = new EventSource("/api/events");
  state.events = source;
  source.onmessage = (message) => {
    try {
      handleEvent(JSON.parse(message.data));
    } catch (_) {
      /* ignore malformed events */
    }
  };
}

// ---------- SSH prompts ----------

function addPrompt(p) {
  if (state.prompts.some((x) => x.id === p.id)) return;
  state.prompts.push(p);
  syncPromptModal();
}

function removePrompt(id) {
  state.prompts = state.prompts.filter((x) => x.id !== id);
  syncPromptModal();
}

function promptCopy(p) {
  const text = (p.text || "").toLowerCase();
  if (p.kind === "confirm") {
    return {
      title: "Trust this host?",
      icon: "shield",
      note: "ssh has not seen this host key before. Check the fingerprint with the host owner if you are unsure.",
      send: "Trust and continue",
      cancel: "Reject",
    };
  }
  if (text.includes("passphrase")) return { title: "SSH key passphrase needed", icon: "key", note: "Sent to ssh only. Not stored or logged.", send: "Unlock", cancel: "Cancel" };
  if (text.includes("verification") || text.includes("otp") || text.includes("token")) return { title: "Verification code needed", icon: "shield", note: "Sent to ssh only. Not stored or logged.", send: "Verify", cancel: "Cancel" };
  return { title: "SSH password needed", icon: "lock", note: "Sent to ssh only. Not stored or logged. MongoDB Data Collector asks once per node.", send: "Connect", cancel: "Cancel" };
}

function syncPromptModal() {
  const modal = $("ssh-modal");
  const next = state.prompts[0] || null;
  if (state.step === "collect" || state.running) $("collect-message").textContent = collectMessage();
  if (!next) {
    state.activePrompt = null;
    const returnFocus = state.promptReturnFocus;
    state.promptReturnFocus = null;
    modal.hidden = true;
    if (returnFocus && returnFocus.isConnected) returnFocus.focus();
    return;
  }
  const queued = state.prompts.length - 1;
  $("ssh-queue").hidden = queued <= 0;
  $("ssh-queue").textContent = queued > 0 ? queued + " more prompt" + (queued === 1 ? "" : "s") + " waiting" : "";
  if (state.activePrompt && state.activePrompt.id === next.id && !modal.hidden) return;
  state.activePrompt = next;
  const copy = promptCopy(next);
  $("ssh-title").textContent = copy.title;
  $("ssh-icon").innerHTML = ICONS[copy.icon];
  $("ssh-icon").className = "modal-icon" + (next.kind === "confirm" ? " confirm" : "");
  $("ssh-context").textContent = next.context ? "ssh " + next.context : "An SSH connection is waiting for input.";
  $("ssh-prompt").textContent = (next.text || "").trim();
  $("ssh-prompt").hidden = !next.text;
  $("ssh-note").textContent = copy.note;
  $("ssh-send").textContent = copy.send;
  $("ssh-cancel").textContent = copy.cancel;
  $("ssh-error").hidden = true;
  $("ssh-input-row").hidden = next.kind === "confirm";
  $("ssh-answer").value = "";
  $("ssh-answer").type = "password";
  $("ssh-reveal").textContent = "Show";
  $("ssh-send").disabled = false;
  if (modal.hidden) state.promptReturnFocus = document.activeElement;
  modal.hidden = false;
  if (next.kind === "confirm") $("ssh-send").focus();
  else $("ssh-answer").focus();
}

function promptFocusable() {
  return Array.from($("ssh-form").querySelectorAll("button, input, [tabindex]"))
    .filter((element) => !element.hidden && !element.disabled && element.getClientRects().length);
}

async function answerPrompt(cancel) {
  const p = state.activePrompt;
  if (!p) return;
  const answer = p.kind === "confirm" ? (cancel ? "no" : "yes") : cancel ? "" : $("ssh-answer").value;
  if (!cancel && p.kind !== "confirm" && !answer) {
    $("ssh-error").hidden = false;
    $("ssh-error").textContent = "Type the answer, or choose Cancel.";
    return;
  }
  $("ssh-send").disabled = true;
  try {
    await api("/api/ssh-answer", { id: p.id, answer, cancel: cancel && p.kind !== "confirm" });
    $("ssh-answer").value = "";
    removePrompt(p.id);
  } catch (err) {
    $("ssh-send").disabled = false;
    if (err.status === 404) {
      removePrompt(p.id);
      return;
    }
    $("ssh-error").hidden = false;
    $("ssh-error").textContent = err.message;
  }
}

$("ssh-form").addEventListener("submit", (e) => {
  e.preventDefault();
  answerPrompt(false);
});
$("ssh-cancel").addEventListener("click", () => answerPrompt(true));
$("ssh-reveal").addEventListener("click", () => {
  const input = $("ssh-answer");
  input.type = input.type === "password" ? "text" : "password";
  $("ssh-reveal").textContent = input.type === "password" ? "Show" : "Hide";
  input.focus();
});
document.addEventListener("keydown", (e) => {
  if ($("ssh-modal").hidden) return;
  if (e.key === "Escape") {
    e.preventDefault();
    answerPrompt(true);
    return;
  }
  if (e.key !== "Tab") return;
  const focusable = promptFocusable();
  if (!focusable.length) {
    e.preventDefault();
    return;
  }
  const first = focusable[0];
  const last = focusable[focusable.length - 1];
  if (!$("ssh-form").contains(document.activeElement)) {
    e.preventDefault();
    (e.shiftKey ? last : first).focus();
  } else if (e.shiftKey && document.activeElement === first) {
    e.preventDefault();
    last.focus();
  } else if (!e.shiftKey && document.activeElement === last) {
    e.preventDefault();
    first.focus();
  }
});

// ---------- Session ----------

function fillConnection(conn) {
  if (!conn) return;
  $("cluster-name").value = conn.cluster_name || "";
  $("seed-host").value = conn.seed_host || "";
  $("seed-port").value = conn.seed_port || "";
  $("username").value = conn.username || "";
  $("uri-options").value = conn.uri_options || "";
  if (conn.max_collections) $("max-collections").value = String(conn.max_collections);
  state.hasPassword = Boolean(conn.has_password);
  state.savedUsername = conn.username || "";
  state.seed = { host: conn.seed_host || "localhost", port: conn.seed_port || "27017" };
  updatePasswordHint();
}

// The server keeps the password only for the user and seed it was entered for,
// so the "saved password" shortcut applies only while the form still matches.
function storedPasswordApplies(username) {
  if (!state.hasPassword || !state.savedUsername || username !== state.savedUsername) return false;
  const host = $("seed-host").value.trim() || "localhost";
  const port = $("seed-port").value.trim() || "27017";
  return Boolean(state.seed) && host === state.seed.host && port === state.seed.port;
}

function updatePasswordHint() {
  $("password-hint").hidden = !storedPasswordApplies($("username").value.trim());
}

function pickScope() {
  if (!state.modes.some((mode) => mode.id === state.scope)) {
    const preferred = state.modes.find((mode) => mode.id === "one-secondary") || state.modes.find((mode) => !mode.error);
    if (preferred) state.scope = preferred.id;
  }
}

function applySession(data) {
  state.discovered = Boolean(data.discovered);
  state.running = Boolean(data.running);
  state.finished = Boolean(data.finished);
  state.nodes = data.nodes || [];
  state.modes = data.modes || [];
  state.outputDir = data.output_dir || "";
  fillConnection(data.connection);
  for (const p of data.prompts || []) addPrompt(p);
  if (state.modes.length) {
    pickScope();
    renderModes();
  }
}

// ---------- Wiring ----------

document.querySelectorAll(".step-btn").forEach((btn) => {
  btn.addEventListener("click", () => {
    if (canOpen(btn.dataset.step)) show(btn.dataset.step);
  });
});

function labelThemeToggle() {
  const label = document.documentElement.getAttribute("data-theme") === "dark" ? "Switch to light mode" : "Switch to dark mode";
  $("theme-toggle").setAttribute("aria-label", label);
  $("theme-toggle").title = label;
}
$("theme-toggle").addEventListener("click", () => {
  const next = document.documentElement.getAttribute("data-theme") === "dark" ? "light" : "dark";
  document.documentElement.setAttribute("data-theme", next);
  localStorage.setItem("dcrcli-theme", next);
  labelThemeToggle();
});
labelThemeToggle();

$("view-diagram").addEventListener("click", () => {
  setView("diagram");
  renderTopology();
});
$("view-table").addEventListener("click", () => setView("table"));

document.querySelectorAll("#connect-form input").forEach((input) => {
  input.addEventListener("input", () => {
    const label = input.closest("label.invalid");
    if (label) {
      label.classList.remove("invalid");
      const msg = label.querySelector(".field-msg");
      if (msg) msg.remove();
    }
    if (input.id === "username" || input.id === "seed-host" || input.id === "seed-port") updatePasswordHint();
  });
});

function connectBody() {
  return {
    cluster_name: $("cluster-name").value.trim(),
    seed_host: $("seed-host").value.trim(),
    seed_port: $("seed-port").value.trim(),
    username: $("username").value.trim(),
    password: $("password").value,
    uri_options: $("uri-options").value.trim(),
    max_collections: Number($("max-collections").value || 0),
  };
}

function precheck(body) {
  if (!body.cluster_name) {
    return { code: "cluster_name", field: "cluster_name", title: "Give this collection a name", message: "The cluster name becomes part of the output folder, so MongoDB Data Collector needs one before it connects.", hints: ["Use letters, digits, dots, dashes, or underscores, for example prod-rs0."] };
  }
  const storedPassword = storedPasswordApplies(body.username);
  if (body.username && !body.password && !storedPassword) {
    return {
      code: "password_missing",
      field: "password",
      title: "Password missing for " + body.username,
      message: "You entered a MongoDB username but no password. With auth enabled, the connection will be rejected.",
      hints: [
        "Type the password for " + body.username + ". It stays in this process's memory only and is never written to disk or logs.",
        "If this deployment has authentication turned off, clear the username or connect without a password.",
      ],
    };
  }
  return null;
}

async function discover(skipPrecheck) {
  clearProblem();
  const body = connectBody();
  if (!skipPrecheck) {
    const p = precheck(body);
    if (p) {
      const actions = p.code === "password_missing"
        ? [
          { label: "Enter password", run: () => { clearProblem(); markField("password", "Password required"); $("password").focus(); } },
          { label: "Connect without a password", run: () => discover(true) },
        ]
        : null;
      showProblem(p, actions);
      return;
    }
  }
  const button = $("discover-btn");
  button.disabled = true;
  button.textContent = "Discovering…";
  try {
    const data = await api("/api/discover", body);
    $("password").value = "";
    state.discovered = true;
    state.finished = false;
    state.running = false;
    state.snapshot = null;
    state.nodes = data.nodes || [];
    state.modes = data.modes || [];
    fillConnection(data.connection);
    state.scope = "one-secondary";
    pickScope();
    renderModes();
    show("topology");
  } catch (err) {
    const p = problemFromError(err);
    const actions = [];
    if (p.field === "password" || p.code === "auth_failed") {
      actions.push({ label: "Re-enter password", run: () => { $("password").value = ""; $("password").focus(); } });
    }
    if (p.details) {
      actions.push({ label: "Retry", run: () => discover(true) });
    }
    showProblem(p, actions);
  } finally {
    button.disabled = false;
    button.textContent = "Discover topology";
  }
}

$("connect-form").addEventListener("submit", (event) => {
  event.preventDefault();
  discover(false);
});

$("guide-to-connect").addEventListener("click", () => show("connect"));
$("back-connect").addEventListener("click", () => show("connect"));
$("to-scope").addEventListener("click", () => {
  renderModes();
  show("scope");
});
$("back-topology").addEventListener("click", () => show("topology"));

for (const id of ["art-gmd", "art-ftdc", "art-logs", "art-commands"]) {
  $(id).addEventListener("change", () => {
    updateSSHField();
    updateStartButton();
  });
}
$("allow-standalone").addEventListener("change", () => {
  updateStartButton();
  renderScopeDiagram();
});

$("start-collect").addEventListener("click", async () => {
  clearProblem();
  $("start-collect").disabled = true;
  try {
    await api("/api/collect", {
      collect_nodes: state.scope,
      collect_data: collectDataValue(),
      ssh_username: needsSSH() ? $("ssh-username").value.trim() : "",
      allow_standalone_primary: $("allow-standalone").checked,
    });
    state.running = true;
    state.finished = false;
    state.snapshot = null;
    $("activity-log").replaceChildren();
    $("progress-nodes").replaceChildren();
    $("bar-fill").style.width = "0%";
    $("bar-label").textContent = "0%";
    show("collect");
    $("collect-message").textContent = collectMessage();
    openEvents();
    if (demoKind) Demo.collect();
  } catch (err) {
    showProblem(problemFromError(err));
    updateStartButton();
  }
});

$("view-log").addEventListener("click", () => show("collect"));
$("copy-path").addEventListener("click", async () => {
  const path = $("results-path").textContent;
  try {
    await navigator.clipboard.writeText(path);
    $("copy-path").textContent = "Copied";
  } catch (_) {
    $("copy-path").textContent = "Copy failed";
  }
  setTimeout(() => ($("copy-path").textContent = "Copy path"), 1500);
});
$("start-over").addEventListener("click", async () => {
  try {
    await api("/api/reset", {});
  } catch (err) {
    showProblem(problemFromError(err));
    return;
  }
  if (state.events) state.events.close();
  Object.assign(state, { discovered: false, running: false, finished: false, nodes: [], modes: [], snapshot: null, outputDir: "", hasPassword: false, savedUsername: "", prompts: [] });
  syncPromptModal();
  $("password").value = "";
  $("password-hint").hidden = true;
  show("connect");
});

// ---------- Demo mode (?demo=sharded|rs|standalone) ----------

const Demo = (function () {
  function n(hostname, port, role, shard) {
    return { hostname, port, role, shard_role: shard || "" };
  }
  const clusters = {
    sharded: {
      seed: { host: "mongos-a.example.net", port: "27017" },
      nodes: [
        n("mongos-a.example.net", 27017, "MONGOS"),
        n("mongos-b.example.net", 27017, "MONGOS"),
        n("shard0-a.example.net", 27018, "PRIMARY", "shard0"),
        n("shard0-b.example.net", 27018, "SECONDARY", "shard0"),
        n("shard0-c.example.net", 27018, "SECONDARY", "shard0"),
        n("shard1-a.example.net", 27018, "SECONDARY", "shard1"),
        n("shard1-b.example.net", 27018, "PRIMARY", "shard1"),
        n("shard1-c.example.net", 27018, "SECONDARY", "shard1"),
        n("cfg-a.example.net", 27019, "PRIMARY", "config"),
        n("cfg-b.example.net", 27019, "SECONDARY", "config"),
        n("cfg-c.example.net", 27019, "SECONDARY", "config"),
      ],
    },
    rs: {
      seed: { host: "rs0-a.example.net", port: "27017" },
      nodes: [
        n("rs0-a.example.net", 27017, "PRIMARY"),
        n("rs0-b.example.net", 27017, "SECONDARY"),
        n("rs0-c.example.net", 27017, "SECONDARY"),
        n("rs0-arb.example.net", 27017, "ARBITER"),
      ],
    },
    standalone: {
      seed: { host: "localhost", port: "27017" },
      nodes: [n("localhost", 27017, "PRIMARY")],
    },
  };
  const cluster = clusters[demoKind] || clusters.sharded;
  let pendingAnswer = null;

  function modes() {
    const nodes = cluster.nodes;
    const secs = nodes.filter((x) => x.role === "SECONDARY");
    const mongos = nodes.filter((x) => x.role === "MONGOS");
    const cfg = nodes.filter((x) => x.shard_role === "config" && x.role === "SECONDARY");
    const standalone = secs.length === 0;
    const err = standalone ? "no secondary nodes found" : "";
    const allSecs = secs.filter((x) => x.shard_role !== "config");
    const allSecTargets = mongos.length ? allSecs.concat(mongos.slice(0, 1), cfg.slice(0, 1)) : allSecs;
    return [
      { id: "one-secondary", label: "One secondary", description: "one secondary only", targets: standalone ? nodes.slice(0, 1) : secs.slice(0, 1), error: err, standalone },
      { id: "all-secondaries", label: "All secondaries", description: "all secondaries plus one mongos and one config server when sharded", targets: standalone ? nodes.slice(0, 1) : allSecTargets, error: err, standalone },
      { id: "all-nodes", label: "All nodes", description: "all discovered nodes (every mongod primary/secondary, all mongos, all config members)", targets: nodes.filter((x) => x.role !== "ARBITER") },
    ];
  }

  const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

  async function api(path, body) {
    await sleep(path === "/api/discover" ? 650 : 120);
    if (path === "/api/state") return { discovered: false };
    if (path === "/api/discover") {
      if (body.username && !body.password) {
        throw new ApiError("auth failed", {
          code: "password_missing",
          field: "password",
          title: "Authentication failed: no password was sent",
          message: "The server rejected user " + body.username + " because the password was empty.",
          hints: ["Type the password and try again.", "If auth is off on this deployment, clear the username."],
          details: "MongoServerError: Authentication failed.\n  at Connection.onMessage (demo)",
        }, 400);
      }
      return {
        nodes: cluster.nodes,
        modes: modes(),
        connection: { cluster_name: body.cluster_name, seed_host: cluster.seed.host, seed_port: cluster.seed.port, username: body.username, has_password: Boolean(body.password), uri_options: body.uri_options, max_collections: body.max_collections },
      };
    }
    if (path === "/api/ssh-answer") {
      if (pendingAnswer) pendingAnswer(body);
      return { status: "sent" };
    }
    if (path === "/api/collect") {
      state._demoReq = body;
      return { status: "started" };
    }
    return { status: "ok" };
  }

  function now() {
    return new Date().toISOString();
  }
  function log(message, level) {
    handleEvent({ type: "log", level: level || "info", time: now(), message });
  }

  async function collect() {
    const req = state._demoReq || {};
    const mode = modes().find((m) => m.id === req.collect_nodes) || modes()[0];
    const data = req.collect_data || "all";
    const want = {
      getMongoData: data === "all" || data.includes("getmongodata"),
      FTDC: data === "all" || data.includes("ftdc"),
      logs: data === "all" || data.includes("logs"),
      commands: data === "all" || data.split(",").includes("commands"),
    };
    const nodes = mode.targets.map((t) => ({
      hostname: t.hostname, port: t.port, role: t.role, shard_role: t.shard_role, state: "pending",
      tasks: Object.keys(want).map((name) => ({ name, state: want[name] ? "pending" : "omitted" })),
    }));
    const total = nodes.reduce((a, x) => a + x.tasks.filter((t) => t.state !== "omitted").length, 0);
    let doneCount = 0;
    const outputDir = "./outputs/" + (($("cluster-name").value || "demo").trim()) + "/demo-" + Date.now();
    const snap = (status, message) => handleEvent({ type: "snapshot", status, message, percent: Math.round((doneCount / total) * 100), output_dir: outputDir, nodes: JSON.parse(JSON.stringify(nodes)) });

    log("Demo collection started. Nothing is contacted.");
    snap("running", "Starting collection…");
    for (const [i, node] of nodes.entries()) {
      node.state = "active";
      for (const task of node.tasks) {
        if (task.state === "omitted") continue;
        if (i === 0 && (task.name === "FTDC" || task.name === "logs") && !state._demoAsked) {
          state._demoAsked = true;
          const answered = new Promise((resolve) => (pendingAnswer = resolve));
          handleEvent({ type: "ssh_prompt", prompt: { id: "demo-1", kind: "secret", text: (req.ssh_username || "mongod") + "@" + node.hostname + "'s password:", context: (req.ssh_username || "mongod") + "@" + node.hostname + ", for FTDC, logs, and host commands on " + node.hostname + ":" + node.port } });
          snap("running", "Waiting for SSH…");
          const ans = await answered;
          handleEvent({ type: "ssh_prompt_done", prompt: { id: "demo-1" } });
          if (ans.cancel) {
            log("SSH prompt cancelled in the browser", "warn");
            for (const t of node.tasks) if (t.name === "FTDC" || t.name === "logs") if (t.state === "pending") { t.state = "failed"; t.note = "SSH connection failed: cancelled"; doneCount++; }
          } else {
            log("SSH connection to " + node.hostname + " established");
          }
        }
        if (task.state !== "pending") continue;
        task.state = "running";
        snap("running", "Collecting " + task.name + " from " + node.hostname + ":" + node.port);
        log("Collecting " + task.name + " from " + node.hostname + ":" + node.port);
        await sleep(650 + Math.random() * 600);
        task.state = "ok";
        doneCount++;
        snap("running", "Collected " + task.name + " from " + node.hostname + ":" + node.port);
      }
      node.state = node.tasks.some((t) => t.state === "failed") ? "partial" : "done";
      snap("running", node.hostname + " finished");
      await sleep(300);
    }
    log("Demo collection finished.");
    snap("done", "Demo collection finished. In a real run the archive would be in " + outputDir + ".");
  }

  return { api, collect, cluster };
})();

// ---------- Boot ----------

async function boot() {
  if (demoKind) {
    $("demo-pill").hidden = false;
    $("demo-pill").textContent = "Demo data · " + demoKind;
    $("demo-link").textContent = "Leave demo";
    $("demo-link").href = "/";
    if (!$("cluster-name").value) $("cluster-name").value = "demo-" + demoKind;
    $("seed-host").value = Demo.cluster.seed.host;
    $("seed-port").value = Demo.cluster.seed.port;
    show("connect");
    return;
  }
  try {
    const data = await api("/api/state");
    applySession(data);
    if (data.running) {
      show("collect");
      openEvents();
    } else if (data.finished) {
      show("results");
      openEvents();
    } else if (data.discovered) {
      show("topology");
    } else {
      show("connect");
    }
  } catch (err) {
    show("connect");
    showProblem(problemFromError(err));
  }
}

boot();
