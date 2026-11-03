/* pill landing page behaviour. Plain JS, no dependencies. */
(function () {
  "use strict";

  document.documentElement.classList.add("js");

  var reduceQuery = window.matchMedia("(prefers-reduced-motion: reduce)");
  var reduced = reduceQuery.matches;
  var hasIO = "IntersectionObserver" in window;

  /* ---------- Reveal on scroll ---------- */
  var revealEls = Array.prototype.slice.call(document.querySelectorAll(".reveal"));

  // Stagger siblings that share a parent (cards, steps, install blocks).
  revealEls.forEach(function (el) {
    var siblings = Array.prototype.filter.call(el.parentNode.children, function (c) {
      return c.classList.contains("reveal");
    });
    var i = siblings.indexOf(el);
    if (i > 0) el.style.setProperty("--delay", Math.min(i * 0.12, 0.6) + "s");
  });

  function showAll() {
    revealEls.forEach(function (el) { el.classList.add("is-visible"); });
  }

  if (reduced || !hasIO) {
    showAll();
  } else {
    var revealObserver = new IntersectionObserver(function (entries) {
      entries.forEach(function (entry) {
        if (entry.isIntersecting) {
          entry.target.classList.add("is-visible");
          revealObserver.unobserve(entry.target);
        }
      });
    }, { threshold: 0.15, rootMargin: "0px 0px -6% 0px" });
    revealEls.forEach(function (el) { revealObserver.observe(el); });
  }

  /* ---------- Scroll-linked: progress bar and hero capsule ---------- */
  var bar = document.querySelector(".progress-bar");
  var hero = document.querySelector(".hero");
  var art = document.querySelector(".hero-art");
  var ticking = false;

  function onScroll() {
    var y = window.scrollY || window.pageYOffset;
    var max = document.documentElement.scrollHeight - window.innerHeight;
    if (bar) bar.style.transform = "scaleX(" + (max > 0 ? Math.min(y / max, 1) : 0) + ")";

    if (art && hero && !reduced) {
      var h = hero.offsetHeight || 1;
      var p = Math.max(0, Math.min(y / h, 1));
      // Rotate, split the two halves apart, and drift slower than the page (parallax).
      art.style.setProperty("--rot", (-32 + p * 140).toFixed(2) + "deg");
      art.style.setProperty("--split", (p * 46).toFixed(1) + "px");
      art.style.setProperty("--lift", (p * h * 0.28).toFixed(1) + "px");
    }
    ticking = false;
  }

  window.addEventListener("scroll", function () {
    if (!ticking) {
      ticking = true;
      window.requestAnimationFrame(onScroll);
    }
  }, { passive: true });
  window.addEventListener("resize", onScroll);
  onScroll();

  /* ---------- Counting numbers ---------- */
  var counters = Array.prototype.slice.call(document.querySelectorAll("[data-count]"));

  function countUp(el) {
    var target = parseInt(el.getAttribute("data-count"), 10);
    var start = null;
    var duration = 1400;
    function frame(ts) {
      if (start === null) start = ts;
      var t = Math.min((ts - start) / duration, 1);
      var eased = 1 - Math.pow(1 - t, 3);
      el.textContent = String(Math.round(target * eased));
      if (t < 1) window.requestAnimationFrame(frame);
    }
    window.requestAnimationFrame(frame);
  }

  if (!reduced && hasIO && counters.length) {
    var countObserver = new IntersectionObserver(function (entries) {
      entries.forEach(function (entry) {
        if (entry.isIntersecting) {
          countUp(entry.target);
          countObserver.unobserve(entry.target);
        }
      });
    }, { threshold: 0.6 });
    counters.forEach(function (el) {
      el.textContent = "0";
      countObserver.observe(el);
    });
  }

  /* ---------- Terminal that types its commands ---------- */
  var terminal = document.querySelector("[data-typewriter]");

  function typeTerminal(term) {
    var lines = Array.prototype.slice.call(term.querySelectorAll(".t-line"));
    var texts = lines.map(function (line) {
      var cmd = line.querySelector(".t-cmd");
      var text = cmd.textContent;
      cmd.setAttribute("aria-label", text);
      cmd.textContent = "";
      line.classList.add("is-pending");
      return text;
    });

    var li = 0;
    function nextLine() {
      if (li >= lines.length) return;
      var line = lines[li];
      var cmd = line.querySelector(".t-cmd");
      var text = texts[li];
      var ci = 0;
      line.classList.remove("is-pending");
      line.classList.add("is-typing");
      (function typeChar() {
        ci += 1;
        cmd.textContent = text.slice(0, ci);
        if (ci < text.length) {
          window.setTimeout(typeChar, 22 + Math.random() * 30);
        } else {
          window.setTimeout(function () {
            line.classList.remove("is-typing");
            cmd.removeAttribute("aria-label");
            li += 1;
            window.setTimeout(nextLine, 120);
          }, 160);
        }
      })();
    }
    nextLine();
  }

  if (terminal && !reduced && hasIO) {
    var termObserver = new IntersectionObserver(function (entries) {
      entries.forEach(function (entry) {
        if (entry.isIntersecting) {
          termObserver.disconnect();
          typeTerminal(entry.target);
        }
      });
    }, { threshold: 0.35 });
    termObserver.observe(terminal);
  }

  /* ---------- Copy buttons ---------- */
  function fallbackCopy(text) {
    var ta = document.createElement("textarea");
    ta.value = text;
    ta.setAttribute("readonly", "");
    ta.style.position = "fixed";
    ta.style.top = "-1000px";
    ta.style.opacity = "0";
    document.body.appendChild(ta);
    ta.select();
    var ok = false;
    try { ok = document.execCommand("copy"); } catch (e) { ok = false; }
    document.body.removeChild(ta);
    return ok;
  }

  function selectText(node) {
    var range = document.createRange();
    range.selectNodeContents(node);
    var sel = window.getSelection();
    sel.removeAllRanges();
    sel.addRange(range);
  }

  function flash(btn, label) {
    var original = btn.getAttribute("data-label") || btn.textContent;
    btn.setAttribute("data-label", original);
    btn.textContent = label;
    btn.classList.toggle("is-done", label === "Copied");
    window.clearTimeout(btn._t);
    btn._t = window.setTimeout(function () {
      btn.textContent = original;
      btn.classList.remove("is-done");
    }, 1800);
  }

  document.querySelectorAll(".copy").forEach(function (btn) {
    btn.addEventListener("click", function () {
      var code = btn.parentNode.querySelector("code");
      var text = code.textContent.trim();
      function onFail() {
        if (fallbackCopy(text)) {
          flash(btn, "Copied");
        } else {
          selectText(code);
          flash(btn, "Press ⌘C");
        }
      }
      if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(text).then(function () { flash(btn, "Copied"); }, onFail);
      } else {
        onFail();
      }
    });
  });

  /* ---------- Music button ---------- */
  // The track is streamed from the official NoCopyrightSounds upload via YouTube's
  // embed player; nothing is downloaded or bundled. The iframe is only created after
  // the visitor clicks. Note: the YouTube player may not load when this page is
  // opened from file:// (the embed can refuse a null/file origin); serve the folder
  // over http (e.g. `python3 -m http.server`) if the button stays silent.
  var musicBtn = document.querySelector(".music");
  var musicFrame = document.getElementById("music-frame");
  var musicLabel = musicBtn && musicBtn.querySelector(".music-label");
  var CLIP_SECONDS = 62 - 24;
  var stopTimer = null;

  function setMusicState(playing) {
    musicBtn.setAttribute("aria-pressed", playing ? "true" : "false");
    var label = playing ? "Stop the theme" : "Play the theme";
    musicBtn.setAttribute("aria-label", label);
    if (musicLabel) musicLabel.textContent = label;
  }

  function stopMusic() {
    window.clearTimeout(stopTimer);
    while (musicFrame.firstChild) musicFrame.removeChild(musicFrame.firstChild);
    setMusicState(false);
  }

  function playMusic() {
    var iframe = document.createElement("iframe");
    iframe.src = "https://www.youtube-nocookie.com/embed/CLiXUT3MS34?start=24&end=62&autoplay=1&rel=0&playsinline=1";
    iframe.title = "Pill by Heuse & Zeus x Crona (feat. Emma Sameth), NoCopyrightSounds";
    iframe.setAttribute("allow", "autoplay; encrypted-media");
    iframe.setAttribute("tabindex", "-1");
    iframe.setAttribute("aria-hidden", "true");
    // Reset the button once the clip has had time to finish.
    iframe.addEventListener("load", function () {
      window.clearTimeout(stopTimer);
      stopTimer = window.setTimeout(stopMusic, (CLIP_SECONDS + 3) * 1000);
    });
    musicFrame.appendChild(iframe);
    setMusicState(true);
  }

  if (musicBtn && musicFrame) {
    musicBtn.addEventListener("click", function () {
      if (musicFrame.querySelector("iframe")) stopMusic();
      else playMusic();
    });
  }

  /* ---------- React to a live change of the motion preference ---------- */
  var onMotionChange = function (e) {
    reduced = e.matches;
    if (reduced) {
      showAll();
      if (art) {
        art.style.removeProperty("--rot");
        art.style.removeProperty("--split");
        art.style.removeProperty("--lift");
      }
    }
  };
  if (reduceQuery.addEventListener) reduceQuery.addEventListener("change", onMotionChange);
  else if (reduceQuery.addListener) reduceQuery.addListener(onMotionChange);
})();
