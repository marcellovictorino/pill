const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)');
const reveals = document.querySelectorAll('.reveal');
if ('IntersectionObserver' in window && !reducedMotion.matches) {
  document.documentElement.classList.add('js-motion');
  const observer = new IntersectionObserver((entries, instance) => {
    entries.forEach(entry => {
      if (entry.isIntersecting) {
        entry.target.classList.add('visible');
        instance.unobserve(entry.target);
      }
    });
  }, { threshold: 0.08 });
  reveals.forEach(element => observer.observe(element));
}

const progress = document.querySelector('.progress span');
const capsule = document.querySelector('.capsule-wrap');
function updateScroll() {
  const distance = document.documentElement.scrollHeight - window.innerHeight;
  progress.style.transform = `scaleX(${distance > 0 ? window.scrollY / distance : 0})`;
  if (!reducedMotion.matches && window.scrollY < window.innerHeight) {
    capsule.style.transform = `translateY(${window.scrollY * 0.1}px) rotate(${-38 + window.scrollY * 0.035}deg)`;
  }
}
window.addEventListener('scroll', updateScroll, { passive: true });
updateScroll();

document.querySelectorAll('.copy').forEach(button => {
  button.addEventListener('click', async () => {
    const value = button.parentElement.querySelector('code').textContent;
    try {
      if (!navigator.clipboard || !window.isSecureContext) throw new Error('Clipboard unavailable');
      await navigator.clipboard.writeText(value);
    } catch (error) {
      const field = document.createElement('textarea');
      field.value = value;
      field.style.position = 'fixed';
      field.style.left = '-9999px';
      document.body.appendChild(field);
      field.select();
      const copied = document.execCommand('copy');
      field.remove();
      if (!copied) {
        button.textContent = 'Select text';
        return;
      }
    }
    button.textContent = 'Copied!';
    setTimeout(() => { button.textContent = 'Copy'; }, 2000);
  });
});

const musicButton = document.querySelector('.music-button');
const player = document.querySelector('#player');
musicButton.addEventListener('click', () => {
  const playing = musicButton.getAttribute('aria-pressed') === 'true';
  if (playing) {
    player.replaceChildren();
  } else {
    // The YouTube player may not load when the page is opened from file://.
    const iframe = document.createElement('iframe');
    iframe.src = 'https://www.youtube-nocookie.com/embed/CLiXUT3MS34?start=24&end=62&autoplay=1&rel=0&playsinline=1';
    iframe.allow = 'autoplay; encrypted-media';
    iframe.title = 'Pill theme music';
    player.appendChild(iframe);
  }
  musicButton.setAttribute('aria-pressed', String(!playing));
  musicButton.querySelector('.music-label').textContent = playing ? 'Play the theme' : 'Stop the theme';
});
