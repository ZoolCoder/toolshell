// toolshell: the ZoolCoder tool shell's only script. Served from /_ts so the
// CSP needs no 'unsafe-inline'. Every feature is opt-in by markup, so a page
// without the hooks below runs none of it. Theme and sidebar state are
// cookies the server reads, so nothing here has to run before first paint.
(function () {
  'use strict';

  function onReady(fn) {
    if (document.readyState !== 'loading') fn(); else document.addEventListener('DOMContentLoaded', fn);
  }
  // TEXT is the script's own text in the page's language, from #ts-text.
  var TEXT = {};
  function t(s) { return TEXT[s] || s; }
  function rtl() { return document.documentElement.dir === 'rtl'; }
  function typing() {
    var el = document.activeElement;
    return el && (/^(INPUT|TEXTAREA|SELECT)$/.test(el.tagName) || el.isContentEditable);
  }

  onReady(function () {
    try { TEXT = JSON.parse(document.getElementById('ts-text').textContent) || {}; } catch (e) { TEXT = {}; }
    var shell = document.querySelector('.shell');
    var toast = toasts();
    window.toolshell = { toast: toast };
    busy(shell);
    menus();
    var fold = sidebar(shell);
    var drawer = slideIn(shell);
    var tips = tooltips();
    if (fold) fold.onChange = tips.refresh;
    var pal = palette(toast);
    shortcuts(fold, drawer, pal);
    document.querySelectorAll('[data-keys]').forEach(function (b) { b.addEventListener('click', function () { if (pal) pal.keys(); }); });

    // Copy buttons: <button data-copy="#id"> copies that element's text. A
    // [data-label] child says Copied for a moment; the icon stays.
    document.querySelectorAll('[data-copy]').forEach(function (b) {
      b.addEventListener('click', function () {
        var src = document.querySelector(b.dataset.copy);
        if (!src) return;
        var failed = function () { toast('bad', t('Copy failed')); };
        if (!navigator.clipboard) { failed(); return; }
        navigator.clipboard.writeText(src.textContent.trim()).then(function () {
          var lbl = b.querySelector('[data-label]');
          var was = lbl ? lbl.textContent : '';
          if (lbl) lbl.textContent = t('Copied');
          b.classList.add('done');
          setTimeout(function () { if (lbl) lbl.textContent = was; b.classList.remove('done'); }, 1600);
        }, failed);
      });
    });

    // <button data-back> goes back, or home when there is no history.
    document.querySelectorAll('[data-back]').forEach(function (b) {
      b.addEventListener('click', function () {
        if (history.length > 1 && document.referrer) history.back(); else location.href = '/';
      });
    });

    // The ?msg= / ?err= leave the URL so a reload does not show them again.
    var u = new URL(location.href);
    if (u.searchParams.has('msg') || u.searchParams.has('err')) {
      u.searchParams.delete('msg'); u.searchParams.delete('err');
      history.replaceState(null, '', u.pathname + u.search + u.hash);
    }

    // Disclosure rows: <tr data-toggle="id"> opens the detail row with that id.
    document.querySelectorAll('[data-toggle]').forEach(function (row) {
      var target = document.getElementById(row.dataset.toggle);
      if (!target) return;
      function flip() {
        var open = target.hidden;
        target.hidden = !open; row.setAttribute('aria-expanded', open ? 'true' : 'false');
      }
      row.addEventListener('click', function (e) { if (!e.target.closest('a,button')) flip(); });
      row.addEventListener('keydown', function (e) { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); flip(); } });
    });

    document.querySelectorAll('table[data-table]').forEach(function (t) {
      var api = table(t);
      var csv = t.dataset.csv && document.getElementById(t.dataset.csv);
      if (csv) csv.addEventListener('click', function () { exportCSV(t, t.dataset.csvName || 'export.csv', api.visible()); });
    });
  });

  // sidebar folds to icons and back on desktop; a cookie keeps the choice so
  // the server draws it folded on the next page.
  function sidebar(shell) {
    var btn = document.querySelector('[data-fold]');
    if (!shell || !btn) return null;
    var api = { onChange: null, toggle: toggle };
    function label(mini) {
      var text = t(mini ? 'Expand menu' : 'Collapse menu');
      btn.setAttribute('aria-expanded', String(!mini));
      btn.setAttribute('aria-label', text);
      btn.dataset.tip = text + '  [';
    }
    function toggle() {
      var mini = shell.classList.toggle('mini');
      label(mini);
      document.cookie = (shell.dataset.cookie || 'ts') + '-side=' + (mini ? 'mini' : 'full') + '; path=/; max-age=31536000; samesite=lax';
      if (api.onChange) api.onChange();
    }
    label(shell.classList.contains('mini'));
    btn.addEventListener('click', toggle);
    return api;
  }

  // slideIn is the phone menu: the sidebar slides over the page with a scrim
  // behind it, and Escape, the scrim, the x or a link close it.
  function slideIn(shell) {
    var side = document.getElementById('ts-side');
    var scrim = document.querySelector('[data-scrim]');
    var opener = document.querySelector('[data-drawer]');
    if (!shell || !side || !scrim || !opener) return null;
    function isOpen() { return shell.classList.contains('open'); }
    function open() {
      scrim.hidden = false;
      opener.setAttribute('aria-expanded', 'true');
      document.body.classList.add('locked');
      // the menu is visibility:hidden until the class lands, and a hidden
      // link cannot take focus
      requestAnimationFrame(function () {
        shell.classList.add('open');
        var first = side.querySelector('nav a.on') || side.querySelector('nav a');
        if (first) first.focus();
      });
    }
    function close() {
      if (!isOpen()) return;
      shell.classList.remove('open');
      opener.setAttribute('aria-expanded', 'false');
      document.body.classList.remove('locked');
      setTimeout(function () { if (!isOpen()) scrim.hidden = true; }, 230);
      opener.focus();
    }
    opener.addEventListener('click', function () { if (isOpen()) close(); else open(); });
    scrim.addEventListener('click', close);
    document.querySelectorAll('[data-drawer-close]').forEach(function (b) { b.addEventListener('click', close); });
    document.addEventListener('keydown', function (e) {
      if (e.key === 'Escape' && isOpen()) { e.preventDefault(); close(); }
    });
    side.addEventListener('keydown', function (e) {
      // keep Tab inside the open menu
      if (e.key === 'Tab' && isOpen()) {
        var f = Array.prototype.filter.call(side.querySelectorAll('a[href],button'), function (el) { return el.offsetParent !== null; });
        if (!f.length) return;
        if (e.shiftKey && document.activeElement === f[0]) { e.preventDefault(); f[f.length - 1].focus(); }
        else if (!e.shiftKey && document.activeElement === f[f.length - 1]) { e.preventDefault(); f[0].focus(); }
      }
    });
    window.matchMedia('(min-width:901px)').addEventListener('change', function (m) { if (m.matches) close(); });
    return { open: open, close: close, isOpen: isOpen };
  }

  // tooltips: templates write plain title attributes, so the hint is there
  // without the script. This swaps them for a tooltip that also opens on
  // keyboard focus, closes with Escape, and is announced as the control's
  // description. A folded sidebar shows its labels the same way.
  function tooltips() {
    var tip = document.createElement('div');
    tip.id = 'ts-tip';
    tip.className = 'tip';
    tip.setAttribute('role', 'tooltip');
    tip.hidden = true;
    document.body.appendChild(tip);
    var host = null, timer = 0;
    var interactive = 'a[href],button,input,select,textarea,summary,[tabindex]';

    function adopt(root) {
      root.querySelectorAll('[title]:not(iframe):not(svg *):not(title)').forEach(function (el) {
        var text = el.getAttribute('title').trim();
        el.removeAttribute('title');
        if (!text) return;
        el.dataset.tip = text;
        // The title was this control's only name (an icon button): keep it as the label.
        if (el.matches(interactive) && !el.getAttribute('aria-label') && !el.textContent.trim()) el.setAttribute('aria-label', text);
        // A plain span cannot take focus, so screen readers get the hint as hidden text instead.
        if (!el.matches(interactive) && el.textContent.indexOf(text) < 0) {
          var sr = document.createElement('span');
          sr.className = 'sr-only';
          sr.textContent = ' (' + text + ')';
          el.appendChild(sr);
        }
      });
    }
    function navTips() {
      var mini = document.querySelector('.shell.mini') && window.matchMedia('(min-width:901px)').matches;
      document.querySelectorAll('.side nav a, .side .palhint').forEach(function (a) {
        var lbl = a.querySelector('.lbl');
        if (mini && lbl) { a.dataset.tip = lbl.textContent.trim(); a.dataset.tipPos = 'right'; }
        else { delete a.dataset.tip; delete a.dataset.tipPos; }
      });
      var fold = document.querySelector('[data-fold]');
      if (fold) fold.dataset.tipPos = mini ? 'right' : '';
    }
    function place() {
      var r = host.getBoundingClientRect(), t = tip.getBoundingClientRect(), gap = 8;
      var vw = document.documentElement.clientWidth, x, y;
      if (host.dataset.tipPos === 'right') {
        x = rtl() ? r.left - gap - t.width : r.right + gap; y = r.top + (r.height - t.height) / 2;
      } else {
        x = r.left + (r.width - t.width) / 2; y = r.top - t.height - gap;
        if (y < 4) y = r.bottom + gap;
      }
      x = Math.max(4, Math.min(x, vw - t.width - 4));
      tip.style.left = Math.round(x) + 'px';
      tip.style.top = Math.round(y) + 'px';
    }
    function show(el) {
      clearTimeout(timer);
      if (host && host !== el) hide();
      host = el;
      tip.textContent = el.dataset.tip;
      tip.hidden = false;
      place();
      var name = el.getAttribute('aria-label') || el.textContent.trim();
      if (name !== el.dataset.tip) el.setAttribute('aria-describedby', 'ts-tip');
    }
    function hide() {
      clearTimeout(timer);
      if (host && host.getAttribute('aria-describedby') === 'ts-tip') host.removeAttribute('aria-describedby');
      host = null;
      tip.hidden = true;
    }
    function later(fn, ms) { clearTimeout(timer); timer = setTimeout(fn, ms); }
    function target(e) { return e.target.closest && e.target.closest('[data-tip]'); }

    adopt(document);
    navTips();
    window.matchMedia('(min-width:901px)').addEventListener('change', navTips);
    document.addEventListener('mouseover', function (e) {
      var el = target(e);
      if (el && el !== host) later(function () { show(el); }, host ? 0 : 300);
      else if (el) clearTimeout(timer);
    });
    document.addEventListener('mouseout', function (e) {
      var to = e.relatedTarget;
      if (to && host && host.contains(to)) return;
      if (host || target(e)) later(hide, 100);
    });
    document.addEventListener('focusin', function (e) {
      var el = target(e);
      if (el && e.target.matches(':focus-visible')) show(el); else if (host) hide();
    });
    document.addEventListener('focusout', function () { if (host) hide(); });
    document.addEventListener('keydown', function (e) { if (e.key === 'Escape' && host) hide(); });
    window.addEventListener('scroll', function () { if (host) hide(); }, true);
    return { refresh: function () { hide(); navTips(); } };
  }

  // palette is the Ctrl/Cmd-K jump list: the sidebar's pages plus whatever
  // the app serves at the dialog's data-src.
  function palette(toast) {
    var dlg = document.getElementById('ts-palette');
    var help = document.getElementById('ts-keys');
    if (!dlg || !dlg.showModal) return null;
    var input = dlg.querySelector('input');
    var list = dlg.querySelector('ul');
    var count = dlg.querySelector('[data-palcount]');
    var mac = /Mac|iPhone|iPad/.test(navigator.platform || navigator.userAgent);
    document.querySelectorAll('[data-palkey]').forEach(function (k) { k.textContent = mac ? '⌘K' : 'Ctrl K'; });

    var pages = [];
    document.querySelectorAll('.side nav a[href]').forEach(function (a) {
      var lbl = a.querySelector('.lbl');
      var svg = a.querySelector('svg');
      pages.push({ label: lbl ? lbl.textContent.trim() : a.textContent.trim(), hint: t('Page'), href: a.getAttribute('href'), icon: svg ? svg.outerHTML : '' });
    });
    var extra = null, shown = [], active = 0, opener = null;

    function load() {
      if (extra || !dlg.dataset.src) return;
      extra = [];
      fetch(dlg.dataset.src, { cache: 'no-store', credentials: 'same-origin' }).then(function (r) {
        if (!r.ok) throw new Error(r.status);
        return r.json();
      }).then(function (d) {
        extra = d || [];
        render();
      }).catch(function () {
        extra = null;
        toast('bad', t('Could not load the actions list.'));
      });
    }
    // score: every query letter in order; earlier, word-start and back-to-back
    // hits rank higher. null is no match.
    function score(text, q) {
      text = (text || '').toLowerCase();
      var s = 0, at = -1;
      for (var i = 0; i < q.length; i++) {
        var k = text.indexOf(q[i], at + 1);
        if (k < 0) return null;
        s += (k === at + 1 ? 3 : 0) + (k === 0 || text[k - 1] === ' ' ? 2 : 0) - Math.min(k - at, 10) / 10;
        at = k;
      }
      return s;
    }
    function render() {
      var q = input.value.trim().toLowerCase();
      var actions = (extra || []).filter(function (it) { return it.post; });
      var links = (extra || []).filter(function (it) { return !it.post; });
      if (!q) {
        shown = actions.concat(pages);
      } else {
        shown = actions.concat(pages, links).map(function (it) {
          var s = score(it.label, q), h = score(it.hint, q);
          return { it: it, s: s !== null ? s : h !== null ? h - 100 : null };
        })
          .filter(function (x) { return x.s !== null; })
          .sort(function (a, b) { return b.s - a.s; })
          .slice(0, 12).map(function (x) { return x.it; });
      }
      active = 0;
      list.textContent = '';
      shown.forEach(function (it, i) {
        var li = document.createElement('li');
        li.id = 'ts-pal-' + i;
        li.setAttribute('role', 'option');
        if (it.icon) li.insertAdjacentHTML('beforeend', it.icon);
        var b = document.createElement('b');
        b.textContent = it.label;
        var sm = document.createElement('small');
        sm.textContent = it.hint || (it.post ? t('Action') : '');
        li.appendChild(b);
        li.appendChild(sm);
        li.addEventListener('mousedown', function (e) { e.preventDefault(); go(it); });
        li.addEventListener('mousemove', function () { if (active !== i) { active = i; mark(); } });
        list.appendChild(li);
      });
      if (!shown.length) {
        var none = document.createElement('li');
        none.className = 'none';
        none.textContent = t('Nothing matches');
        list.appendChild(none);
      }
      count.textContent = shown.length === 1 ? t('1 result') : t('%d results').replace('%d', shown.length);
      mark();
    }
    function mark() {
      list.querySelectorAll('[role=option]').forEach(function (li, i) {
        li.setAttribute('aria-selected', i === active ? 'true' : 'false');
        if (i === active) li.scrollIntoView({ block: 'nearest' });
      });
      if (shown.length) input.setAttribute('aria-activedescendant', 'ts-pal-' + active);
      else input.removeAttribute('aria-activedescendant');
    }
    function go(it) {
      if (it.post) {
        // requestSubmit, not submit, so the busy veil and its guards run
        var f = document.createElement('form');
        f.method = 'post';
        f.action = it.post;
        f.dataset.busy = it.label;
        Object.keys(it.fields || {}).forEach(function (k) {
          var h = document.createElement('input');
          h.type = 'hidden'; h.name = k; h.value = it.fields[k];
          f.appendChild(h);
        });
        document.body.appendChild(f);
        dlg.close();
        f.requestSubmit();
        return;
      }
      location.href = it.href;
    }
    function open() {
      if (dlg.open) return;
      opener = document.activeElement;
      input.value = '';
      load();
      render();
      dlg.showModal();
      input.focus();
    }
    function keys() {
      if (dlg.open) dlg.close();
      opener = opener || document.activeElement;
      help.showModal();
    }
    [dlg, help].forEach(function (d) {
      d.addEventListener('close', function () { if (opener && opener.focus) opener.focus(); opener = null; });
      d.addEventListener('click', function (e) { if (e.target === d) d.close(); });
      // Chrome skips the built-in cancel for a dialog opened without a user gesture
      d.addEventListener('keydown', function (e) { if (e.key === 'Escape') { e.preventDefault(); d.close(); } });
    });
    input.addEventListener('input', render);
    input.addEventListener('keydown', function (e) {
      if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
        e.preventDefault();
        if (!shown.length) return;
        active = (active + (e.key === 'ArrowDown' ? 1 : shown.length - 1)) % shown.length;
        mark();
      } else if (e.key === 'Enter' && shown[active]) {
        e.preventDefault();
        go(shown[active]);
      } else if (e.key === '?' && !input.value) {
        e.preventDefault();
        keys();
      }
    });
    document.querySelectorAll('[data-palette]').forEach(function (b) { b.addEventListener('click', open); });
    return {
      open: open, keys: keys,
      toggle: function () { if (help.open) help.close(); if (dlg.open) dlg.close(); else open(); },
      busy: function () { return dlg.open || help.open; }
    };
  }

  // shortcuts: Ctrl/Cmd-K palette, ? list, [ fold, g then a nav hotkey.
  function shortcuts(fold, drawer, pal) {
    var jumps = {};
    document.querySelectorAll('.side nav a[data-hotkey]').forEach(function (a) { jumps[a.dataset.hotkey] = a.getAttribute('href'); });
    var gAt = 0;
    document.addEventListener('keydown', function (e) {
      if (pal && (e.metaKey || e.ctrlKey) && !e.altKey && (e.key === 'k' || e.key === 'K')) {
        e.preventDefault();
        if (drawer) drawer.close();
        pal.toggle();
        return;
      }
      if (typing() || e.metaKey || e.ctrlKey || e.altKey || (pal && pal.busy())) return;
      if (e.key === '?' && pal) {
        e.preventDefault();
        pal.keys();
      } else if (e.key === '[' && fold) {
        e.preventDefault();
        fold.toggle();
      } else if (e.key === 'g') {
        gAt = Date.now();
      } else if (gAt && Date.now() - gAt < 1200 && jumps[e.key]) {
        gAt = 0;
        location.href = jumps[e.key];
      } else {
        gAt = 0;
      }
    });
  }

  // toasts: the server writes the outcome of a form as a toast; this wires its
  // close button and timer, and makes toast(kind, text) for the script. A
  // success leaves on its own once its bar runs out (paused while hovered);
  // an error stays until closed.
  function toasts() {
    var box = document.getElementById('ts-toasts');
    var still = window.matchMedia('(prefers-reduced-motion:reduce)');
    function dismiss(el) {
      if (!el.isConnected) return;
      el.classList.add('out');
      setTimeout(function () { el.remove(); }, still.matches ? 0 : 230);
    }
    function wire(el) {
      var life = el.querySelector('.life');
      if (!life) return;
      if (still.matches) setTimeout(function () { dismiss(el); }, 8000);
      else life.addEventListener('animationend', function () { dismiss(el); });
    }
    document.addEventListener('click', function (e) {
      var b = e.target.closest && e.target.closest('[data-dismiss]');
      var el = b && b.closest('.toast,.flash');
      if (!el) return;
      if (el.classList.contains('toast')) dismiss(el); else el.remove();
    });
    if (box) box.querySelectorAll('.toast').forEach(function (el) {
      wire(el);
      // a live region is announced when it changes, not when the page loads
      // with it, so the text goes in again once the page is up
      var d = el.querySelector('div');
      var kids = Array.prototype.slice.call(d.childNodes);
      d.replaceChildren();
      setTimeout(function () { d.replaceChildren.apply(d, kids); }, 150);
    });
    var icons = {
      ok: '<path d="M3.5 8.5l3 3 6-7"/>',
      bad: '<path d="M8 2.5l6 11H2z"/><path d="M8 6.5v3M8 11.5v.01"/>',
      close: '<path d="M4 4l8 8M12 4l-8 8"/>'
    };
    function svg(name) {
      return '<svg aria-hidden="true" focusable="false" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">' + icons[name] + '</svg>';
    }
    return function (kind, text) {
      if (!box) return;
      var ok = kind !== 'bad';
      var el = document.createElement('div');
      el.className = 'toast ' + (ok ? 'ok' : 'bad');
      el.setAttribute('role', ok ? 'status' : 'alert');
      el.innerHTML = '<span class="ti">' + svg(ok ? 'ok' : 'bad') + '</span><div><b></b><span></span></div>' +
        '<button class="iconbtn" type="button" data-dismiss>' + svg('close') + '</button>' + (ok ? '<i class="life"></i>' : '');
      el.querySelector('b').textContent = t(ok ? 'Done' : 'Something went wrong');
      el.querySelector('div>span').textContent = text;
      el.querySelector('button').setAttribute('aria-label', t('Dismiss'));
      box.appendChild(el);
      wire(el);
    };
  }

  // busy: a post blocks the page until the next one loads. The bar starts at
  // once; after a beat a veil covers the page with an elapsed timer, and the
  // shell goes inert so nothing else can be clicked or submitted. A form with
  // data-confirm="text" asks first; data-busy="Planning…" names the work.
  // A page restored from the back/forward cache comes back unlocked.
  function busy(shell) {
    var bar = document.getElementById('ts-progress');
    var veil = document.getElementById('ts-veil');
    var ask = document.getElementById('ts-confirm');
    var on = false, t0 = 0, timers = [], locked = [];

    function progress() {
      if (!bar) return;
      bar.hidden = true;
      void bar.offsetWidth; // restart the animation
      bar.hidden = false;
    }
    function tick() {
      var s = Math.floor((Date.now() - t0) / 1000);
      veil.querySelector('[data-veil-time]').textContent = s < 60 ? s + 's' : Math.floor(s / 60) + 'm ' + (s % 60) + 's';
      if (s >= 60 && !veil.classList.contains('slow')) {
        veil.classList.add('slow');
        veil.querySelector('[data-veil-note]').textContent = t('Still working — some providers are slow.');
      }
      timers.push(setTimeout(tick, 1000));
    }
    function start(form, submitter) {
      on = true; t0 = Date.now();
      document.documentElement.classList.add('busy');
      progress();
      // after the submit event, so the button's own name and value still go
      setTimeout(function () {
        form.querySelectorAll('button[type=submit],button:not([type])').forEach(function (b) {
          b.disabled = true; locked.push(b);
        });
        if (submitter) {
          submitter.classList.add('busy');
          var lbl = submitter.querySelector('[data-label]');
          if (lbl && form.dataset.busy) { lbl.dataset.was = lbl.textContent; lbl.textContent = form.dataset.busy; }
        }
        if (shell) shell.inert = true;
      }, 0);
      timers.push(setTimeout(function () {
        if (!veil) return;
        veil.querySelector('[data-veil-text]').textContent = form.dataset.busy || t('Working…');
        veil.hidden = false;
        tick();
      }, 250));
    }
    function stop() {
      on = false;
      timers.forEach(clearTimeout); timers = [];
      document.documentElement.classList.remove('busy');
      if (bar) bar.hidden = true;
      if (veil) { veil.hidden = true; veil.classList.remove('slow'); }
      if (shell) shell.inert = false;
      locked.forEach(function (b) {
        b.disabled = false; b.classList.remove('busy');
        var lbl = b.querySelector('[data-label]');
        if (lbl && lbl.dataset.was) { lbl.textContent = lbl.dataset.was; delete lbl.dataset.was; }
      });
      locked = [];
    }
    function confirmFirst(form, submitter) {
      if (!ask || !ask.showModal) return false;
      ask.querySelector('#ts-confirm-p').textContent = form.dataset.confirm;
      ask.returnValue = '';
      ask.addEventListener('close', function done() {
        ask.removeEventListener('close', done);
        if (ask.returnValue !== 'ok') return;
        form.dataset.confirmed = '1';
        form.requestSubmit(submitter && submitter.form === form ? submitter : undefined);
      });
      ask.showModal();
      ask.querySelector('[data-confirm-ok]').focus();
      return true;
    }

    document.addEventListener('submit', function (e) {
      var form = e.target, submitter = e.submitter;
      if ((form.getAttribute('method') || '').toLowerCase() === 'dialog') return;
      if (on) { e.preventDefault(); return; }
      if (form.dataset.confirm && !form.dataset.confirmed) {
        if (confirmFirst(form, submitter)) { e.preventDefault(); return; }
      }
      delete form.dataset.confirmed;
      if (e.defaultPrevented) return;
      if (form.method === 'post' || form.dataset.busy) start(form, submitter);
      else progress();
    });
    // the bar alone for a plain link to another page here
    document.addEventListener('click', function (e) {
      var a = e.target.closest && e.target.closest('a[href]');
      if (!a || e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
      if (a.target || a.hasAttribute('download') || a.origin !== location.origin) return;
      if (a.pathname === location.pathname && a.search === location.search && a.hash) return;
      progress();
    });
    window.addEventListener('pageshow', function (e) { if (e.persisted) stop(); });
    // A cross-document view transition is skipped when the next page is slow,
    // as after a long POST; its promises then reject with nothing to report.
    function quiet(e) {
      var vt = e.viewTransition;
      if (!vt) return;
      [vt.ready, vt.finished, vt.updateCallbackDone].forEach(function (p) { if (p) p.catch(function () {}); });
    }
    window.addEventListener('pageswap', quiet);
    window.addEventListener('pagereveal', quiet);
    window.addEventListener('pagehide', function () { if (bar) bar.hidden = true; });
  }

  // menus: the header popovers. Opening one focuses its first item; arrows,
  // Home and End move through the items; Escape and a click outside close
  // it on their own.
  function menus() {
    document.querySelectorAll('.menu[popover]').forEach(function (m) {
      function items() { return Array.prototype.slice.call(m.querySelectorAll('a,button')); }
      m.addEventListener('toggle', function (e) {
        if (e.newState !== 'open') return;
        var list = items();
        var pick = m.querySelector('[aria-checked=true]') || list[0];
        if (pick) pick.focus();
      });
      m.addEventListener('keydown', function (e) {
        var list = items(), i = list.indexOf(document.activeElement);
        var k = { ArrowDown: i + 1, ArrowUp: i - 1, Home: 0, End: list.length - 1 }[e.key];
        if (k === undefined || !list.length) return;
        e.preventDefault();
        list[(k + list.length) % list.length].focus();
      });
      m.addEventListener('click', function (e) {
        if (e.target.closest('button') && m.hidePopover) m.hidePopover();
      });
    });
  }

  // table: search, sort and pagination for a table whose rows carry
  // data-search. Options come from data attributes on the table:
  // data-search-input, data-shown, data-per-page, data-pager.
  function table(t) {
    var tbody = t.tBodies[0];
    var rows = Array.prototype.slice.call(tbody.querySelectorAll('tr[data-search]'));
    var q = t.dataset.searchInput ? document.querySelector(t.dataset.searchInput) : null;
    var shown = t.dataset.shown ? document.querySelector(t.dataset.shown) : null;
    var pager = t.dataset.pager ? document.querySelector(t.dataset.pager) : null;
    var per = Number(t.dataset.perPage) || 0, page = 1;

    function matches(r) {
      var term = q ? q.value.trim().toLowerCase() : '';
      return !term || r.dataset.search.indexOf(term) >= 0;
    }
    function button(label, p, disabled, on) {
      var b = document.createElement('button');
      b.type = 'button'; b.className = 'btn sm' + (on ? ' on' : ''); b.textContent = label; b.disabled = disabled;
      b.dataset.p = p;
      return b;
    }
    function render() {
      var vis = rows.filter(matches), n = vis.length;
      rows.forEach(function (r) { r.hidden = true; });
      var from = 0, to = n;
      if (per) {
        var pages = Math.max(1, Math.ceil(n / per));
        if (page > pages) page = pages;
        from = (page - 1) * per; to = Math.min(page * per, n);
        if (pager) {
          pager.replaceChildren(button('‹', page - 1, page <= 1));
          for (var p = 1; p <= pages; p++) {
            if (pages > 9 && Math.abs(p - page) > 2 && p !== 1 && p !== pages) {
              if (p === 2 || p === pages - 1) { var gap = document.createElement('span'); gap.textContent = '…'; pager.append(gap); }
              continue;
            }
            pager.append(button(String(p), p, false, p === page));
          }
          pager.append(button('›', page + 1, page >= pages));
          var info = document.createElement('span');
          info.textContent = (n ? from + 1 : 0) + '–' + to + ' of ' + n;
          pager.append(info);
        }
      }
      vis.slice(from, to).forEach(function (r) { r.hidden = false; });
      if (shown) shown.textContent = n;
    }

    // The search term lives in the URL so a view survives reload.
    var params = new URLSearchParams(location.search);
    if (q && params.get('q')) q.value = params.get('q');
    if (q) {
      q.addEventListener('input', function () {
        page = 1;
        var u = new URL(location.href);
        if (q.value.trim()) u.searchParams.set('q', q.value.trim()); else u.searchParams.delete('q');
        history.replaceState(null, '', u.pathname + u.search + u.hash);
        render();
      });
      document.addEventListener('keydown', function (e) {
        var typing = /input|select|textarea/i.test(document.activeElement.tagName);
        if (e.key === '/' && !typing) { e.preventDefault(); q.focus(); }
        if (e.key === 'Escape' && document.activeElement === q) { q.value = ''; q.dispatchEvent(new Event('input')); q.blur(); }
      });
    }
    if (pager) pager.addEventListener('click', function (e) {
      var b = e.target.closest('button[data-p]');
      if (b && !b.disabled) { page = Number(b.dataset.p); render(); }
    });
    t.querySelectorAll('th[data-sort]').forEach(function (th) {
      var btn = th.querySelector('button') || th;
      btn.addEventListener('click', function () {
        var key = th.dataset.sort, dir = th.getAttribute('aria-sort') === 'ascending' ? 'descending' : 'ascending';
        t.querySelectorAll('th[data-sort]').forEach(function (o) { o.removeAttribute('aria-sort'); });
        th.setAttribute('aria-sort', dir);
        var numeric = th.dataset.type === 'num', sign = dir === 'ascending' ? 1 : -1;
        rows.sort(function (a, b) {
          var x = a.dataset[key] || '', y = b.dataset[key] || '';
          return sign * (numeric ? Number(x) - Number(y) : x.localeCompare(y));
        });
        rows.forEach(function (r) { tbody.appendChild(r); });
        render();
      });
    });
    render();
    return { visible: function () { return rows.filter(matches); } };
  }

  function exportCSV(t, filename, rows) {
    function cells(r) {
      return Array.prototype.map.call(r.cells, function (c) {
        var v = (c.dataset.csv !== undefined ? c.dataset.csv : c.textContent).replace(/\s+/g, ' ').trim();
        // A leading = + - @ makes a spreadsheet run the cell as a formula.
        if (/^[=+\-@]/.test(v)) v = "'" + v;
        return '"' + v.replace(/"/g, '""') + '"';
      }).join(',');
    }
    var lines = [cells(t.tHead.rows[0])].concat(rows.map(cells));
    var url = URL.createObjectURL(new Blob([lines.join('\n')], { type: 'text/csv' }));
    var a = document.createElement('a');
    a.href = url; a.download = filename;
    document.body.appendChild(a); a.click(); a.remove();
    setTimeout(function () { URL.revokeObjectURL(url); }, 1000);
  }
})();
