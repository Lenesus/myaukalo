(() => {
  "use strict";

  const LS_ID = "myaukalo.identity";
  const LS_THEME = "myaukalo.theme";
  const LS_CAT = "myaukalo.cat";
  const RING_CIRC = 2 * Math.PI * 6;
  const EMOJIS = ["👍", "❤️", "😂", "🐱", "🔥", "", "🎉", "😢"];
  const TYPING_TTL = 3000;
  const TYPING_THROTTLE = 1500;

  const state = {
    identity: null,
    chats: [],
    activeChatId: null,
    messages: [],
    seenIds: new Set(),
    es: null,
    theme: "dark",
    cat: false,
    ttl: "",
    chatQuery: "",
    msgQuery: "",
    typing: {},          // name -> last-seen ts
    lastTypingSent: 0,
    pickerFor: null,
  };

  const $ = (s) => document.querySelector(s);
  const enc = (s) => encodeURIComponent(s);
  const el = (tag, cls, text) => {
    const n = document.createElement(tag);
    if (cls) n.className = cls;
    if (text != null) n.textContent = text;
    return n;
  };
  const q = (id) => document.querySelector('.msg[data-id="' + String(id).replace(/\\/g, "\\\\").replace(/"/g, '\\"') + '"]');
  const fmtTime = (iso) => { const d = new Date(iso); return isNaN(d) ? "" : d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" }); };
  const fnv1a = (str) => { let h = 0x811c9dc5; for (let i = 0; i < str.length; i++) { h ^= str.charCodeAt(i); h = Math.imul(h, 0x01000193); } return h >>> 0; };
  const fmtLeft = (ms) => { if (ms <= 0) return "0s"; const s = Math.floor(ms / 1000); if (s >= 3600) return Math.floor(s / 3600) + "h"; if (s >= 60) return Math.floor(s / 60) + "m"; return s + "s"; };
  const matches = (needle, hay) => { needle = needle.trim().toLowerCase(); if (!needle) return true; return String(hay || "").toLowerCase().includes(needle); };
  const norm = (m) => ({ id: m.id, chatId: m.chat_id, author: m.author, text: m.text, createdAt: m.created_at, expiresAt: m.expires_at || null, reactions: m.reactions || [] });

  // ---------- идентичность ----------
  const ADJ = ["полосатик","пушистик","рыжик","мурзик","барсик","снежок","уголёк","персик","зефирка","масяня","тигрик","котлетка"];
  const rollName = () => ADJ[Math.floor(Math.random() * ADJ.length)] + "_" + Math.floor(Math.random() * 0xfff).toString(16).toUpperCase().padStart(3, "0");
  const loadIdentity = () => {
    let raw = null;
    try { raw = localStorage.getItem(LS_ID); } catch (_) {}
    if (raw) { try { state.identity = JSON.parse(raw); } catch (_) { raw = null; } }
    if (!raw) { state.identity = { name: rollName(), seed: Math.random().toString(36).slice(2) }; saveIdentity(); }
  };
  const saveIdentity = () => { try { localStorage.setItem(LS_ID, JSON.stringify(state.identity)); } catch (_) {} };
  const exportIdentity = () => {
    const blob = new Blob([JSON.stringify(state.identity, null, 2)], { type: "application/json" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url; a.download = "myaukalo-identity.json";
    document.body.appendChild(a); a.click(); a.remove();
    URL.revokeObjectURL(url);
  };
  const importIdentity = (file) => {
    const reader = new FileReader();
    reader.onload = () => {
      try {
        const obj = JSON.parse(reader.result);
        if (!obj || typeof obj.name !== "string" || typeof obj.seed !== "string") throw new Error("bad shape");
        state.identity = { name: obj.name, seed: obj.seed };
        saveIdentity(); renderIdentityCard(); renderChatList(); renderFeed();
      } catch (_) {}
    };
    reader.readAsText(file);
  };

  // ---------- аватар ----------
  const COATS = ["#e8a87c","#c98b5e","#9c9c9c","#5b5b5b","#f0e6d2","#d98c5f","#7a8b6f"];
  const avatarSVG = (seed) => {
    const h = fnv1a(String(seed));
    const coat = COATS[h % COATS.length];
    const eye = ["#3a2a1a","#274058","#4a3a2a","#1f8a4c"][(h >> 3) % 4];
    const ear = (h >> 6) % 3;
    const whisk = (h >> 9) % 2;
    const earPath = ear === 0 ? "M8,12 L4,2 L14,8 Z M24,12 L28,2 L18,8 Z"
      : ear === 1 ? "M7,11 L6,3 L15,8 Z M25,11 L26,3 L17,8 Z"
      : "M9,12 L7,4 L15,9 Z M23,12 L25,4 L17,9 Z";
    const w = whisk ? '<path d="M6,18 L1,17 M6,20 L1,21 M26,18 L31,17 M26,20 L31,21" stroke="#00000022" stroke-width="1"/>' : "";
    return '<svg viewBox="0 0 32 32" width="22" height="22" role="img" aria-label="avatar">' +
      '<path d="' + earPath + '" fill="' + coat + '"/>' +
      '<circle cx="16" cy="18" r="11" fill="' + coat + '"/>' +
      '<circle cx="12" cy="16" r="1.6" fill="' + eye + '"/>' +
      '<circle cx="20" cy="16" r="1.6" fill="' + eye + '"/>' +
      '<path d="M16,19 l-1.4,1.4 h2.8 z" fill="#7a4a3a"/>' + w + "</svg>";
  };

  // ---------- темы ----------
  const applyPrefs = () => {
    let t = null, c = null;
    try { t = localStorage.getItem(LS_THEME); c = localStorage.getItem(LS_CAT); } catch (_) {}
    if (!t) t = window.matchMedia && window.matchMedia("(prefers-color-scheme: light)").matches ? "light" : "dark";
    state.theme = t; state.cat = c === "on";
    document.documentElement.dataset.theme = state.theme;
    document.documentElement.dataset.cat = state.cat ? "on" : "off";
    $("#themeToggle").textContent = state.theme === "dark" ? "☾" : "☀";
    $("#catToggle").classList.toggle("active", state.cat);
  };
  const toggleTheme = () => {
    state.theme = state.theme === "dark" ? "light" : "dark";
    document.documentElement.dataset.theme = state.theme;
    $("#themeToggle").textContent = state.theme === "dark" ? "☾" : "☀";
    try { localStorage.setItem(LS_THEME, state.theme); } catch (_) {}
  };
  const toggleCat = () => {
    state.cat = !state.cat;
    document.documentElement.dataset.cat = state.cat ? "on" : "off";
    $("#catToggle").classList.toggle("active", state.cat);
    try { localStorage.setItem(LS_CAT, state.cat ? "on" : "off"); } catch (_) {}
    renderChatList(); renderFeed(); renderIdentityCard();
  };

  // ---------- API ----------
  const api = async (path, opts) => {
    const res = await fetch(path, Object.assign({ headers: { "Content-Type": "application/json" } }, opts || {}));
    const data = await res.json().catch(() => ({}));
    if (!res.ok) throw Object.assign(new Error(data.error || res.statusText), { status: res.status });
    return data;
  };

  // ---------- чаты ----------
  const loadChats = async () => {
    try {
      const data = await api("/api/v1/chats");
      state.chats = Array.isArray(data.items) ? data.items : (Array.isArray(data) ? data : []);
    } catch (_) { state.chats = []; }
    renderChatList();
  };
  const renderChatList = () => {
    const ul = $("#chatlist"); ul.textContent = "";
    const shown = state.chats.filter((c) => matches(state.chatQuery, c.title));
    const emptyEl = $("#chatsEmpty");
    emptyEl.style.display = shown.length ? "none" : "block";
    emptyEl.textContent = (state.chatQuery.trim() && state.chats.length && !shown.length) ? "no rooms match" : "no rooms. press + to start";
    for (const chat of shown) {
      const li = el("li", "chatrow" + (chat.id === state.activeChatId ? " active" : ""));
      li.dataset.id = chat.id;
      const av = el("span", "avatar"); if (state.cat) av.innerHTML = avatarSVG(chat.id);
      li.appendChild(av);
      li.appendChild(el("span", "dot", "●"));
      li.appendChild(el("span", "cname", chat.title || "(untitled)"));
      li.appendChild(el("span", "num", chat.unreadCount ? String(chat.unreadCount) : ""));
      li.addEventListener("click", () => openChat(chat.id));
      ul.appendChild(li);
    }
  };

  // ---------- комната ----------
  const openChat = async (id) => {
    state.activeChatId = id;
    state.messages = []; state.seenIds = new Set();
    state.msgQuery = ""; const ms = $("#msgSearch"); if (ms) ms.value = "";
    state.typing = {}; renderTyping();
    document.body.classList.remove("view-list"); document.body.classList.add("view-chat");
    renderChatList();
    let chat = state.chats.find((c) => c.id === id);
    if (!chat) { try { chat = await api("/api/v1/chats/" + enc(id)); } catch (_) { chat = { id, title: "(room)" }; } }
    $("#roomTitle").textContent = chat.title || "(untitled)";
    $("#roomMeta").textContent = "";
    try {
      const data = await api("/api/v1/chats/" + enc(id) + "/messages");
      state.messages = (Array.isArray(data) ? data : (data.items || [])).map(norm);
    } catch (_) { state.messages = []; }
    for (const m of state.messages) state.seenIds.add(m.id);
    renderFeed();
    connectSSE(id);
    api("/api/v1/chats/" + enc(id) + "/read", { method: "POST" }).then(loadChats).catch(() => {});
  };

  const renderFeed = () => {
    const feed = $("#feed"); feed.textContent = "";
    const shown = state.messages.filter((m) => matches(state.msgQuery, m.text));
    const emptyEl = $("#feedEmpty");
    emptyEl.style.display = shown.length ? "none" : "block";
    emptyEl.textContent = (state.msgQuery.trim() && state.messages.length && !shown.length) ? "no messages match" : "nothing here yet";
    for (const m of shown) feed.appendChild(buildMsg(m, false));
    scrollFeed();
  };

  const buildTTL = (m) => {
    if (!m.expiresAt) return null;
    const createdMs = Date.parse(m.createdAt || "") || 0;
    const expiresMs = Date.parse(m.expiresAt) || 0;
    if (expiresMs <= 0) return null;
    const ttl = el("span", "ttl");
    ttl.dataset.expires = String(expiresMs);
    ttl.dataset.created = String(createdMs);
    ttl.innerHTML = '<svg class="ring" viewBox="0 0 16 16"><circle class="ring-bg" cx="8" cy="8" r="6"/><circle class="ring-fg" cx="8" cy="8" r="6" stroke-dasharray="' + RING_CIRC.toFixed(2) + '" stroke-dashoffset="0"/></svg><span class="ttl-text"></span>';
    return ttl;
  };

  const renderRx = (rx, m) => {
    rx.textContent = "";
    for (const r of (m.reactions || [])) {
      const mine = (r.users || []).includes(state.identity.name);
      const chip = el("button", "rchip" + (mine ? " mine" : ""), r.emoji + " " + (r.users || []).length);
      chip.type = "button";
      chip.title = (r.users || []).join(", ");
      chip.addEventListener("click", (e) => { e.stopPropagation(); toggleReaction(m.id, r.emoji); });
      rx.appendChild(chip);
    }
    const add = el("button", "react-add", "＋");
    add.type = "button"; add.title = "add reaction";
    add.addEventListener("click", (e) => { e.stopPropagation(); openPicker(m.id, add); });
    rx.appendChild(add);
  };

  const buildMsg = (m, animate) => {
    const mine = m.author === state.identity.name;
    const row = el("div", "msg " + (mine ? "me" : "other") + (animate ? " enter" : "") + (m._pending ? " pending" : "") + (m._failed ? " failed" : ""));
    row.dataset.id = m.id;
    row.appendChild(el("span", "ts", fmtTime(m.createdAt)));
    row.appendChild(el("span", "alias", mine ? "you" : m.author));
    const slot = el("span", "avatar-slot"); if (state.cat) slot.innerHTML = avatarSVG(mine ? state.identity.seed : m.author);
    row.appendChild(slot);
    row.appendChild(el("span", "body", m.text));
    const ttl = buildTTL(m); if (ttl) row.appendChild(ttl);
    const rx = el("span", "rx"); renderRx(rx, m); row.appendChild(rx);
    return row;
  };

  // merge по SSE-republish: реакции едут в event: message с тем же id,
  // поэтому дубликат НЕ отбрасываем, а обновляем реакции/TTL существующей ноды.
  const mergeMessage = (m) => {
    const idx = state.messages.findIndex((x) => x.id === m.id);
    if (idx >= 0) state.messages[idx] = m;
    else { state.messages.push(m); state.seenIds.add(m.id); }
    const node = q(m.id);
    if (node) {
      const rx = node.querySelector(".rx"); if (rx) renderRx(rx, m);
      const oldTtl = node.querySelector(".ttl"); if (oldTtl) oldTtl.remove();
      const ttl = buildTTL(m); if (ttl) node.appendChild(ttl);
    } else if (m.chatId === state.activeChatId && matches(state.msgQuery, m.text)) {
      $("#feedEmpty").style.display = "none";
      $("#feed").appendChild(buildMsg(m, true));
      scrollFeed();
    }
  };

  const appendMsg = (m) => {
    if (state.seenIds.has(m.id)) { mergeMessage(m); return; }
    state.seenIds.add(m.id);
    state.messages.push(m);
    if (m.chatId !== state.activeChatId) return;
    if (!matches(state.msgQuery, m.text)) return;
    $("#feedEmpty").style.display = "none";
    $("#feed").appendChild(buildMsg(m, true));
    scrollFeed();
    loadChats();
  };

  const scrollFeed = () => { const f = $("#feed"); f.scrollTop = f.scrollHeight; };

  // ---------- реакции ----------
  const toggleReaction = async (messageId, emoji) => {
    const m = state.messages.find((x) => x.id === messageId);
    if (!m) return;
    const mine = (m.reactions || []).some((r) => r.emoji === emoji && (r.users || []).includes(state.identity.name));
    try {
      const saved = await api("/api/v1/chats/" + enc(m.chatId) + "/messages/" + enc(messageId) + "/reactions", {
        method: mine ? "DELETE" : "POST",
        body: JSON.stringify({ user: state.identity.name, emoji }),
      });
      mergeMessage(norm(saved));
    } catch (_) {}
  };

  const openPicker = (messageId, anchor) => {
    state.pickerFor = messageId;
    const p = $("#reactPicker");
    p.textContent = "";
    for (const e of EMOJIS) {
      const b = el("button", "rp-item", e);
      b.type = "button";
      b.addEventListener("click", () => { closePicker(); toggleReaction(messageId, e); });
      p.appendChild(b);
    }
    p.hidden = false;
    const r = anchor.getBoundingClientRect();
    const pw = p.offsetWidth, ph = p.offsetHeight;
    let left = Math.min(Math.max(8, r.left), window.innerWidth - pw - 8);
    let top = r.bottom + 6;
    if (top + ph > window.innerHeight - 8) top = r.top - ph - 6;
    p.style.left = left + "px"; p.style.top = top + "px";
  };
  const closePicker = () => { state.pickerFor = null; $("#reactPicker").hidden = true; };

  // ---------- typing ----------
  const notifyTyping = () => {
    if (!state.activeChatId) return;
    const now = Date.now();
    if (now - state.lastTypingSent < TYPING_THROTTLE) return;
    state.lastTypingSent = now;
    api("/api/v1/chats/" + enc(state.activeChatId) + "/typing", {
      method: "POST", body: JSON.stringify({ user: state.identity.name }),
    }).catch(() => {});
  };
  const renderTyping = () => {
    const names = Object.keys(state.typing);
    const bar = $("#typingBar");
    if (!names.length) { bar.hidden = true; bar.textContent = ""; return; }
    bar.hidden = false; bar.textContent = "";
    const dots = el("span", "tdots");
    dots.appendChild(el("i")); dots.appendChild(el("i")); dots.appendChild(el("i"));
    bar.appendChild(dots);
    bar.appendChild(el("span", "tlabel", names.length === 1 ? names[0] + " печатает…" : names.join(", ") + " печатают…"));
  };

  // ---------- тиккер ----------
  const tickTTL = () => {
    const now = Date.now();
    for (const node of document.querySelectorAll(".ttl[data-expires]")) {
      const exp = Number(node.dataset.expires) || 0;
      const cr = Number(node.dataset.created) || 0;
      const msgNode = node.closest(".msg");
      const id = msgNode ? msgNode.dataset.id : null;
      if (now >= exp) {
        if (msgNode) msgNode.remove();
        if (id) state.messages = state.messages.filter((x) => x.id !== id);
        if (!state.messages.length) $("#feedEmpty").style.display = "block";
        continue;
      }
      const total = (exp - cr) || 1;
      const frac = Math.min(1, Math.max(0, (exp - now) / total));
      const fg = node.querySelector(".ring-fg");
      const txt = node.querySelector(".ttl-text");
      if (fg) fg.setAttribute("stroke-dashoffset", (RING_CIRC * (1 - frac)).toFixed(2));
      if (txt) txt.textContent = fmtLeft(exp - now);
      node.classList.toggle("expiring", frac <= 0.1);
    }
    let changed = false;
    for (const u of Object.keys(state.typing)) {
      if (now - state.typing[u] > TYPING_TTL) { delete state.typing[u]; changed = true; }
    }
    if (changed) renderTyping();
  };

  // ---------- SSE ----------
  const setConn = (cls, title) => { const c = $("#conn"); c.className = "conn " + cls; c.title = title; };
  const connectSSE = (id) => {
    if (state.es) { state.es.close(); state.es = null; }
    const es = new EventSource("/api/v1/events?chat_id=" + enc(id));
    state.es = es;
    es.onopen = () => setConn("online", "connected");
    es.onerror = () => setConn("reconn", "reconnecting…");
    es.addEventListener("message", (ev) => {
      let m; try { m = JSON.parse(ev.data); } catch (_) { return; }
      appendMsg(norm(m));
    });
    es.addEventListener("typing", (ev) => {
      let d; try { d = JSON.parse(ev.data); } catch (_) { return; }
      const u = d.user;
      if (!u || u === state.identity.name) return;
      state.typing[u] = Date.now();
      renderTyping();
    });
  };

  // ---------- отправка ----------
  const sendMessage = async (text) => {
    text = text.trim();
    if (!text || !state.activeChatId) return;
    const tmpId = "tmp-" + Math.random().toString(36).slice(2);
    const optimistic = { id: tmpId, chatId: state.activeChatId, author: state.identity.name, text, createdAt: new Date().toISOString(), expiresAt: null, reactions: [], _pending: true };
    state.seenIds.add(tmpId);
    state.messages.push(optimistic);
    if (matches(state.msgQuery, text)) {
      $("#feedEmpty").style.display = "none";
      $("#feed").appendChild(buildMsg(optimistic, true));
      scrollFeed();
    }
    const body = { author: state.identity.name, text };
    if (state.ttl) body.ttl = state.ttl;
    try {
      const saved = norm(await api("/api/v1/chats/" + enc(state.activeChatId) + "/messages", { method: "POST", body: JSON.stringify(body) }));
      state.seenIds.delete(tmpId); state.seenIds.add(saved.id);
      const idx = state.messages.findIndex((x) => x.id === tmpId);
      if (idx >= 0) state.messages[idx] = saved;
      const node = q(tmpId);
      if (node) {
        node.dataset.id = saved.id;
        node.classList.remove("pending");
        const rx = node.querySelector(".rx"); if (rx) renderRx(rx, saved);
        const oldTtl = node.querySelector(".ttl"); if (oldTtl) oldTtl.remove();
        const ttl = buildTTL(saved); if (ttl) node.appendChild(ttl);
      }
    } catch (_) {
      optimistic._failed = true;
      const node = q(tmpId);
      if (node) { node.classList.remove("pending"); node.classList.add("failed"); }
    }
  };

  const createChat = async (title) => {
    const chat = await api("/api/v1/chats", { method: "POST", body: JSON.stringify({ title: title || "" }) });
    await loadChats();
    openChat(chat.id);
  };

  const renderIdentityCard = () => {
    $("#idName").textContent = state.identity.name;
    $("#idAvatar").innerHTML = state.cat ? avatarSVG(state.identity.seed) : "";
  };

  // ---------- wiring ----------
  const bind = () => {
    $("#themeToggle").addEventListener("click", toggleTheme);
    $("#catToggle").addEventListener("click", toggleCat);
    $("#backBtn").addEventListener("click", () => {
      document.body.classList.remove("view-chat"); document.body.classList.add("view-list");
      if (state.es) { state.es.close(); state.es = null; }
      state.activeChatId = null; state.typing = {}; renderTyping();
      renderChatList();
    });
    $("#identityBtn").addEventListener("click", () => { renderIdentityCard(); $("#identityDialog").showModal(); });
    $("#idDoneBtn").addEventListener("click", () => $("#identityDialog").close());
    $("#rerollBtn").addEventListener("click", () => { state.identity = { name: rollName(), seed: Math.random().toString(36).slice(2) }; saveIdentity(); renderIdentityCard(); renderFeed(); });
    $("#exportBtn").addEventListener("click", exportIdentity);
    $("#importBtn").addEventListener("click", () => $("#importFile").click());
    $("#importFile").addEventListener("change", (e) => { const f = e.target.files && e.target.files[0]; if (f) importIdentity(f); e.target.value = ""; });
    $("#chatSearch").addEventListener("input", (e) => { state.chatQuery = e.target.value; renderChatList(); });
    $("#msgSearch").addEventListener("input", (e) => { state.msgQuery = e.target.value; renderFeed(); });
    $("#ttlpicker").addEventListener("click", (e) => {
      const chip = e.target.closest(".chip"); if (!chip) return;
      state.ttl = chip.dataset.ttl || "";
      for (const c of $("#ttlpicker").querySelectorAll(".chip")) c.classList.toggle("active", c === chip);
    });
    $("#newChatBtn").addEventListener("click", () => { $("#newChatTitle").value = ""; $("#newChatDialog").showModal(); });
    $("#newChatCancel").addEventListener("click", () => $("#newChatDialog").close());
    $("#newChatForm").addEventListener("submit", (e) => { e.preventDefault(); createChat($("#newChatTitle").value).finally(() => $("#newChatDialog").close()); });
    $("#composer").addEventListener("submit", (e) => { e.preventDefault(); const v = $("#input").value; $("#input").value = ""; sendMessage(v); });
    $("#input").addEventListener("input", notifyTyping);
    $("#feed").addEventListener("scroll", closePicker);
    window.addEventListener("resize", closePicker);
    document.addEventListener("click", (e) => { if (!e.target.closest("#reactPicker") && !e.target.closest(".react-add")) closePicker(); });
    document.addEventListener("keydown", (e) => { if (e.key === "Escape") closePicker(); });
  };

  const boot = () => {
    loadIdentity(); applyPrefs(); bind(); loadChats();
    setInterval(tickTTL, 1000);
  };
  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", boot);
  else boot();
})();
