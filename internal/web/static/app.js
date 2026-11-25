// gitbay progressive enhancement: popovers, Cmd+K palette, copy buttons,
// in-place swaps for reactions/toggles/pickers, inline diff comments, and
// keyboard shortcuts. Every action degrades cleanly without JS.
(() => {
  'use strict';
  document.documentElement.classList.add('js');

  let toastTimer = 0;
  function showToast(msg, isErr = false) {
    let el = document.getElementById('gb-toast');
    if (!el) {
      el = document.createElement('div');
      el.id = 'gb-toast';
      el.className = 'toast';
      el.setAttribute('role', 'status');
      document.body.appendChild(el);
    }
    el.textContent = msg;
    el.classList.toggle('toast-err', isErr);
    el.classList.add('toast-show');
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => el.classList.remove('toast-show'), 2200);
  }

  const isTyping = (el) => {
    if (!el) return false;
    const tag = el.tagName;
    return tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || el.isContentEditable;
  };

  // ---- 1. Popover hygiene (close others, click outside, Escape) ----
  const popovers = () => document.querySelectorAll('details.popover, details.railpins, details.refmenu');
  document.addEventListener('toggle', (e) => {
    const d = e.target;
    if (!(d instanceof HTMLDetailsElement) || !d.open) return;
    if (!d.matches('details.popover, details.railpins, details.refmenu')) return;
    for (const other of popovers()) {
      if (other !== d && other.open) other.open = false;
    }
    const input = d.querySelector('input[type="text"], input[type="search"]');
    if (input) setTimeout(() => input.focus(), 0);
  }, true);

  document.addEventListener('click', (e) => {
    for (const d of popovers()) {
      if (d.open && !d.contains(e.target)) d.open = false;
    }
  });

  // ---- 2. Copy buttons on clone & copyable blocks ----
  function initCopyButtons(root = document) {
    for (const pre of root.querySelectorAll('.clone pre, pre.copyable')) {
      if (pre.dataset.copyWired) continue;
      pre.dataset.copyWired = '1';
      const code = pre.querySelector('code') || pre;
      const wrap = document.createElement('div');
      wrap.className = 'copywrap';
      pre.parentNode.insertBefore(wrap, pre);
      wrap.appendChild(pre);
      const btn = document.createElement('button');
      btn.type = 'button';
      btn.className = 'copybtn';
      btn.textContent = 'Copy';
      btn.setAttribute('aria-label', 'Copy command');
      btn.addEventListener('click', async () => {
        try {
          await navigator.clipboard.writeText(code.textContent.trim());
          btn.textContent = 'Copied';
          setTimeout(() => { btn.textContent = 'Copy'; }, 1400);
        } catch {
          btn.textContent = 'Failed';
          setTimeout(() => { btn.textContent = 'Copy'; }, 1400);
        }
      });
      wrap.appendChild(btn);
    }
  }

  // ---- 3. In-place fragment swaps (reactions, header toggles, sidebar pickers) ----
  document.addEventListener('submit', async (e) => {
    const form = e.target;
    if (!(form instanceof HTMLFormElement) || form.method.toLowerCase() !== 'post') return;
    let swapSel = null;
    if (form.classList.contains('reactions')) {
      swapSel = 'main#content';
    } else if (form.closest('.identity')) {
      swapSel = '.identity';
    } else if (form.closest('.aside .popover')) {
      swapSel = '.aside';
    } else {
      return;
    }
    e.preventDefault();
    const data = new FormData(form);
    if (e.submitter && e.submitter.name) {
      data.append(e.submitter.name, e.submitter.value);
    }
    const body = new URLSearchParams(data);
    try {
      const res = await fetch(form.action, {
        method: 'POST',
        body,
        headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
        credentials: 'same-origin',
      });
      if (!res.ok) { form.submit(); return; }
      const html = await res.text();
      const doc = new DOMParser().parseFromString(html, 'text/html');
      const next = doc.querySelector(swapSel);
      const cur = document.querySelector(swapSel);
      if (next && cur) {
        cur.replaceWith(next);
        initCopyButtons(next);
      } else {
        location.reload();
      }
    } catch {
      form.submit();
    }
  });

  // ---- 4. Inline diff-line comment composer without page reload ----
  document.addEventListener('click', (e) => {
    const a = e.target.closest('a.cmt');
    if (!a) return;
    const url = new URL(a.href, location.origin);
    const cpath = url.searchParams.get('cpath');
    const cline = url.searchParams.get('cline');
    const cside = url.searchParams.get('cside') || 'new';
    if (!cpath || !cline) return;
    const row = a.closest('tr');
    if (!row) return;
    e.preventDefault();

    const existing = document.getElementById('inline-compose-row');
    if (existing) existing.remove();

    const base = url.pathname;
    const cols = row.closest('table')?.classList.contains('split') ? 4 : 3;
    const tr = document.createElement('tr');
    tr.id = 'inline-compose-row';
    tr.className = 'threadrow';
    const td = document.createElement('td');
    td.colSpan = cols;

    const form = document.createElement('form');
    form.method = 'post';
    form.action = base + '/diff-comment';
    form.className = 'thread composing';

    const hidden = (name, val) => {
      const inp = document.createElement('input');
      inp.type = 'hidden';
      inp.name = name;
      inp.value = val;
      return inp;
    };
    form.append(hidden('path', cpath), hidden('line', cline), hidden('side', cside));

    if (cside !== 'old') {
      const pRange = document.createElement('p');
      pRange.className = 'hint';
      pRange.textContent = `Line ${cline} — use \`\`\`suggestion to propose replacement lines`;
      form.appendChild(pRange);
    }

    const pArea = document.createElement('p');
    const ta = document.createElement('textarea');
    ta.name = 'body';
    ta.rows = 3;
    ta.placeholder = `Comment on ${cpath}:${cline}`;
    ta.required = true;
    pArea.appendChild(ta);

    const pAct = document.createElement('p');
    pAct.className = 'btngroup';
    const bNow = document.createElement('button');
    bNow.type = 'submit';
    bNow.textContent = 'Comment';
    const bPend = document.createElement('button');
    bPend.type = 'submit';
    bPend.name = 'pending';
    bPend.value = 'on';
    bPend.className = 'btn';
    bPend.textContent = 'Add to review';
    const bCancel = document.createElement('button');
    bCancel.type = 'button';
    bCancel.className = 'linklike';
    bCancel.textContent = 'Cancel';
    bCancel.addEventListener('click', () => tr.remove());
    pAct.append(bNow, bPend, bCancel);

    form.append(pArea, pAct);
    td.appendChild(form);
    tr.appendChild(td);
    row.insertAdjacentElement('afterend', tr);
    ta.focus();
  });

  // ---- 5. Cmd+K Command Palette ----
  function collectCommands() {
    const items = [];
    const seen = new Set();
    const add = (group, label, href) => {
      if (!href || seen.has(href)) return;
      seen.add(href);
      items.push({ group, label, href });
    };
    for (const a of document.querySelectorAll('nav.tabs a')) {
      add('Repository', a.textContent.trim().replace(/\s+/g, ' '), a.getAttribute('href'));
    }
    for (const a of document.querySelectorAll('.railpins .raildrop a')) {
      add('Pinned', a.textContent.trim(), a.getAttribute('href'));
    }
    add('Navigate', 'Dashboard', '/');
    add('Navigate', 'Explore repositories', '/explore');
    add('Navigate', 'Global search', '/search');
    add('Navigate', 'Notifications', '/notifications');
    add('Navigate', 'New repository', '/new');
    add('Navigate', 'Account settings', '/settings');
    return items;
  }

  function openPalette() {
    const dlg = document.getElementById('cmdk');
    if (!(dlg instanceof HTMLDialogElement)) return;
    const input = dlg.querySelector('input');
    const list = dlg.querySelector('ul');
    const all = collectCommands();
    let active = 0;

    const render = () => {
      const q = input.value.trim().toLowerCase();
      const filtered = q ? all.filter((it) => (it.label + ' ' + it.group).toLowerCase().includes(q)) : all;
      active = Math.min(active, Math.max(0, filtered.length - 1));
      list.textContent = '';
      if (filtered.length === 0) {
        const li = document.createElement('li');
        li.className = 'cmdk-empty';
        li.textContent = 'No matching destinations';
        list.appendChild(li);
        return;
      }
      filtered.forEach((it, idx) => {
        const li = document.createElement('li');
        const a = document.createElement('a');
        a.href = it.href;
        if (idx === active) a.setAttribute('aria-selected', 'true');
        const g = document.createElement('span');
        g.className = 'cmdk-grp';
        g.textContent = it.group;
        const t = document.createElement('span');
        t.textContent = it.label;
        a.append(g, t);
        li.appendChild(a);
        list.appendChild(li);
      });
    };

    input.value = '';
    render();
    if (!dlg.open) dlg.showModal();
    input.focus();

    input.oninput = () => { active = 0; render(); };
    input.onkeydown = (ev) => {
      const links = list.querySelectorAll('a');
      if (ev.key === 'ArrowDown') {
        ev.preventDefault();
        if (links.length) { active = (active + 1) % links.length; render(); }
      } else if (ev.key === 'ArrowUp') {
        ev.preventDefault();
        if (links.length) { active = (active - 1 + links.length) % links.length; render(); }
      } else if (ev.key === 'Enter') {
        const sel = links[active];
        if (sel) { ev.preventDefault(); location.href = sel.href; }
      }
    };
  }

  document.addEventListener('click', (e) => {
    if (e.target.closest('.cmdk-trigger')) {
      e.preventDefault();
      openPalette();
      return;
    }
    const dlg = document.getElementById('cmdk');
    if (dlg && e.target === dlg) dlg.close();
  });

  // ---- 6. Keyboard shortcuts ----
  let chord = '';
  let chordTimer = 0;
  document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') {
      for (const d of popovers()) if (d.open) d.open = false;
      const inline = document.getElementById('inline-compose-row');
      if (inline) inline.remove();
      return;
    }
    if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
      e.preventDefault();
      openPalette();
      return;
    }
    if (isTyping(e.target) || e.metaKey || e.ctrlKey || e.altKey) return;

    if (e.key === '/') {
      const search = document.querySelector('main input[type="search"], .railsearch input[type="search"]');
      if (search) { e.preventDefault(); search.focus(); return; }
      e.preventDefault();
      openPalette();
      return;
    }

    if (chord === 'g') {
      clearTimeout(chordTimer);
      chord = '';
      const tabLink = (match) => {
        for (const a of document.querySelectorAll('nav.tabs a')) {
          if (a.textContent.trim().toLowerCase().startsWith(match)) return a.href;
        }
        return null;
      };
      const dest = {
        d: '/',
        e: '/explore',
        n: '/notifications',
        c: tabLink('code'),
        i: tabLink('issues'),
        m: tabLink('merge'),
        b: tabLink('builds'),
        r: tabLink('releases'),
        w: tabLink('wiki'),
      }[e.key.toLowerCase()];
      if (dest) { e.preventDefault(); location.href = dest; }
      return;
    }
    if (e.key === 'g') {
      chord = 'g';
      chordTimer = setTimeout(() => { chord = ''; }, 900);
    }
  });

  // ---- 7. Auto-save checkboxes & selects on /settings ----
  document.addEventListener('change', async (e) => {
    const form = e.target.closest('form[data-autosave]');
    if (!(form instanceof HTMLFormElement)) return;
    if (e.target.matches('[data-live-theme]')) {
      const val = e.target.value;
      if (val === 'system') delete document.documentElement.dataset.theme;
      else document.documentElement.dataset.theme = val;
    }
    const body = new URLSearchParams(new FormData(form));
    try {
      const res = await fetch(form.action, {
        method: 'POST',
        body,
        headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
        credentials: 'same-origin',
      });
      if (!res.ok) { showToast('Save failed', true); return; }
      showToast('Saved');
    } catch {
      showToast('Save failed', true);
    }
  });

  // ---- 8. Live client-side table & list filters ----
  document.addEventListener('input', (e) => {
    const inp = e.target.closest('input[data-filter-table], input[data-filter-list]');
    if (!inp) return;
    const q = inp.value.trim().toLowerCase();
    if (inp.dataset.filterTable) {
      const table = document.getElementById(inp.dataset.filterTable);
      if (!table) return;
      for (const row of table.querySelectorAll('tbody tr')) {
        row.hidden = q !== '' && !row.textContent.toLowerCase().includes(q);
      }
      return;
    }
    const list = document.getElementById(inp.dataset.filterList);
    if (!list) return;
    const ul = list.tagName === 'UL' ? list : list.querySelector('ul');
    if (!ul) return;
    for (const item of ul.querySelectorAll(':scope > li')) {
      if (item.classList.contains('empty')) continue;
      item.hidden = q !== '' && !item.textContent.toLowerCase().includes(q);
    }
  });

  // ---- 9. In-place Refresh (/admin, /) ----
  document.addEventListener('click', async (e) => {
    const btn = e.target.closest('button[data-refresh]');
    if (!btn) return;
    const url = btn.dataset.refresh;
    const targetId = btn.dataset.refreshTarget || 'admin-queues';
    btn.disabled = true;
    try {
      const res = await fetch(url, { credentials: 'same-origin' });
      if (!res.ok) { showToast('Refresh failed', true); return; }
      const html = await res.text();
      const doc = new DOMParser().parseFromString(html, 'text/html');
      const next = doc.getElementById(targetId);
      const cur = document.getElementById(targetId);
      if (next && cur) {
        cur.replaceWith(next);
        initScrollSpy();
        showToast('Refreshed');
      }
    } catch {
      showToast('Refresh failed', true);
    } finally {
      btn.disabled = false;
    }
  });

  // ---- 9b. Smooth in-place tab switching (profile tabs) ----
  document.addEventListener('click', async (e) => {
    if (e.metaKey || e.ctrlKey || e.shiftKey || e.altKey || e.button !== 0) return;
    const a = e.target.closest('nav[data-tab-swap] a');
    if (!a) return;
    const targetId = a.closest('nav[data-tab-swap]').dataset.tabSwap;
    const cur = document.getElementById(targetId);
    if (!cur) return;
    e.preventDefault();
    try {
      const res = await fetch(a.href, { credentials: 'same-origin' });
      if (!res.ok) { location.href = a.href; return; }
      const html = await res.text();
      const doc = new DOMParser().parseFromString(html, 'text/html');
      const next = doc.getElementById(targetId);
      if (!next) { location.href = a.href; return; }
      cur.replaceWith(next);
      history.pushState({ tabSwap: targetId }, '', a.href);
      initCopyButtons(next);
    } catch {
      location.href = a.href;
    }
  });
  window.addEventListener('popstate', (e) => {
    if (e.state && e.state.tabSwap) location.reload();
  });

  // ---- 10. Scroll-spy on sticky section navs ----
  function initScrollSpy() {
    const nav = document.querySelector('nav[data-scrollspy]');
    if (!nav || !('IntersectionObserver' in window)) return;
    const links = Array.from(nav.querySelectorAll('a[href^="#"]'));
    const byId = new Map();
    for (const a of links) {
      const id = a.getAttribute('href').slice(1);
      if (id) byId.set(id, a);
    }
    const io = new IntersectionObserver((entries) => {
      for (const ent of entries) {
        if (!ent.isIntersecting) continue;
        for (const a of links) a.removeAttribute('aria-current');
        const hit = byId.get(ent.target.id);
        if (hit) hit.setAttribute('aria-current', 'location');
      }
    }, { rootMargin: '-15% 0px -70% 0px' });
    for (const id of byId.keys()) {
      const sec = document.getElementById(id);
      if (sec) io.observe(sec);
    }
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', () => { initCopyButtons(); initScrollSpy(); });
  } else {
    initCopyButtons();
    initScrollSpy();
  }
})();
