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
  const popovers = () => document.querySelectorAll('details.popover, details.railpins, details.railcreate, details.railusermenu, details.refmenu');
  document.addEventListener('toggle', (e) => {
    const d = e.target;
    if (!(d instanceof HTMLDetailsElement) || !d.open) return;
    if (!d.matches('details.popover, details.railpins, details.railcreate, details.railusermenu, details.refmenu')) return;
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

  // ---- 2. Copy buttons on clone, copyable & rendered code blocks ----
  function initCopyButtons(root = document) {
    for (const pre of root.querySelectorAll('.clone pre, pre.copyable, .rendered pre')) {
      if (pre.dataset.copyWired) continue;
      pre.dataset.copyWired = '1';
      const code = pre.querySelector('code') || pre;
      const rawText = code.textContent.trim();
      const wrap = document.createElement('div');
      wrap.className = rawText.includes('\n') ? 'copywrap' : 'copywrap oneline';
      pre.parentNode.insertBefore(wrap, pre);
      wrap.appendChild(pre);
      const btn = document.createElement('button');
      btn.type = 'button';
      btn.className = 'copybtn';
      btn.textContent = 'Copy';
      btn.setAttribute('aria-label', 'Copy code');
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

  document.addEventListener('click', async (e) => {
    const urlBtn = e.target.closest('button[data-copy-url]');
    if (urlBtn) {
      const orig = urlBtn.textContent;
      try {
        const res = await fetch(urlBtn.dataset.copyUrl, { credentials: 'same-origin' });
        if (!res.ok) throw new Error('fetch failed');
        await navigator.clipboard.writeText(await res.text());
        urlBtn.textContent = 'copied';
        showToast('File copied to clipboard');
        setTimeout(() => { urlBtn.textContent = orig; }, 1400);
      } catch {
        showToast('Copy failed', true);
      }
      return;
    }
    const quoteBtn = e.target.closest('button[data-quote-from]');
    if (quoteBtn) {
      const art = document.getElementById(quoteBtn.dataset.quoteFrom);
      const ta = document.getElementById('main-comment-box') || document.querySelector('form.commentform textarea[name="body"]');
      if (art && ta) {
        const rendered = art.querySelector('.rendered');
        const text = (rendered ? rendered.innerText : '').trim();
        if (text) {
          const quoted = text.split('\n').map((l) => '> ' + l).join('\n') + '\n\n';
          ta.value = (ta.value ? ta.value.replace(/\s*$/, '\n\n') : '') + quoted;
          ta.focus();
          ta.setSelectionRange(ta.value.length, ta.value.length);
          ta.scrollIntoView({ behavior: 'smooth', block: 'center' });
        }
      }
      return;
    }
    const btn = e.target.closest('button[data-copy]');
    if (!btn) return;
    const orig = btn.textContent;
    try {
      await navigator.clipboard.writeText(btn.dataset.copy);
      btn.textContent = 'Copied';
      showToast('Copied to clipboard');
      setTimeout(() => { btn.textContent = orig; }, 1400);
    } catch {
      showToast('Copy failed', true);
    }
  });
  document.addEventListener('submit', async (e) => {
    const form = e.target;
    if (!(form instanceof HTMLFormElement) || form.method.toLowerCase() !== 'post') return;

    // No-reload markup preview on any form with a Preview button
    if (e.submitter && e.submitter.name === 'preview') {
      e.preventDefault();
      const btn = e.submitter;
      const data = new FormData(form);
      data.append('preview', btn.value || '1');
      btn.disabled = true;
      try {
        const res = await fetch(form.action, {
          method: 'POST',
          body: new URLSearchParams(data),
          headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
          credentials: 'same-origin',
        });
        if (!res.ok) { form.submit(); return; }
        const doc = new DOMParser().parseFromString(await res.text(), 'text/html');
        const nextPrev = doc.querySelector('.preview');
        if (!nextPrev) { showToast('Nothing to preview'); return; }
        const prev = form.previousElementSibling;
        if (prev && prev.classList.contains('preview')) prev.replaceWith(nextPrev);
        else form.insertAdjacentElement('beforebegin', nextPrev);
        initCopyButtons(nextPrev);
      } catch {
        form.submit();
      } finally {
        btn.disabled = false;
      }
      return;
    }

    let swapSel = null;
    let keepPopoverIdx = -1;
    let foldId = '';
    if (form.classList.contains('reactions') && form.dataset.reactSlot !== undefined) {
      swapSel = `form.reactions[data-react-slot="${CSS.escape(form.dataset.reactSlot)}"]`;
    } else if (form.closest('.identity')) {
      swapSel = '.identity';
    } else if (form.closest('.aside .popover')) {
      swapSel = '.aside';
      const openPop = form.closest('.aside .popover');
      const allPops = Array.from(document.querySelectorAll('.aside .popover'));
      keepPopoverIdx = allPops.indexOf(openPop);
    } else if (form.dataset.notifForm) {
      swapSel = '#notif-root';
    } else if (form.closest('details.difffold') && (form.classList.contains('composing') || form.closest('details.threadreply') || form.classList.contains('threadact'))) {
      const fold = form.closest('details.difffold');
      if (fold && fold.id) {
        foldId = fold.id;
        swapSel = '#' + CSS.escape(fold.id);
      }
    }
    if (!swapSel) return;
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
      const errBanner = doc.querySelector('main .error');
      if (errBanner) {
        showToast(errBanner.textContent.trim(), true);
        return;
      }
      const next = doc.querySelector(swapSel);
      const cur = document.querySelector(swapSel);
      if (next && cur) {
        if (foldId) next.open = true;
        cur.replaceWith(next);
        initCopyButtons(next);
        if (swapSel === '.identity') {
          const nextPins = doc.querySelector('.railpins');
          const curPins = document.querySelector('.railpins');
          if (nextPins && curPins) curPins.replaceWith(nextPins);
        } else if (swapSel === '.aside' && keepPopoverIdx >= 0) {
          const newPops = next.querySelectorAll('.popover');
          if (newPops[keepPopoverIdx]) newPops[keepPopoverIdx].open = true;
        } else if (foldId) {
          const nextAside = doc.querySelector('.aside');
          const curAside = document.querySelector('.aside');
          if (nextAside && curAside) curAside.replaceWith(nextAside);
        } else if (swapSel === '#notif-root') {
          const nextBell = doc.querySelector('.rail a[href="/notifications"]');
          const curBell = document.querySelector('.rail a[href="/notifications"]');
          if (nextBell && curBell) curBell.replaceWith(nextBell);
          showToast('Marked as read');
        }
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
    const isSplit = row.closest('table')?.classList.contains('split');
    const cols = isSplit ? 4 : 3;
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

    const pArea = document.createElement('p');
    const ta = document.createElement('textarea');
    ta.name = 'body';
    ta.rows = 4;
    ta.placeholder = `Comment on ${cpath}:${cline}`;
    ta.required = true;
    pArea.appendChild(ta);

    let startInp = null;
    if (cside !== 'old') {
      const pRange = document.createElement('p');
      pRange.className = 'hint';
      const lbl = document.createElement('label');
      lbl.style.display = 'inline';
      lbl.textContent = 'From line ';
      startInp = document.createElement('input');
      startInp.type = 'number';
      startInp.name = 'start_line';
      startInp.min = '1';
      startInp.max = cline;
      startInp.placeholder = cline;
      startInp.style.width = '5rem';
      startInp.style.height = '26px';
      lbl.appendChild(startInp);
      const bSug = document.createElement('button');
      bSug.type = 'button';
      bSug.className = 'linklike';
      bSug.style.marginLeft = '8px';
      bSug.textContent = 'Insert suggestion';
      bSug.addEventListener('click', () => {
        const srcCell = isSplit ? row.querySelectorAll('td.src')[1] : row.querySelector('td.src');
        const origLine = srcCell ? srcCell.textContent : '';
        const block = '```suggestion\n' + origLine + '\n```\n';
        const pos = ta.selectionStart ?? ta.value.length;
        ta.value = ta.value.slice(0, pos) + block + ta.value.slice(pos);
        ta.focus();
      });
      pRange.append(lbl, document.createTextNode(` to ${cline} · `), bSug);
      form.appendChild(pRange);
    }

    const pAct = document.createElement('p');
    pAct.className = 'btngroup';
    const bNow = document.createElement('button');
    bNow.type = 'submit';
    bNow.className = 'btn';
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

  // ---- 5. Cmd+K Command Palette (with live repo/file/issue/MR search) ----
  function currentRepoContext() {
    const head = document.querySelector('.repohead [data-repo], .repohead[data-repo]');
    if (!head) return { repo: '', ref: '' };
    return { repo: head.dataset.repo || '', ref: head.dataset.ref || '' };
  }

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
    for (const a of document.querySelectorAll('.railusermenu .raildrop a')) {
      const txt = a.textContent.trim();
      if (txt.includes('queries') || txt.includes('snippets') || txt.includes('Bookmarks')) {
        add('Account', txt, a.getAttribute('href'));
      }
    }
    add('Navigate', 'Dashboard', '/');
    add('Navigate', 'Explore repositories', '/explore');
    add('Navigate', 'Global search', '/search');
    add('Navigate', 'Notifications', '/notifications');
    add('Navigate', 'New repository', '/new');
    add('Navigate', 'Account settings', '/settings');
    return items;
  }

  let paletteFetchTimer = 0;
  function openPalette(filesOnly = false) {
    const dlg = document.getElementById('cmdk');
    if (!(dlg instanceof HTMLDialogElement)) return;
    const input = dlg.querySelector('input');
    const list = dlg.querySelector('ul');
    const ctx = currentRepoContext();
    const localItems = filesOnly ? [] : collectCommands();
    let remoteItems = [];
    let active = 0;

    const render = () => {
      const raw = input.value.trim();
      const q = raw.toLowerCase();
      const merged = [];
      const seen = new Set();
      for (const it of [...remoteItems, ...localItems]) {
        if (!it.href || seen.has(it.href)) continue;
        if (q && !(it.label + ' ' + it.group).toLowerCase().includes(q)) continue;
        seen.add(it.href);
        merged.push(it);
      }
      if (raw) {
        if (ctx.repo) {
          merged.push({ group: 'Code', label: `Search "${raw}" in ${ctx.repo}…`, href: `/${ctx.repo}/search?q=` + encodeURIComponent(raw) });
        }
        merged.push({ group: 'Search', label: `Search "${raw}" across site…`, href: '/search?q=' + encodeURIComponent(raw) });
      }
      active = Math.min(active, Math.max(0, merged.length - 1));
      list.textContent = '';
      if (merged.length === 0) {
        const li = document.createElement('li');
        li.className = 'cmdk-empty';
        li.textContent = filesOnly ? 'Type to jump to any file in this repository…' : 'No matching destinations';
        list.appendChild(li);
        return;
      }
      merged.slice(0, 50).forEach((it, idx) => {
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

    const loadRemote = (q) => {
      const params = new URLSearchParams({ format: 'palette', q });
      if (ctx.repo) params.set('repo', ctx.repo);
      if (ctx.ref) params.set('ref', ctx.ref);
      fetch('/search?' + params.toString(), { credentials: 'same-origin' })
        .then((r) => (r.ok ? r.json() : null))
        .then((data) => {
          if (data && Array.isArray(data.items)) {
            remoteItems = filesOnly ? data.items.filter((x) => x.group === 'File') : data.items;
            render();
          }
        })
        .catch(() => {});
    };

    input.placeholder = filesOnly && ctx.repo ? `Find file in ${ctx.repo}…` : 'Jump to repository, file, issue, MR, or page…';
    input.value = '';
    render();
    loadRemote('');
    if (!dlg.open) dlg.showModal();
    input.focus();

    input.oninput = () => {
      active = 0;
      render();
      clearTimeout(paletteFetchTimer);
      paletteFetchTimer = setTimeout(() => loadRemote(input.value.trim()), 110);
    };
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

  function openShortcutsHelp() {
    const dlg = document.getElementById('kbhelp');
    if (dlg instanceof HTMLDialogElement && !dlg.open) dlg.showModal();
  }

  document.addEventListener('click', (e) => {
    if (e.target.closest('.cmdk-trigger')) {
      e.preventDefault();
      openPalette(false);
      return;
    }
    if (e.target.closest('.kbhelp-trigger')) {
      e.preventDefault();
      openShortcutsHelp();
      return;
    }
    if (e.target.closest('.kbhelp-close')) {
      document.getElementById('kbhelp')?.close();
      return;
    }
    for (const id of ['cmdk', 'kbhelp']) {
      const dlg = document.getElementById(id);
      if (dlg && e.target === dlg) dlg.close();
    }
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
    if ((e.metaKey || e.ctrlKey) && e.key === 'Enter' && e.target instanceof HTMLTextAreaElement && e.target.form) {
      e.preventDefault();
      const btn = e.target.form.querySelector('button[type="submit"]:not([name="preview"]), button:not([type]):not([name="preview"])');
      if (btn) e.target.form.requestSubmit(btn);
      else e.target.form.requestSubmit();
      return;
    }
    if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
      e.preventDefault();
      openPalette();
      return;
    }
    if (isTyping(e.target) || e.metaKey || e.ctrlKey || e.altKey) return;

    if (e.key === '?') {
      e.preventDefault();
      openShortcutsHelp();
      return;
    }
    if (e.key === 't' && currentRepoContext().repo) {
      e.preventDefault();
      openPalette(true);
      return;
    }
    if (e.key === 'y') {
      const blobMain = document.querySelector('.blobmain[data-permalink]');
      if (blobMain && blobMain.dataset.permalink) {
        e.preventDefault();
        const nextUrl = blobMain.dataset.permalink + location.search + location.hash;
        history.replaceState(null, '', nextUrl);
        navigator.clipboard?.writeText(location.origin + nextUrl).catch(() => {});
        showToast('Immutable permalink copied');
        return;
      }
    }
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
    const prevTheme = document.documentElement.dataset.theme || 'system';
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
      const doc = new DOMParser().parseFromString(await res.text(), 'text/html');
      const errEl = doc.querySelector('main .error');
      if (errEl) {
        if (e.target.matches('[data-live-theme]')) {
          if (prevTheme === 'system') delete document.documentElement.dataset.theme;
          else document.documentElement.dataset.theme = prevTheme;
        }
        if (e.target.id) {
          const serverCtl = doc.getElementById(e.target.id);
          if (serverCtl instanceof HTMLInputElement && e.target instanceof HTMLInputElement && serverCtl.type === 'checkbox') {
            e.target.checked = serverCtl.checked;
          } else if (serverCtl instanceof HTMLSelectElement && e.target instanceof HTMLSelectElement) {
            e.target.value = serverCtl.value;
          }
        }
        showToast(errEl.textContent.trim(), true);
        return;
      }
      showToast('Saved');
    } catch {
      showToast('Save failed', true);
    }
  });

  // ---- 8. Live client-side table, list & link filters ----
  document.addEventListener('input', (e) => {
    const inp = e.target.closest('input[data-filter-table], input[data-filter-list], input[data-filter-links]');
    if (!inp) return;
    const q = inp.value.trim().toLowerCase();
    if (inp.dataset.filterLinks) {
      const box = inp.closest('.' + inp.dataset.filterLinks);
      if (!box) return;
      let shown = 0;
      for (const a of box.querySelectorAll('a:not(.allrefs)')) {
        const hide = q !== '' && !a.textContent.toLowerCase().includes(q);
        a.hidden = hide;
        if (!hide) shown++;
      }
      let empty = box.querySelector('.filter-empty');
      if (q !== '' && shown === 0) {
        if (!empty) {
          empty = document.createElement('span');
          empty.className = 'none filter-empty';
          empty.style.display = 'block';
          empty.style.padding = '4px 12px';
          empty.textContent = 'No matching branches';
          box.insertBefore(empty, box.querySelector('.allrefs'));
        }
        empty.hidden = false;
      } else if (empty) {
        empty.hidden = true;
      }
      return;
    }
    if (inp.dataset.filterTable) {
      const container = document.getElementById(inp.dataset.filterTable);
      const table = container?.tagName === 'TABLE' ? container : container?.querySelector('table');
      if (!table) return;
      const colSel = inp.dataset.filterCol || '';
      const tbody = table.querySelector('tbody') || table;
      let shown = 0;
      for (const row of tbody.querySelectorAll('tr:not(.filter-empty):not(.cols):not(.tipbar)')) {
        if (row.querySelector('td.empty')) continue;
        const cell = colSel ? row.querySelector(colSel) : row;
        const hay = (cell ? cell.textContent : row.textContent).toLowerCase();
        const hide = q !== '' && !hay.includes(q);
        row.hidden = hide;
        if (!hide) shown++;
      }
      let emptyRow = tbody.querySelector('tr.filter-empty');
      if (q !== '' && shown === 0) {
        if (!emptyRow) {
          emptyRow = document.createElement('tr');
          emptyRow.className = 'filter-empty';
          const td = document.createElement('td');
          td.colSpan = 10;
          td.className = 'empty';
          td.textContent = 'No matching items';
          emptyRow.appendChild(td);
          tbody.appendChild(emptyRow);
        }
        emptyRow.hidden = false;
      } else if (emptyRow) {
        emptyRow.hidden = true;
      }
      return;
    }
    const list = document.getElementById(inp.dataset.filterList);
    if (!list) return;
    const ul = list.tagName === 'UL' ? list : list.querySelector('ul');
    if (!ul) return;
    let shown = 0;
    for (const item of ul.querySelectorAll(':scope > li:not(.filter-empty)')) {
      if (item.classList.contains('empty')) continue;
      const hide = q !== '' && !item.textContent.toLowerCase().includes(q);
      item.hidden = hide;
      if (!hide) shown++;
    }
    let emptyLi = ul.querySelector(':scope > li.filter-empty');
    if (q !== '' && shown === 0) {
      if (!emptyLi) {
        emptyLi = document.createElement('li');
        emptyLi.className = 'empty filter-empty';
        emptyLi.textContent = 'No matching items';
        ul.appendChild(emptyLi);
      }
      emptyLi.hidden = false;
    } else if (emptyLi) {
      emptyLi.hidden = true;
    }
  });

  // ---- 9. In-place Refresh (/admin, /, /builds, /notifications) ----
  document.addEventListener('click', async (e) => {
    const btn = e.target.closest('button[data-refresh]');
    if (!btn) return;
    const url = btn.dataset.refresh === 'self' ? location.href : btn.dataset.refresh;
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
        initCopyButtons(next);
        initScrollSpy();
        showToast('Refreshed');
      }
    } catch {
      showToast('Refresh failed', true);
    } finally {
      btn.disabled = false;
    }
  });

  // ---- 9b. Smooth in-place tab switching (profile & list tabs) ----
  let tabSwappedOnPage = false;
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
      if (!tabSwappedOnPage && !(history.state && history.state.tabSwap)) {
        history.replaceState({ tabSwap: targetId }, '', location.href);
      }
      tabSwappedOnPage = true;
      cur.replaceWith(next);
      if (doc.title) document.title = doc.title;
      history.pushState({ tabSwap: targetId }, '', a.href);
      initCopyButtons(next);
    } catch {
      location.href = a.href;
    }
  });
  window.addEventListener('popstate', async (e) => {
    const targetId = (e.state && e.state.tabSwap) || document.querySelector('nav[data-tab-swap]')?.dataset.tabSwap;
    if (!targetId) return;
    const cur = document.getElementById(targetId);
    if (!cur) { location.reload(); return; }
    try {
      const res = await fetch(location.href, { credentials: 'same-origin' });
      if (!res.ok) { location.reload(); return; }
      const doc = new DOMParser().parseFromString(await res.text(), 'text/html');
      const next = doc.getElementById(targetId);
      if (!next) { location.reload(); return; }
      cur.replaceWith(next);
      if (doc.title) document.title = doc.title;
      initCopyButtons(next);
    } catch {
      location.reload();
    }
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

  // ---- 11. Client-side panel tabs (/new) & import repo-name inference ----
  function initPanelTabs() {
    const nav = document.querySelector('nav[data-panel-tabs]');
    if (!nav) return;
    const container = document.getElementById(nav.dataset.panelTabs);
    if (!container) return;
    const links = Array.from(nav.querySelectorAll('a[data-panel]'));
    const panels = Array.from(container.querySelectorAll('[data-panel-id]'));
    const show = (id) => {
      const valid = panels.some((p) => p.dataset.panelId === id);
      const target = valid ? id : panels[0]?.dataset.panelId;
      for (const p of panels) p.hidden = p.dataset.panelId !== target;
      for (const a of links) {
        if (a.dataset.panel === target) a.setAttribute('aria-current', 'page');
        else a.removeAttribute('aria-current');
      }
    };
    const initial = location.hash.replace(/^#/, '') || nav.dataset.activePanel || (document.querySelector('#import input[name="from"]')?.value ? 'import' : 'create');
    show(initial);
    for (const a of links) {
      a.addEventListener('click', (e) => {
        e.preventDefault();
        show(a.dataset.panel);
        history.replaceState(null, '', '#' + a.dataset.panel);
      });
    }
  }

  document.addEventListener('input', (e) => {
    const src = e.target.closest('input[data-infer-name]');
    if (!src) return;
    const dst = document.getElementById(src.dataset.inferName);
    if (!dst || dst.dataset.userEdited === '1') return;
    const raw = src.value.trim().replace(/\/+$/, '').replace(/\.git$/i, '');
    const seg = raw.split('/').pop() || '';
    const clean = seg.toLowerCase().replace(/[^a-z0-9._-]+/g, '-').replace(/^[^a-z0-9]+/, '').slice(0, 63);
    if (clean) dst.value = clean;
  });
  document.addEventListener('input', (e) => {
    if (e.target.id === 'imp-name') e.target.dataset.userEdited = e.target.value ? '1' : '';
  });

  function adaptShortcutLabels() {
    if (/Mac|iPhone|iPad|iPod/i.test(navigator.platform || navigator.userAgent || '')) return;
    for (const kbd of document.querySelectorAll('.cmdk-trigger kbd')) {
      kbd.textContent = kbd.textContent.replace('⌘', 'Ctrl+');
    }
    for (const el of document.querySelectorAll('.meta, .crumbs')) {
      if (el.childNodes.length === 1 && el.textContent.includes('⌘↵')) {
        el.textContent = el.textContent.replace('⌘↵', 'Ctrl+Enter');
      }
    }
  }

  // ---- 12. Diff "Viewed" checkboxes, inline hunk expansion & blob Shift+Click ranges ----
  function initDiffEnhancements(root = document) {
    const wrap = root.querySelector?.('.diffwrap') || document.querySelector('.diffwrap');
    if (!wrap) return;
    const repo = wrap.dataset.repo || '';
    const head = wrap.dataset.head || '';

    // Per-file Viewed checkbox backed by localStorage
    if (repo && head) {
      for (const fold of wrap.querySelectorAll('details.difffold[data-file-path]')) {
        const sum = fold.querySelector(':scope > summary');
        if (!sum || sum.dataset.viewedWired) continue;
        sum.dataset.viewedWired = '1';
        const fpath = fold.dataset.filePath;
        const key = `gb-viewed:${repo}:${head}:${fpath}`;
        const lbl = document.createElement('label');
        lbl.className = 'viewed-toggle';
        lbl.addEventListener('click', (ev) => ev.stopPropagation());
        const cb = document.createElement('input');
        cb.type = 'checkbox';
        cb.checked = localStorage.getItem(key) === '1';
        const syncViewed = (checked, collapse) => {
          const tocItem = wrap.querySelector(`.difftoc-list li[data-toc-path="${CSS.escape(fpath)}"]`);
          if (tocItem) tocItem.classList.toggle('toc-viewed', checked);
          if (collapse) fold.open = !checked;
        };
        if (cb.checked) syncViewed(true, true);
        cb.addEventListener('change', () => {
          try {
            if (cb.checked) localStorage.setItem(key, '1');
            else localStorage.removeItem(key);
          } catch {}
          syncViewed(cb.checked, true);
        });
        lbl.append(cb, document.createTextNode('Viewed'));
        sum.appendChild(lbl);
      }
    }

    // Inline hunk context expansion ("↑ Expand")
    if (repo && head) {
      const hunkRe = /^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@/;
      for (const tr of wrap.querySelectorAll('tr.hunk[data-hunk][data-file]')) {
        if (tr.dataset.expandWired) continue;
        tr.dataset.expandWired = '1';
        const m = hunkRe.exec(tr.dataset.hunk || '');
        if (!m) continue;
        let curNewStart = parseInt(m[2], 10);
        let curOldStart = parseInt(m[1], 10);
        if (curNewStart <= 1) continue;
        const srcTd = tr.querySelector('td.src');
        if (!srcTd) continue;
        const btn = document.createElement('button');
        btn.type = 'button';
        btn.className = 'hunk-expand';
        btn.textContent = '↑ Expand';
        btn.title = 'Load up to 20 preceding context lines';
        btn.addEventListener('click', async () => {
          const to = curNewStart - 1;
          const from = Math.max(1, to - 19);
          if (to < 1) { btn.remove(); return; }
          btn.disabled = true;
          try {
            const fpath = tr.dataset.file;
            const res = await fetch(`/${repo}/blob/${head}/${fpath}?lines=${from}:${to}`, { credentials: 'same-origin' });
            if (!res.ok) throw new Error('fetch');
            const data = await res.json();
            const isSplit = tr.closest('table')?.classList.contains('split');
            const oldOffset = curOldStart - curNewStart;
            for (const item of (data.lines || [])) {
              const ctxTr = document.createElement('tr');
              if (isSplit) {
                ctxTr.className = 'pair';
                const ln1 = document.createElement('td'); ln1.className = 'ln ctx old'; ln1.textContent = String(Math.max(1, item.n + oldOffset));
                const s1 = document.createElement('td'); s1.className = 'src ctx old chroma'; s1.innerHTML = item.html;
                const ln2 = document.createElement('td'); ln2.className = 'ln ctx'; ln2.textContent = String(item.n);
                const s2 = document.createElement('td'); s2.className = 'src ctx chroma'; s2.innerHTML = item.html;
                ctxTr.append(ln1, s1, ln2, s2);
              } else {
                ctxTr.className = 'ctx';
                const ln1 = document.createElement('td'); ln1.className = 'ln'; ln1.textContent = String(Math.max(1, item.n + oldOffset));
                const ln2 = document.createElement('td'); ln2.className = 'ln'; ln2.textContent = String(item.n);
                const s = document.createElement('td'); s.className = 'src chroma'; s.innerHTML = item.html;
                ctxTr.append(ln1, ln2, s);
              }
              tr.parentNode.insertBefore(ctxTr, tr.nextSibling);
            }
            // Move expanded rows in ascending order right after tr
            const inserted = [];
            let sib = tr.nextSibling;
            for (let k = 0; k < (data.lines || []).length && sib; k++) {
              inserted.push(sib);
              sib = sib.nextSibling;
            }
            inserted.reverse().forEach((n) => tr.parentNode.insertBefore(n, sib));
            curOldStart = Math.max(1, curOldStart - (curNewStart - from));
            curNewStart = from;
            if (curNewStart <= 1) btn.remove();
          } catch {
            showToast('Could not expand context', true);
          } finally {
            btn.disabled = false;
          }
        });
        srcTd.prepend(btn);
      }
    }
  }

  // Blob Shift+Click multi-line range (#L10-L25)
  function applyLineRangeFromHash() {
    const m = /^#L(\d+)(?:-L?(\d+))?$/.exec(location.hash);
    for (const el of document.querySelectorAll('.code .line.range-hl')) el.classList.remove('range-hl');
    if (!m || !m[2]) return;
    let a = parseInt(m[1], 10), b = parseInt(m[2], 10);
    if (a > b) [a, b] = [b, a];
    for (let i = a; i <= b; i++) {
      const anchor = document.getElementById('L' + i);
      const line = anchor?.closest('.line');
      if (line) line.classList.add('range-hl');
    }
  }
  let lastClickedLine = 0;
  document.addEventListener('click', (e) => {
    const lnLink = e.target.closest('.code a.lnlinks[href^="#L"]');
    if (!lnLink) return;
    const n = parseInt(lnLink.getAttribute('href').slice(2), 10);
    if (!n) return;
    if (e.shiftKey && lastClickedLine > 0 && lastClickedLine !== n) {
      e.preventDefault();
      const lo = Math.min(lastClickedLine, n), hi = Math.max(lastClickedLine, n);
      history.replaceState(null, '', `#L${lo}-L${hi}`);
      applyLineRangeFromHash();
    } else {
      lastClickedLine = n;
      setTimeout(applyLineRangeFromHash, 0);
    }
  });
  window.addEventListener('hashchange', applyLineRangeFromHash);

  // ---- 13. Composer Write | Preview tabs & @/# /! autocomplete ----
  function initComposers(root = document) {
    const forms = root.querySelectorAll ? root.querySelectorAll('form') : [];
    for (const form of forms) {
      if (form.dataset.composerWired) continue;
      const previewBtn = form.querySelector('button[name="preview"]');
      const ta = form.querySelector('textarea');
      if (!previewBtn || !ta) continue;
      form.dataset.composerWired = '1';

      const bar = document.createElement('div');
      bar.className = 'composer-bar';
      const writeTab = document.createElement('button');
      writeTab.type = 'button';
      writeTab.className = 'composer-tab';
      writeTab.setAttribute('aria-selected', 'true');
      writeTab.textContent = 'Write';

      const prevTab = document.createElement('button');
      prevTab.type = 'button';
      prevTab.className = 'composer-tab';
      prevTab.setAttribute('aria-selected', 'false');
      prevTab.textContent = 'Preview';

      const hint = document.createElement('span');
      hint.className = 'composer-hint';
      hint.textContent = '@user · #issue · !MR';
      bar.append(writeTab, prevTab, hint);

      const pane = document.createElement('div');
      pane.className = 'composer-preview rendered';
      pane.hidden = true;

      const wrapTarget = ta.closest('p') || ta;
      wrapTarget.parentNode.insertBefore(bar, wrapTarget);
      wrapTarget.parentNode.insertBefore(pane, wrapTarget.nextSibling);
      previewBtn.classList.add('js-hidden-preview');

      writeTab.addEventListener('click', () => {
        writeTab.setAttribute('aria-selected', 'true');
        prevTab.setAttribute('aria-selected', 'false');
        pane.hidden = true;
        wrapTarget.hidden = false;
        ta.focus();
      });

      prevTab.addEventListener('click', async () => {
        writeTab.setAttribute('aria-selected', 'false');
        prevTab.setAttribute('aria-selected', 'true');
        wrapTarget.hidden = true;
        pane.hidden = false;
        if (!ta.value.trim()) {
          pane.innerHTML = '<p class="none">Nothing to preview.</p>';
          return;
        }
        pane.innerHTML = '<p class="none">Rendering preview…</p>';
        try {
          const fd = new FormData(form);
          fd.set('preview', '1');
          const res = await fetch(form.action || location.href, {
            method: 'POST',
            body: new URLSearchParams(fd),
            headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
            credentials: 'same-origin',
          });
          if (!res.ok) throw new Error('preview');
          const doc = new DOMParser().parseFromString(await res.text(), 'text/html');
          const rendered = doc.querySelector('.preview .rendered');
          pane.innerHTML = rendered ? rendered.innerHTML : '<p class="none">Nothing to preview.</p>';
          initCopyButtons(pane);
        } catch {
          pane.innerHTML = '<p class="error">Preview failed.</p>';
        }
      });
    }
  }

  // Lightweight @mention, #issue, and !MR autocomplete on textareas
  let acMenu = null;
  let acItems = [];
  let acIndex = 0;
  let acContext = null;
  let acTimer = 0;

  function closeAutocomplete() {
    if (acMenu) acMenu.hidden = true;
    acItems = [];
    acContext = null;
  }

  function ensureAutocompleteMenu() {
    if (acMenu) return acMenu;
    acMenu = document.createElement('ul');
    acMenu.className = 'ac-menu';
    acMenu.hidden = true;
    document.body.appendChild(acMenu);
    return acMenu;
  }

  function applyAutocomplete(choice) {
    if (!acContext || !choice) return;
    const { ta, start, end } = acContext;
    const before = ta.value.slice(0, start);
    const after = ta.value.slice(end);
    const insert = choice.insert + ' ';
    ta.value = before + insert + after;
    const pos = before.length + insert.length;
    ta.setSelectionRange(pos, pos);
    ta.focus();
    closeAutocomplete();
  }

  function renderAutocomplete(ta) {
    const menu = ensureAutocompleteMenu();
    if (!acItems.length) {
      closeAutocomplete();
      return;
    }
    menu.innerHTML = '';
    acItems.forEach((it, idx) => {
      const li = document.createElement('li');
      li.className = idx === acIndex ? 'ac-active' : '';
      const b = document.createElement('strong');
      b.textContent = it.insert;
      const sp = document.createElement('span');
      sp.textContent = it.label;
      li.append(b, sp);
      li.addEventListener('mousedown', (ev) => {
        ev.preventDefault();
        applyAutocomplete(it);
      });
      menu.appendChild(li);
    });
    const rect = ta.getBoundingClientRect();
    menu.style.left = `${Math.max(8, rect.left + window.scrollX + 12)}px`;
    menu.style.top = `${rect.top + window.scrollY + Math.min(rect.height, 42)}px`;
    menu.hidden = false;
  }

  function collectPagePeople() {
    const set = new Set();
    for (const opt of document.querySelectorAll('#repo-people option')) {
      if (opt.value) set.add(opt.value);
    }
    for (const a of document.querySelectorAll('.commenthead a[href^="/"], .aside a[href^="/"]')) {
      const u = a.getAttribute('href').slice(1);
      if (u && !u.includes('/') && !u.includes('?')) set.add(u);
    }
    return Array.from(set);
  }

  document.addEventListener('input', (e) => {
    if (!(e.target instanceof HTMLTextAreaElement)) return;
    const ta = e.target;
    const pos = ta.selectionStart;
    const upto = ta.value.slice(0, pos);
    const m = /(?:^|\s)([@#!])([a-zA-Z0-9._-]{0,24})$/.exec(upto);
    if (!m) {
      closeAutocomplete();
      return;
    }
    const trig = m[1];
    const q = m[2].toLowerCase();
    const start = pos - m[2].length - 1;
    acContext = { ta, start, end: pos };
    acIndex = 0;

    if (trig === '@') {
      const people = collectPagePeople().filter((u) => u.toLowerCase().startsWith(q)).slice(0, 6);
      acItems = people.map((u) => ({ insert: '@' + u, label: u }));
      renderAutocomplete(ta);
      return;
    }
    const ctx = currentRepoContext();
    if (!ctx.repo) {
      closeAutocomplete();
      return;
    }
    clearTimeout(acTimer);
    acTimer = setTimeout(async () => {
      try {
        const params = new URLSearchParams({ format: 'palette', repo: ctx.repo, q });
        const res = await fetch('/search?' + params.toString(), { credentials: 'same-origin' });
        if (!res.ok) return;
        const data = await res.json();
        const wantGroup = trig === '#' ? 'Issue' : 'MR';
        acItems = (data.items || [])
          .filter((it) => it.group === wantGroup)
          .slice(0, 6)
          .map((it) => {
            const num = it.href.split('/').pop();
            return { insert: trig + num, label: it.label };
          });
        renderAutocomplete(ta);
      } catch {}
    }, 120);
  });

  document.addEventListener('keydown', (e) => {
    if (!acMenu || acMenu.hidden || !acItems.length) return;
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      acIndex = (acIndex + 1) % acItems.length;
      if (acContext) renderAutocomplete(acContext.ta);
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      acIndex = (acIndex - 1 + acItems.length) % acItems.length;
      if (acContext) renderAutocomplete(acContext.ta);
    } else if (e.key === 'Enter' || e.key === 'Tab') {
      e.preventDefault();
      applyAutocomplete(acItems[acIndex]);
    } else if (e.key === 'Escape') {
      e.preventDefault();
      closeAutocomplete();
    }
  }, true);
  document.addEventListener('blur', (e) => {
    if (e.target instanceof HTMLTextAreaElement) setTimeout(closeAutocomplete, 150);
  }, true);

  function bootAll() {
    initCopyButtons();
    initScrollSpy();
    initPanelTabs();
    adaptShortcutLabels();
    initDiffEnhancements();
    applyLineRangeFromHash();
    initComposers();
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', bootAll);
  } else {
    bootAll();
  }
})();
