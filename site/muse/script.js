(() => {
  const reduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  if (reduced) return;

  // scroll progress
  const bar = document.getElementById('progress');
  const onScroll = () => {
    const h = document.documentElement.scrollHeight - window.innerHeight;
    const p = Math.min(100, Math.max(0, (window.scrollY / h) * 100));
    bar.style.width = p + '%';
  };
  window.addEventListener('scroll', onScroll, {passive:true});

  // section reveal
  const io = new IntersectionObserver(entries => {
    entries.forEach(e => {
      if (e.isIntersecting) e.target.classList.add('visible');
    });
  }, {threshold:0.2});
  document.querySelectorAll('.section').forEach(s => io.observe(s));

  // hero lines
  const lines = document.querySelectorAll('.oprah .line');
  const heroIO = new IntersectionObserver(entries => {
    if (entries[0].isIntersecting) {
      lines.forEach((el,i)=>{ setTimeout(()=>el.classList.add('show'), i*500); });
      heroIO.disconnect();
    }
  });
  heroIO.observe(document.getElementById('hero'));

  // hero capsule parallax
  const caps = document.querySelectorAll('.hero-capsule .capsule');
  window.addEventListener('scroll', () => {
    const y = window.scrollY / 10;
    caps[0].style.transform = `rotate(${y*0.5}deg) translateY(${y*0.3}px)`;
    caps[1].style.transform = `rotate(${-y*0.4}deg) translateY(${y*0.2}px)`;
  }, {passive:true});

  // copy buttons
  document.querySelectorAll('.copy').forEach(btn=>{
    btn.addEventListener('click', async ()=>{
      const code = btn.parentElement.querySelector('code').innerText;
      try {
        await navigator.clipboard.writeText(code);
        btn.textContent = 'Copied!';
      } catch {
        const ta = document.createElement('textarea');
        ta.value = code;
        document.body.appendChild(ta);
        ta.select();
        document.execCommand('copy');
        document.body.removeChild(ta);
        btn.textContent = 'Copied!';
      }
      setTimeout(()=>btn.textContent='Copy',1500);
    });
  });

  // music toggle
  const musicBtn = document.getElementById('music-toggle');
  const holder = document.getElementById('music-holder');
  let playing = false;
  musicBtn.addEventListener('click', () => {
    playing = !playing;
    musicBtn.setAttribute('aria-pressed', playing);
    musicBtn.classList.toggle('on', playing);
    if (playing){
      // YouTube embed created only after click
      holder.innerHTML = '<iframe src="https://www.youtube-nocookie.com/embed/CLiXUT3MS34?start=24&end=62&autoplay=1&rel=0&playsinline=1" allow="autoplay; encrypted-media" title="Pill theme music"></iframe>';
      // Note: YouTube player may not load when opened from file:// due to browser security restrictions
    } else {
      holder.innerHTML = '';
    }
  });

  // smooth scroll for buttons
  document.querySelectorAll('[data-scroll]').forEach(btn=>{
    btn.addEventListener('click', e => {
      e.preventDefault();
      const target = document.querySelector(btn.getAttribute('data-scroll'));
      target.scrollIntoView({behavior:'smooth'});
    });
  });
})();
