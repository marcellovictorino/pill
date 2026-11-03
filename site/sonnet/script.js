(function () {
  'use strict';

  var root = document.documentElement;
  var reduce = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  root.classList.add('js');

  /* ---------- Scroll reveal ---------- */
  var reveals = document.querySelectorAll('.reveal');
  if ('IntersectionObserver' in window && !reduce) {
    var io = new IntersectionObserver(function (entries) {
      entries.forEach(function (e) {
        if (e.isIntersecting) { e.target.classList.add('in'); io.unobserve(e.target); }
      });
    }, { threshold: 0.15, rootMargin: '0px 0px -6% 0px' });
    reveals.forEach(function (el, i) {
      var sib = el.parentElement ? Array.prototype.indexOf.call(el.parentElement.children, el) : 0;
      el.style.setProperty('--i', Math.min(sib, 4));
      io.observe(el);
    });
  } else {
    reveals.forEach(function (el) { el.classList.add('in'); });
  }

  /* ---------- Scroll-driven: progress bar + hero capsule ---------- */
  var bar = document.getElementById('progress');
  var capsule = document.getElementById('capsule');
  var ticking = false;
  function onScroll() {
    var y = window.scrollY || 0;
    var max = document.documentElement.scrollHeight - window.innerHeight;
    if (bar) bar.style.transform = 'scaleX(' + (max > 0 ? Math.min(y / max, 1) : 0) + ')';
    if (capsule && !reduce) {
      var h = window.innerHeight;
      var p = Math.min(y / h, 1.2);
      capsule.style.setProperty('--py', (y * 0.25).toFixed(1));
      capsule.style.setProperty('--rot', (p * 40).toFixed(1));
      capsule.style.setProperty('--split', (p * 120).toFixed(1));
    }
    ticking = false;
  }
  window.addEventListener('scroll', function () {
    if (!ticking) { ticking = true; requestAnimationFrame(onScroll); }
  }, { passive: true });
  onScroll();

  /* ---------- Pi context bar ---------- */
  var ctx = document.querySelector('.ctx');
  if (ctx && !reduce && 'IntersectionObserver' in window) {
    ctx.style.setProperty('--w', '62%');
    var cio = new IntersectionObserver(function (es) {
      if (es[0].isIntersecting) { ctx.style.setProperty('--w', '14%'); cio.disconnect(); }
    }, { threshold: 0.6 });
    cio.observe(ctx);
  }

  /* ---------- Terminal typing ---------- */
  var term = document.getElementById('term');
  function sleep(ms) { return new Promise(function (r) { setTimeout(r, ms); }); }
  function typeTerminal() {
    var rows = Array.prototype.slice.call(term.querySelectorAll('li'));
    rows.forEach(function (li) { li.querySelector('.cmd').textContent = ''; });
    return rows.reduce(function (chain, li) {
      return chain.then(function () {
        var cmd = li.querySelector('.cmd');
        var text = cmd.getAttribute('data-text');
        cmd.classList.add('typing');
        var i = 0;
        return new Promise(function (resolve) {
          (function tick() {
            cmd.textContent = text.slice(0, ++i);
            if (i < text.length) setTimeout(tick, 28); else resolve();
          })();
        }).then(function () {
          cmd.classList.remove('typing');
          li.classList.add('done');
          return sleep(220);
        });
      });
    }, Promise.resolve());
  }
  if (term) {
    if (reduce || !('IntersectionObserver' in window)) {
      term.querySelectorAll('li').forEach(function (li) { li.classList.add('done'); });
    } else {
      var tio = new IntersectionObserver(function (es) {
        if (es[0].isIntersecting) { tio.disconnect(); typeTerminal(); }
      }, { threshold: 0.25 });
      tio.observe(term);
    }
  }

  /* ---------- Counting numbers ---------- */
  var counters = document.querySelectorAll('[data-count]');
  function countUp(el) {
    var target = parseFloat(el.getAttribute('data-count'));
    var start = null, dur = 1400;
    function step(t) {
      if (start === null) start = t;
      var p = Math.min((t - start) / dur, 1);
      el.textContent = Math.round(target * (1 - Math.pow(1 - p, 3)));
      if (p < 1) requestAnimationFrame(step);
    }
    requestAnimationFrame(step);
  }
  if (!reduce && 'IntersectionObserver' in window) {
    counters.forEach(function (el) { el.textContent = '0'; });
    var nio = new IntersectionObserver(function (es) {
      es.forEach(function (e) {
        if (e.isIntersecting) { countUp(e.target); nio.unobserve(e.target); }
      });
    }, { threshold: 0.6 });
    counters.forEach(function (el) { nio.observe(el); });
  }

  /* ---------- Copy buttons ---------- */
  function fallbackCopy(text) {
    var ta = document.createElement('textarea');
    ta.value = text;
    ta.setAttribute('readonly', '');
    ta.style.cssText = 'position:fixed;top:0;left:0;opacity:0';
    document.body.appendChild(ta);
    ta.select();
    var ok = false;
    try { ok = document.execCommand('copy'); } catch (e) { ok = false; }
    document.body.removeChild(ta);
    return ok;
  }
  document.querySelectorAll('.copy').forEach(function (btn) {
    btn.addEventListener('click', function () {
      var text = btn.parentElement.querySelector('code').innerText.trim();
      var done = function (ok) {
        btn.textContent = ok ? 'Copied' : 'Press ⌘C';
        btn.classList.toggle('ok', ok);
        setTimeout(function () { btn.textContent = 'Copy'; btn.classList.remove('ok'); }, 1800);
      };
      if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(text).then(function () { done(true); }, function () { done(fallbackCopy(text)); });
      } else {
        done(fallbackCopy(text));
      }
    });
  });

  /* ---------- Music ---------- */
  // Note: the YouTube player may not load when this page is opened from file://
  // (YouTube rejects embeds without an http(s) referrer). Serve the folder over
  // http (for example `python3 -m http.server`) if the theme stays silent.
  // No audio is downloaded or bundled; only the official embed is created, on click.
  var musicBtn = document.getElementById('music');
  var label = document.getElementById('music-label');
  var holder = document.getElementById('player');
  var status = document.getElementById('status');
  var stopTimer = null;
  var SRC = 'https://www.youtube-nocookie.com/embed/CLiXUT3MS34?start=24&end=62&autoplay=1&rel=0&playsinline=1';

  function stopMusic() {
    clearTimeout(stopTimer);
    holder.innerHTML = '';
    musicBtn.setAttribute('aria-pressed', 'false');
    label.textContent = 'Play the theme';
    status.textContent = 'Theme stopped.';
  }
  function startMusic() {
    var f = document.createElement('iframe');
    f.src = SRC;
    f.title = 'Pill by Heuse & Zeus x Crona (feat. Emma Sameth), NoCopyrightSounds';
    f.allow = 'autoplay; encrypted-media';
    f.setAttribute('referrerpolicy', 'strict-origin-when-cross-origin');
    holder.appendChild(f);
    musicBtn.setAttribute('aria-pressed', 'true');
    label.textContent = 'Stop the theme';
    status.textContent = 'Theme playing.';
    stopTimer = setTimeout(stopMusic, 39000); // clip runs 0:24 to 1:02
  }
  musicBtn.addEventListener('click', function () {
    if (holder.firstChild) stopMusic(); else startMusic();
  });
})();
