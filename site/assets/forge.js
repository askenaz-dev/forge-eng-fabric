/* ────────────────────────────────────────────────────────────────────────
   Forge marketing site — small interaction layer.
   No build step. No external deps.
   ──────────────────────────────────────────────────────────────────────── */

(function () {
  "use strict";

  // ── Theme toggle with transition guard ─────────────────────────────────
  var root = document.documentElement;
  var STORAGE_KEY = "forge_site_theme";

  function setTheme(next, persist) {
    root.setAttribute("data-theme-changing", "");
    root.setAttribute("data-theme", next);
    if (persist) {
      try { localStorage.setItem(STORAGE_KEY, next); } catch (_) { /* ignore */ }
    }
    requestAnimationFrame(function () {
      requestAnimationFrame(function () {
        root.removeAttribute("data-theme-changing");
      });
    });
  }

  var toggle = document.querySelector(".theme-toggle");
  if (toggle) {
    toggle.addEventListener("click", function () {
      var current = root.getAttribute("data-theme") || "light";
      setTheme(current === "dark" ? "light" : "dark", true);
    });
  }

  // ── Follow OS theme if no manual preference is saved ───────────────────
  try {
    var saved = localStorage.getItem(STORAGE_KEY);
    if (!saved && window.matchMedia) {
      var mq = window.matchMedia("(prefers-color-scheme: dark)");
      var handle = function (e) {
        if (localStorage.getItem(STORAGE_KEY)) return; // user has chosen
        setTheme(e.matches ? "dark" : "light", false);
      };
      if (mq.addEventListener) mq.addEventListener("change", handle);
      else if (mq.addListener) mq.addListener(handle);
    }
  } catch (_) { /* ignore */ }

  // ── Cmd K / Ctrl K → focus first GitHub link as a friendly fallback ────
  //    The marketing site doesn't ship the full command palette; this
  //    routes power users to the GitHub destination.
  document.addEventListener("keydown", function (e) {
    var isPalette = (e.key === "k" || e.key === "K") && (e.metaKey || e.ctrlKey);
    if (!isPalette) return;
    var tag = (e.target && e.target.tagName) || "";
    if (tag === "INPUT" || tag === "TEXTAREA") return;
    e.preventDefault();
    var cta = document.querySelector('.site-nav a.cta');
    if (cta && typeof cta.focus === "function") {
      cta.focus();
      cta.classList.add("flash");
      setTimeout(function () { cta.classList.remove("flash"); }, 600);
    }
  });

  // ── Smooth in-page anchor scrolling with sticky-nav offset ─────────────
  var navOffset = function () {
    var nav = document.querySelector(".site-nav");
    return nav ? nav.getBoundingClientRect().height + 8 : 0;
  };

  document.addEventListener("click", function (e) {
    var anchor = e.target.closest && e.target.closest('a[href^="#"]');
    if (!anchor) return;
    var href = anchor.getAttribute("href");
    if (!href || href === "#" || href.length < 2) return;
    var target = document.querySelector(href);
    if (!target) return;
    e.preventDefault();
    var y = target.getBoundingClientRect().top + window.pageYOffset - navOffset();
    window.scrollTo({ top: y, behavior: "smooth" });
    history.replaceState(null, "", href);
  });

  // ── Reveal pillars / stats on scroll (very lightweight) ────────────────
  if ("IntersectionObserver" in window) {
    var io = new IntersectionObserver(
      function (entries) {
        entries.forEach(function (entry) {
          if (entry.isIntersecting) {
            entry.target.classList.add("in-view");
            io.unobserve(entry.target);
          }
        });
      },
      { threshold: 0.12 },
    );
    document.querySelectorAll(".pillar, .stat, .step, .arch, .terminal").forEach(function (el) {
      el.classList.add("will-reveal");
      io.observe(el);
    });
  }
})();
