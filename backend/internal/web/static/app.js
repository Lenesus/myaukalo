(() => {
  "use strict";

  const LS_ID = "myaukalo.identity";
  const LS_THEME = "myaukalo.theme";
  const LS_CAT = "myaukalo.cat";

  // Длина окружности кольца: r=6 => 2*pi*6. Совпадает со stroke-width в CSS.
  const RING_CIRC = 2 * Math.PI * 6;

  const state = {
    identity: null,
    chats: [],
    activeChatId: null,
    messages: [],
    seenIds: new Set(),
    es: null,
    theme: "dark",
    cat: false,
    ttl: "",            // выбранная duration-строка для ОТПРАВКИ ("" = бессрочно)
    chatQuery: "",      // фильтр списка комнат по заголовку (клиентский)
    msgQuery: "",       // фильтр загруженных сообщений открытой комнаты по тексту (клиентский)
  };

  // ---------- утилиты ----------
  const $ = (sel) => document.querySelector(sel);
  const el = (tag, cls, text) => {
    const n = document.createElement(tag);
    if (cls) n.className = cls;
    if (text != null) n.textContent = text; // textContent => нет XSS из чужого текста
    return n;
  };
  const fmtTime = (iso) => {
    const d = new Date(iso);
    if (isNaN(d)) return "";
    return d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
  };
  const fnv1a = (str) => {
    let h = 0x811c9dc5;
    for (let i = 0; i < str.length; i++) {
      h ^= str.charCodeAt(i);
      h = Math.imul(h, 0x01000193);
    }
    return h >>> 0;
  };
  // Человекочитаемый остаток: >1ч => "Xh", >1м => "Xm", иначе "Xs".
  const fmtLeft = (ms) => {
    if (ms <= 0) return "0s";
    const s = Math.floor(ms / 1000);
    if (s >= 3600) return Math.floor(s / 3600) + "h";
    if (s >= 60) return Math.floor(s / 60) + "m";
    return s + "s";
  };
  const matches = (needle, hay) => {
    needle = needle.trim().toLowerCase();
    if (!needle) return true;
    return String(hay || "").toLowerCase().includes(needle);
  };

  // ---------- идентичность ----------
  const ADJ = ["полосатик","пушистик","рыжик","мурзик","барсик","снежок","уголёк","персик","зефирка","масяня","тигрик","котлетка"];
  const rollName = () => {
    const a = ADJ[Math.floor(Math.random() * ADJ.length)];
    const hex = Math.floor(Math.random() * 0xfff).toString(16).toUpperCase().padStart(3, "0");
    return a + "_" + hex;
  };
  const loadIdentity = () => {
    let raw = null;
    try { raw = localStorage.getItem(LS_ID); } catch (_) {}
    if (raw) { try { state.identity = JSON.parse(raw); } catch (_) { raw = null; } }
    if (!raw) {
      state.identity = { name: rollName(), seed: Math.random().toString(36).slice(2) };
      saveIdentity();
    }
  };
  const saveIdentity = () => { try { localStorage.setItem(LS_ID, JSON.stringify(state.identity)); } catch (_) {} };

  // Экспорт/импорт идентичности — чисто клиентские, бэкенд не участвует:
  // identity живёт в localStorage как {name, seed}, поэтому export = скачать JSON,
  // import = прочитать JSON и восстановить. Валидируем форму, мусор не принимаем.
  const exportIdentity = () => {
    const blob = new Blob([JSON.stringify(state.identity, null, 2)], { type: "application/json" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = "myaukalo-identity.json";
    document.body.appendChild(a);
    a.click();
    a.remove();
    URL.revokeObjectURL(url);
  };
  const importIdentity = (file) => {
    const reader = new FileReader();
    reader.onload = () => {
      try {
        const obj = JSON.parse(reader.result);
        if (!obj || typeof obj.name !== "string" || typeof obj.seed !== "string") {
          throw new Error("bad identity shape");
        }
        state.identity = { name: obj.name, seed: obj.seed };
        saveIdentity();
        renderIdentityCard();
        renderChatList();   // аватары/alias зависят от identity в cat-режиме
        renderFeed();
      } catch (_) {
        // невалидный файл: молча оставляем текущую идентичность, не крашим UI
      }
    };
    reader.readAsText(file);
  };

  // ---------- генеративный аватар (детерминирован по seed) ----------
  const COATS = ["#e8a87c","#c98b5e","#9c9c9c","#5b5b5b","#f0e6d2","#d98c5f","#7a8b6f"];
  const avatarSVG = (seed) => {
    const h = fnv1a(String(seed));
    const coat = COATS[h % COATS.length];
    const eye = ["#3a2a1a","#274058","#4a3a2a","#1f8a4c"][(h >> 3) % 4];
    const ear = (h >> 6) % 3;
    const whisk = (h >> 9) % 2;
    const earPath = ear === 0
      ? "M8,12 L4,2 L14,8 Z M24,12 L28,2 L18,8 Z"
      : ear === 1
      ? "M7,11 L6,3 L15,8 Z M25,11 L26,3 L17,8 Z"
      : "M9,12 L7,4 L15,9 Z M23,12 L25,4 L17,9 Z";
    const w = whisk
      ? '<path d="M6,18 L1,17 M6,20 L1,21 M26,18 L31,17 M26,20 L31,21" stroke="#00000022" stroke-width="1"/>'
      : "";
    return (
      '<svg viewBox="0 0 32 32" width="22" height="22" role="img" aria-label="avatar">' +
      '<path d="' + earPath + '" fill="' + coat + '"/>' +
      '<circle cx="16" cy="18" r="11" fill="' + coat + '"/>' +
      '<circle cx="12" cy="16" r="1.6" fill="' + eye + '"/>' +
      '<circle cx="20" cy="16" r="1.6" fill="' + eye + '"/>' +
      '<path d="M16,19 l-1.4,1.4 h2.8 z" fill="#7a4a3a"/>' +
      w +
      "</svg>"
    );
  };

  // ---------- темы / cat ----------
  const applyPrefs = () => {
    let t = null, c = null;
    try { t = localStorage.getItem(LS_THEME); c = localStorage.getItem(LS_CAT); } catch (_) {}
    if (!t) t = window.matchMedia && window.matchMedia("(prefers-color-scheme: light)").matches ? "light" : "dark";
    state.theme = t;
    state.cat = c === "on";
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
    renderChatList();
    renderFeed();
    renderIdentityCard();
  };

  // ---------- API ----------
  const api = async (path, opts) => {
    const res = await fetch(path, Object.assign({ headers: { "Content-Type": "application/json" } }, opts || {}));
    const data = await res.json().catch(() => ({}));
    if (!res.ok) throw Object.assign(new Error(data.error || res.statusText), { status: res.status });
    return data;
  };

  // ---------- список чатов ----------
  const loadChats = async () => {
    try {
      const data = await api("/api/v1/chats");
      state.chats = Array.isArray(data.items) ? data.items : (Array.isArray(data) ? data : []);
    } catch (e) { state.chats = []; }
    renderChatList();
  };
  const renderChatList = () => {
    const ul = $("#chatlist");
    ul.textContent = "";
    const shown = state.chats.filter((c) => matches(state.chatQuery, c.title));
    const emptyEl = $("#chatsEmpty");
    emptyEl.style.display = shown.length ? "none" : "block";
    // различаем «вовсе нет комнат» и «есть, но фильтр ничего не поймал»
    emptyEl.textContent = (state.chatQuery.trim() && state.chats.length && !shown.length)
      ? "no rooms match"
      : "no rooms. press + to start";
    for (const chat of shown) {
      const li = el("li", "chatrow" + (chat.id === state.activeChatId ? " active" : ""));
      li.dataset.id = chat.id;

      const av = el("span", "avatar");
      if (state.cat) av.innerHTML = avatarSVG(chat.id);
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
    state.messages = [];
    state.seenIds = new Set();
    // комнатный фильтр привязан к конкретной комнате => сбрасываем при входе
    state.msgQuery = "";
    const ms = $("#msgSearch"); if (ms) ms.value = "";
    document.body.classList.remove("view-list");
    document.body.classList.add("view-chat");
    renderChatList();

    let chat = state.chats.find((c) => c.id === id);
    if (!chat) { try { chat = await api("/api/v1/chats/" + encodeURIComponent(id)); } catch (_) { chat = { id, title: "(room)" }; } }
    $("#roomTitle").textContent = chat.title || "(untitled)";
    $("#roomMeta").textContent = "";

    try {
      const data = await api("/api/v1/chats/" + encodeURIComponent(id) + "/messages");
      state.messages = Array.isArray(data) ? data : (data.items || []);
    } catch (_) { state.messages = []; }
    for (const m of state.messages) state.seenIds.add(m.id);

    renderFeed();
    connectSSE(id);
    api("/api/v1/chats/" + encodeURIComponent(id) + "/read", { method: "POST" }).then(loadChats).catch(() => {});
  };

  const renderFeed = () => {
    const feed = $("#feed");
    feed.textContent = "";
    const shown = state.messages.filter((m) => matches(state.msgQuery, m.text));
    const emptyEl = $("#feedEmpty");
    emptyEl.style.display = shown.length ? "none" : "block";
    emptyEl.textContent = (state.msgQuery.trim() && state.messages.length && !shown.length)
      ? "no messages match"
      : "nothing here yet";
    for (const m of shown) feed.appendChild(buildMsg(m, false));
    scrollFeed();
  };

  // buildMsg: единый DOM для обоих режимов; развилка по стилю — только avatar-slot.
  // TTL-индикатор добавляется, если у сообщения есть expires_at (таймстамп от сервера).
  const buildMsg = (m, animate) => {
    const mine = m.author === state.identity.name;
    const row = el("div", "msg " + (mine ? "me" : "other") + (animate ? " enter" : "") + (m._pending ? " pending" : "") + (m._failed ? " failed" : ""));
    row.dataset.id = m.id;

    row.appendChild(el("span", "ts", fmtTime(m.createdAt)));
    row.appendChild(el("span", "alias", mine ? "you" : m.author));

    const slot = el("span", "avatar-slot");
    if (state.cat) slot.innerHTML = avatarSVG(mine ? state.identity.seed : m.author);
    row.appendChild(slot);

    row.appendChild(el("span", "body", m.text));

    // --- TTL-кольцо: строим только если сервер прислал expires_at ---
    if (m.expiresAt) {
      const createdMs = Date.parse(m.createdAt || "") || 0;
      const expiresMs = Date.parse(m.expiresAt) || 0;
      if (expiresMs > 0) {
        const ttl = el("span", "ttl");
        ttl.dataset.expires = String(expiresMs);
        ttl.dataset.created = String(createdMs);
        ttl.innerHTML =
          '<svg class="ring" viewBox="0 0 16 16">' +
          '<circle class="ring-bg" cx="8" cy="8" r="6"/>' +
          '<circle class="ring-fg" cx="8" cy="8" r="6" stroke-dasharray="' + RING_CIRC.toFixed(2) + '" stroke-dashoffset="0"/>' +
          '</svg><span class="ttl-text"></span>';
        row.appendChild(ttl);
      }
    }
    return row;
  };

  const appendMsg = (m) => {
    if (state.seenIds.has(m.id)) return;     // дедуп optimistic vs SSE по message.id
    state.seenIds.add(m.id);
    state.messages.push(m);                  // храним ВСЕГДА, даже если сейчас скрыт фильтром
    if (m.chatId !== state.activeChatId) return;
    // если активен комнатный фильтр и сообщение не подходит — не рисуем, но в state оно есть;
    // при сбросе/смене фильтра renderFeed перестроит ленту из state.messages и ничего не потеряет
    if (!matches(state.msgQuery, m.text)) return;
    $("#feedEmpty").style.display = "none";
    $("#feed").appendChild(buildMsg(m, true));
    scrollFeed();
    loadChats();
  };

  const scrollFeed = () => { const f = $("#feed"); f.scrollTop = f.scrollHeight; };

  // ---------- TTL-тиккер: ОДИН setInterval на всё приложение ----------
  const tickTTL = () => {
    const now = Date.now();
    const nodes = document.querySelectorAll(".ttl[data-expires]");
    for (const node of nodes) {
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
  };

  // ---------- SSE ----------
  const setConn = (cls, title) => { const c = $("#conn"); c.className = "conn " + cls; c.title = title; };
  const connectSSE = (id) => {
    if (state.es) { state.es.close(); state.es = null; }
    const es = new EventSource("/api/v1/events?chat_id=" + encodeURIComponent(id));
    state.es = es;
    es.onopen = () => setConn("online", "connected");
    es.onerror = () => setConn("reconn", "reconnecting…");
    es.addEventListener("message", (ev) => {
      let m; try { m = JSON.parse(ev.data); } catch (_) { return; }
      appendMsg({
        id: m.id, chatId: m.chat_id, author: m.author, text: m.text,
        createdAt: m.created_at, expiresAt: m.expires_at || null,
      });
    });
  };

  // ---------- отправка (с TTL) ----------
  const sendMessage = async (text) => {
    text = text.trim();
    if (!text || !state.activeChatId) return;
    const tmpId = "tmp-" + Math.random().toString(36).slice(2);
    const optimistic = { id: tmpId, chatId: state.activeChatId, author: state.identity.name, text, createdAt: new Date().toISOString(), expiresAt: null, _pending: true };
    state.seenIds.add(tmpId);
    state.messages.push(optimistic);
    if (matches(state.msgQuery, text)) {
      $("#feedEmpty").style.display = "none";
      const node = buildMsg(optimistic, true);
      $("#feed").appendChild(node);
      scrollFeed();
    }

    const body = { author: state.identity.name, text };
    if (state.ttl) body.ttl = state.ttl;       // duration-строка: "5m","1h",... (контракт routes.go:20)

    try {
      const saved = await api("/api/v1/chats/" + encodeURIComponent(state.activeChatId) + "/messages", {
        method: "POST",
        body: JSON.stringify(body),
      });
      state.seenIds.delete(tmpId);
      state.seenIds.add(saved.id);
      const idx = state.messages.findIndex((x) => x.id === tmpId);
      if (idx >= 0) state.messages[idx] = { id: saved.id, chatId: saved.chat_id, author: saved.author, text: saved.text, createdAt: saved.created_at, expiresAt: saved.expires_at || null };
      const node = document.querySelector('.msg[data-id="' + tmpId.replace(/"/g, '\\"') + '"]');
      if (node) {
        node.dataset.id = saved.id;
        node.classList.remove("pending");
        const oldTtl = node.querySelector(".ttl");
        if (oldTtl) oldTtl.remove();
        if (saved.expires_at) {
          const createdMs = Date.parse(saved.created_at || "") || 0;
          const expiresMs = Date.parse(saved.expires_at) || 0;
          if (expiresMs > 0) {
            const ttl = el("span", "ttl");
            ttl.dataset.expires = String(expiresMs);
            ttl.dataset.created = String(createdMs);
            ttl.innerHTML =
              '<svg class="ring" viewBox="0 0 16 16">' +
              '<circle class="ring-bg" cx="8" cy="8" r="6"/>' +
              '<circle class="ring-fg" cx="8" cy="8" r="6" stroke-dasharray="' + RING_CIRC.toFixed(2) + '" stroke-dashoffset="0"/>' +
              '</svg><span class="ttl-text"></span>';
            node.appendChild(ttl);
          }
        }
      }
    } catch (e) {
      optimistic._failed = true;
      const node = document.querySelector('.msg[data-id="' + tmpId.replace(/"/g, '\\"') + '"]');
      if (node) { node.classList.remove("pending"); node.classList.add("failed"); }
    }
  };

  // ---------- новый чат ----------
  const createChat = async (title) => {
    const chat = await api("/api/v1/chats", { method: "POST", body: JSON.stringify({ title: title || "" }) });
    await loadChats();
    openChat(chat.id);
  };

  // ---------- идентичность (модалка) ----------
  const renderIdentityCard = () => {
    $("#idName").textContent = state.identity.name;
    $("#idAvatar").innerHTML = state.cat ? avatarSVG(state.identity.seed) : "";
  };

  // ---------- wiring ----------
  const bind = () => {
    $("#themeToggle").addEventListener("click", toggleTheme);
    $("#catToggle").addEventListener("click", toggleCat);
    $("#backBtn").addEventListener("click", () => {
      document.body.classList.remove("view-chat");
      document.body.classList.add("view-list");
      if (state.es) { state.es.close(); state.es = null; }
      state.activeChatId = null;
      renderChatList();
    });
    $("#identityBtn").addEventListener("click", () => { renderIdentityCard(); $("#identityDialog").showModal(); });
    $("#idDoneBtn").addEventListener("click", () => $("#identityDialog").close());
    $("#rerollBtn").addEventListener("click", () => { state.identity = { name: rollName(), seed: Math.random().toString(36).slice(2) }; saveIdentity(); renderIdentityCard(); renderFeed(); });

    // экспорт/импорт идентичности
    $("#exportBtn").addEventListener("click", exportIdentity);
    $("#importBtn").addEventListener("click", () => $("#importFile").click());
    $("#importFile").addEventListener("change", (e) => {
      const f = e.target.files && e.target.files[0];
      if (f) importIdentity(f);
      e.target.value = "";   // позволяем повторно выбрать тот же файл
    });

    // фильтры: список комнат (глобальный) и открытая комната (локальный)
    $("#chatSearch").addEventListener("input", (e) => { state.chatQuery = e.target.value; renderChatList(); });
    $("#msgSearch").addEventListener("input", (e) => { state.msgQuery = e.target.value; renderFeed(); });

    // TTL-пикер: клик по чипу фиксирует duration-строку в state.ttl
    $("#ttlpicker").addEventListener("click", (e) => {
      const chip = e.target.closest(".chip");
      if (!chip) return;
      state.ttl = chip.dataset.ttl || "";
      for (const c of $("#ttlpicker").querySelectorAll(".chip")) c.classList.toggle("active", c === chip);
    });

    $("#newChatBtn").addEventListener("click", () => { $("#newChatTitle").value = ""; $("#newChatDialog").showModal(); });
    $("#newChatCancel").addEventListener("click", () => $("#newChatDialog").close());
    $("#newChatForm").addEventListener("submit", (e) => {
      e.preventDefault();
      createChat($("#newChatTitle").value).finally(() => $("#newChatDialog").close());
    });

    $("#composer").addEventListener("submit", (e) => { e.preventDefault(); const v = $("#input").value; $("#input").value = ""; sendMessage(v); });
  };

  // ---------- boot ----------
  const boot = () => {
    loadIdentity();
    applyPrefs();
    bind();
    loadChats();
    setInterval(tickTTL, 1000);   // один тиккер на всё приложение
  };
  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", boot);
  else boot();
})();
