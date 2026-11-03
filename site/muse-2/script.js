// Note: YouTube player may not load when page is opened from file:// due to browser security restrictions.
// Respect reduced motion
const reduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches;

// Scroll progress
const progress = document.getElementById('progress');
if (progress){
  window.addEventListener('scroll', ()=>{
    const h = document.documentElement.scrollHeight - window.innerHeight;
    const p = (window.scrollY / h) * 100;
    progress.style.width = p + '%';
  });
}

// IntersectionObserver for fade in
const sections = document.querySelectorAll('.card-section');
if (!reduced){
  const io = new IntersectionObserver((entries)=>{
    entries.forEach(e=>{
      if (e.isIntersecting){
        e.target.classList.add('visible');
      }
    });
  },{threshold:0.2});
  sections.forEach(s=>io.observe(s));
}else{
  sections.forEach(s=>s.classList.add('visible'));
}

// Hero capsule parallax
const capsule = document.querySelector('.hero-capsule');
if (capsule && !reduced){
  window.addEventListener('scroll', ()=>{
    const y = window.scrollY * 0.1;
    capsule.style.transform = `rotate(${y}deg) translateY(${y*0.5}px)`;
  });
}

// Oprah lines reveal via JS fallback
document.querySelectorAll('.oprah .line').forEach((el,i)=>{
  if (reduced){ el.style.opacity=1; return; }
  setTimeout(()=>{ el.style.opacity=1 }, i*600+200);
});

// Copy buttons
document.querySelectorAll('.copy').forEach(btn=>{
  btn.addEventListener('click', async ()=>{
    const pre = btn.parentElement.querySelector('pre');
    const text = pre ? pre.innerText.trim() : '';
    try{
      await navigator.clipboard.writeText(text);
      btn.textContent = 'Copied!';
      setTimeout(()=>{btn.textContent='Copy'},1500);
    }catch(e){
      const ta = document.createElement('textarea');
      ta.value = text;
      document.body.appendChild(ta);
      ta.select();
      document.execCommand('copy');
      document.body.removeChild(ta);
      btn.textContent='Copied!';
    }
  });
});

// Music button
const musicBtn = document.getElementById('musicBtn');
const ytContainer = document.getElementById('ytContainer');
let playing = false;
if (musicBtn){
  musicBtn.addEventListener('click', ()=>{
    if (playing){
      ytContainer.innerHTML = '';
      playing = false;
      musicBtn.setAttribute('aria-label','Play the theme');
      musicBtn.innerHTML = '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M9 18V5l7-3v13l-7 3z"/><path d="M5 9h14"/><path d="M3 12h2"/><path d="M3 16h2"/></svg><span>Play the theme</span>';
    }else{
      // YouTube embed created only after click
      // Note: may not load when opened from file:// due to browser security
      ytContainer.innerHTML = '<iframe width="1" height="1" src="https://www.youtube-nocookie.com/embed/CLiXUT3MS34?start=24&end=62&autoplay=1&rel=0&playsinline=1" allow="autoplay; encrypted-media" title="Pill theme music"></iframe>';
      playing = true;
      musicBtn.setAttribute('aria-label','Pause the theme');
      musicBtn.innerHTML = '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><path d="M16 12h-4l-2 4-2-4H6"/></svg><span>Pause the theme</span>';
    }
  });
}

// Smooth scroll for Get started
document.querySelectorAll('a[href="#install"]').forEach(a=>{
  a.addEventListener('click', e=>{
    e.preventDefault();
    document.getElementById('install').scrollIntoView({behavior:'smooth'});
  });
});
