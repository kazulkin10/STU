(() => {
  const apiBase = window.STU_API_BASE || '';
  const wsUrl = decideWsUrl();
  const state = {
    access: null,
    refresh: localStorage.getItem('stu_refresh') || null,
    userId: localStorage.getItem('stu_user_id') || null,
    deviceId: localStorage.getItem('stu_device_id') || null,
    dialogs: [],
    messages: {}, // dialogId -> [{...}]
    meta: {}, // messageId -> {delivered, read}
    currentDialog: null,
    ws: null,
    wsConnected: false,
  };

  const el = (id) => document.getElementById(id);
  const dialogListEl = el('dialogList');
  const dialogEmptyEl = el('dialogEmpty');
  const chatTitleEl = el('chatTitle');
  const chatStatusEl = el('chatStatus');
  const messagesEl = el('messages');
  const wsStatusEl = el('wsStatus');
  const sidebarStatusEl = el('sidebarStatus');
  const authPanel = el('authPanel');
  const appPanel = el('appPanel');
  const userBadge = el('userBadge');
  const btnLogout = el('btnLogout');
  const authError = el('authError');
  const newDialogForm = el('newDialogForm');
  let wsBackoff = 500;

  // Tabs
  document.querySelectorAll('.tab').forEach((btn) => {
    btn.addEventListener('click', () => switchTab(btn.dataset.tab));
  });
  function switchTab(name) {
    document.querySelectorAll('.tab').forEach((b) => b.classList.toggle('active', b.dataset.tab === name));
    document.querySelectorAll('.tab-content').forEach((c) => c.classList.toggle('hidden', c.dataset.tab !== name));
    authError.classList.add('hidden');
  }

  // Auth actions
  el('regSubmit').onclick = async () => {
    try {
      const email = el('regEmail').value.trim();
      const password = el('regPassword').value;
      if (!email || !password) return showAuthError('Введите email и пароль');
      await apiFetch('/v1/auth/register', {
        method: 'POST',
        body: JSON.stringify({ email, password }),
      });
      switchTab('verify');
      el('verEmail').value = email;
      showToast('Код отправлен. Откройте Mailpit в dev.');
    } catch (e) {
      showAuthError(e.message || 'Ошибка регистрации');
    }
  };

  el('verSubmit').onclick = async () => {
    try {
      const email = el('verEmail').value.trim();
      const code = el('verCode').value.trim();
      const device_name = el('verDevice').value.trim() || 'web';
      if (!email || !code) return showAuthError('Введите email и код');
      const res = await apiFetch('/v1/auth/verify', {
        method: 'POST',
        body: JSON.stringify({ email, code, device_name, platform: 'web' }),
      }, false);
      onAuthSuccess(res);
    } catch (e) {
      showAuthError(e.message || 'Ошибка подтверждения');
    }
  };

  el('loginSubmit').onclick = async () => {
    try {
      const email = el('loginEmail').value.trim();
      const password = el('loginPassword').value;
      if (!email || !password) return showAuthError('Введите email и пароль');
      const res = await apiFetch('/v1/auth/login', {
        method: 'POST',
        body: JSON.stringify({ email, password, device_name: 'web', platform: 'web' }),
      }, false);
      onAuthSuccess(res);
    } catch (e) {
      showAuthError(e.message || 'Ошибка входа');
    }
  };

  function onAuthSuccess(res) {
    state.access = res.access_token;
    state.refresh = res.refresh_token;
    state.userId = res.user_id;
    state.deviceId = res.device_id;
    localStorage.setItem('stu_refresh', state.refresh);
    localStorage.setItem('stu_user_id', state.userId);
    localStorage.setItem('stu_device_id', state.deviceId || '');
    authPanel.classList.add('hidden');
    appPanel.classList.remove('hidden');
    userBadge.classList.remove('hidden');
    userBadge.textContent = `ID: ${state.userId}`;
    btnLogout.classList.remove('hidden');
    loadDialogs();
    connectWs();
  }

  function showAuthError(msg) {
    authError.textContent = msg;
    authError.classList.remove('hidden');
  }

  // API helper with refresh
  async function apiFetch(path, options = {}, allowRetry = true) {
    const headers = options.headers || {};
    headers['Content-Type'] = headers['Content-Type'] || 'application/json';
    if (state.access) headers['Authorization'] = `Bearer ${state.access}`;
    const res = await fetch(apiBase + path, { ...options, headers });
    if (res.status === 401 && state.refresh && allowRetry) {
      const ok = await tryRefresh();
      if (ok) return apiFetch(path, options, false);
      throw new Error('Не авторизовано');
    }
    if (!res.ok) {
      const text = await res.text();
      if (res.status === 404) {
        console.error('API 404', { path, status: res.status, body: text });
        throw new Error(`Ошибка API (${res.status}): ${path} не найден. Проверь gateway /v1/auth/*.`);
      }
      throw new Error(text || `Ошибка запроса (${res.status})`);
    }
    if (res.status === 204) return {};
    return res.json();
  }

  async function tryRefresh() {
    try {
      const res = await fetch(apiBase + '/v1/auth/refresh', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ refresh_token: state.refresh }),
      });
      if (!res.ok) {
        clearSession();
        return false;
      }
      const data = await res.json();
      state.access = data.access_token;
      state.refresh = data.refresh_token;
      localStorage.setItem('stu_refresh', state.refresh);
      return true;
    } catch (e) {
      clearSession();
      return false;
    }
  }

  function clearSession() {
    state.access = null;
    state.refresh = null;
    localStorage.removeItem('stu_refresh');
    localStorage.removeItem('stu_user_id');
    localStorage.removeItem('stu_device_id');
    location.reload();
  }

  btnLogout.onclick = async () => {
    try {
      if (state.refresh) {
        await fetch(apiBase + '/v1/auth/logout', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ refresh_token: state.refresh }),
        });
      }
    } catch (_) {}
    clearSession();
  };

  // Dialogs
  async function loadDialogs() {
    sidebarStatusEl.textContent = 'Загрузка...';
    try {
      const dialogs = await apiFetch('/v1/dialogs');
      state.dialogs = dialogs;
      renderDialogs();
      sidebarStatusEl.textContent = '';
      if (!dialogs.length) dialogEmptyEl.classList.remove('hidden');
      else dialogEmptyEl.classList.add('hidden');
    } catch (e) {
      sidebarStatusEl.textContent = 'Ошибка загрузки';
      showToast(e.message, true);
    }
  }

  function renderDialogs() {
    dialogListEl.innerHTML = '';
    state.dialogs.forEach((d) => {
      const item = document.createElement('div');
      item.className = 'dialog' + (state.currentDialog === d.id ? ' active' : '');
      item.onclick = () => openDialog(d.id, d.title || 'Диалог');
      const preview = d.last_message ? d.last_message.text : 'Нет сообщений';
      const time = d.last_message ? new Date(d.last_message.created_at).toLocaleTimeString() : '';
      item.innerHTML = `
        <div class="title">${d.title || 'Без имени'}</div>
        <div class="preview"><span>${preview}</span><span>${time}</span></div>
      `;
      dialogListEl.appendChild(item);
    });
  }

  // Messages
  async function openDialog(id, title) {
    state.currentDialog = id;
    chatTitleEl.textContent = title;
    chatStatusEl.textContent = 'Загрузка...';
    renderDialogs();
    try {
      const msgs = await apiFetch(`/v1/dialogs/${id}/messages?limit=50`);
      state.messages[id] = msgs.reverse(); // oldest first
      msgs.forEach((m) => {
        state.meta[m.id] = {
          delivered: m.delivered_by_peer,
          read: m.read_by_peer,
          delivered_to_me: m.delivered_to_me,
          read_by_me: m.read_by_me,
        };
      });
      renderMessages();
      chatStatusEl.textContent = '';
      scrollMessagesBottom();
      await markDeliveredAndRead();
    } catch (e) {
      chatStatusEl.textContent = 'Ошибка загрузки';
      showToast(e.message, true);
    }
  }

  function renderMessages() {
    const msgs = state.messages[state.currentDialog] || [];
    messagesEl.innerHTML = '';
    msgs.forEach((m) => {
      const bubble = document.createElement('div');
      const mine = m.sender_id === state.userId;
      bubble.className = 'bubble' + (mine ? ' me' : '');
      const meta = state.meta[m.id] || {};
      let ticks = '';
      if (mine) {
        if (meta.failed) ticks = `<span class="tick danger">не отправлено</span>`;
        else if (meta.pending) ticks = `<span class="tick">…</span>`;
        else ticks = `<span class="tick ${meta.read ? 'read' : ''}">${meta.read ? '✓✓' : meta.delivered ? '✓' : '·'}</span>`;
      }
      bubble.innerHTML = `
        <div>${escapeHtml(m.text)}</div>
        <div class="meta">
          <span>${new Date(m.created_at).toLocaleTimeString()}</span>
          ${ticks}
        </div>
      `;
      if (meta.failed) {
        const retry = document.createElement('button');
        retry.textContent = 'Повторить';
        retry.className = 'ghost small';
        retry.onclick = () => retrySend(m);
        bubble.appendChild(retry);
      }
      messagesEl.appendChild(bubble);
    });
  }

  function scrollMessagesBottom() {
    messagesEl.scrollTop = messagesEl.scrollHeight;
  }

  el('sendMessage').onclick = sendMessage;
  el('messageInput').addEventListener('keydown', (e) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      sendMessage();
    }
  });

  async function sendMessage() {
    if (!state.currentDialog) return;
    const text = el('messageInput').value.trim();
    if (!text) return;
    el('messageInput').value = '';
    const tempId = Date.now();
    const pendingMsg = {
      id: tempId,
      sender_id: state.userId,
      dialog_id: state.currentDialog,
      text,
      created_at: new Date().toISOString(),
    };
    state.messages[state.currentDialog] = state.messages[state.currentDialog] || [];
    state.messages[state.currentDialog].push(pendingMsg);
    state.meta[tempId] = { pending: true };
    renderMessages();
    scrollMessagesBottom();
    await sendOrFail(pendingMsg);
  }

  async function retrySend(msg) {
    await sendOrFail(msg);
  }

  async function sendOrFail(msg) {
    try {
      const sent = await apiFetch(`/v1/dialogs/${msg.dialog_id}/messages`, {
        method: 'POST',
        body: JSON.stringify({ text: msg.text }),
      });
      state.messages[msg.dialog_id] = state.messages[msg.dialog_id].map((m) => (m.id === msg.id ? sent : m));
      state.meta[sent.id] = { delivered: sent.delivered_by_peer, read: sent.read_by_peer };
      delete state.meta[msg.id];
      renderMessages();
      scrollMessagesBottom();
    } catch (e) {
      state.meta[msg.id] = { failed: true };
      renderMessages();
      showToast(e.message || 'Не отправлено', true);
    }
  }

  async function markDeliveredAndRead() {
    const msgs = state.messages[state.currentDialog] || [];
    const others = msgs.filter((m) => m.sender_id !== state.userId);
    if (!others.length) return;
    try {
      await Promise.all(
        others.map((m) =>
          apiFetch(`/v1/dialogs/${state.currentDialog}/messages/${m.id}/delivered`, { method: 'POST' })
        )
      );
      await Promise.all(
        others.map((m) => apiFetch(`/v1/dialogs/${state.currentDialog}/messages/${m.id}/read`, { method: 'POST' }))
      );
      others.forEach((m) => {
        state.meta[m.id] = { ...(state.meta[m.id] || {}), delivered: true, read: true };
      });
      renderMessages();
    } catch (e) {
      console.warn('mark delivered/read failed', e);
    }
  }

  // New dialog form
  el('btnNewDialog').onclick = () => newDialogForm.classList.toggle('hidden');
  el('cancelDialog').onclick = () => newDialogForm.classList.add('hidden');
  el('createDialogSubmit').onclick = async () => {
    const email = el('newDialogEmail').value.trim();
    if (!email) return;
    try {
      const res = await apiFetch('/v1/dialogs', {
        method: 'POST',
        body: JSON.stringify({ email }),
      });
      newDialogForm.classList.add('hidden');
      el('newDialogEmail').value = '';
      await loadDialogs();
      openDialog(res.dialog_id, 'Новый диалог');
    } catch (e) {
      showToast(e.message, true);
    }
  };

  // WS handling
  function decideWsUrl() {
    if (window.STU_WS_URL) return window.STU_WS_URL;
    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
    return `${proto}//${location.host}/v1/ws`;
  }

  // Browser WS cannot set header; adjust URL to include token
  async function connectWs() {
    if (!state.access && !(await tryRefresh())) return;
    const url = new URL(wsUrl);
    url.searchParams.set('token', state.access);
    const ws = new WebSocket(url.toString(), []);
    state.ws = ws;
    wsStatusEl.classList.remove('ok');
    wsStatusEl.title = 'Подключение...';
    ws.onopen = () => {
      state.wsConnected = true;
      wsBackoff = 500;
      wsStatusEl.classList.add('ok');
      wsStatusEl.title = 'Онлайн';
    };
    ws.onclose = async () => {
      state.wsConnected = false;
      wsStatusEl.classList.remove('ok');
      wsStatusEl.title = 'Переподключение...';
      if (state.refresh) {
        await tryRefresh();
        setTimeout(connectWs, wsBackoff);
        wsBackoff = Math.min(wsBackoff * 2, 10000);
      }
    };
    ws.onerror = () => { wsStatusEl.classList.remove('ok'); };
    ws.onmessage = (ev) => handleEvent(ev.data);
  }

  function handleEvent(data) {
    let evt;
    try { evt = JSON.parse(data); } catch { return; }
    if (evt.type === 'message.new') {
      const d = evt.dialog_id;
      state.messages[d] = state.messages[d] || [];
      const msgObj = {
        id: evt.message_id,
        dialog_id: d,
        sender_id: evt.sender_id,
        text: evt.text,
        created_at: evt.created_at,
      };
      state.messages[d].push(msgObj);
      state.meta[evt.message_id] = { delivered: false, read: false };
      if (state.currentDialog === d) {
        renderMessages();
        scrollMessagesBottom();
        markDeliveredAndRead();
      }
      loadDialogs();
    }
    if (evt.type === 'message.delivered' || evt.type === 'message.read') {
      const meta = state.meta[evt.message_id] || {};
      if (evt.type === 'message.delivered') meta.delivered = true;
      if (evt.type === 'message.read') { meta.delivered = true; meta.read = true; }
      state.meta[evt.message_id] = meta;
      if (state.currentDialog === evt.dialog_id) renderMessages();
    }
  }

  function showToast(msg, danger = false) {
    const div = document.createElement('div');
    div.className = 'alert-inline';
    div.style.position = 'fixed';
    div.style.right = '12px';
    div.style.bottom = '12px';
    div.style.maxWidth = '320px';
    if (!danger) { div.style.background = 'rgba(109,168,255,0.12)'; div.style.border = '1px solid rgba(109,168,255,0.3)'; div.style.color = '#d7e6ff'; }
    div.textContent = msg;
    document.body.appendChild(div);
    setTimeout(() => div.remove(), 4000);
  }

  function escapeHtml(str) {
    return str.replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
  }

  // Auto login via refresh if exists
  async function bootstrap() {
    if (state.refresh) {
      const ok = await tryRefresh();
      if (ok) {
        authPanel.classList.add('hidden');
        appPanel.classList.remove('hidden');
        userBadge.classList.remove('hidden');
        userBadge.textContent = `ID: ${state.userId || ''}`;
        btnLogout.classList.remove('hidden');
        await loadDialogs();
        connectWs();
        return;
      }
    }
    authPanel.classList.remove('hidden');
  }

  bootstrap();
})();
