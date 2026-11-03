// Pill Landing Page Script

document.addEventListener('DOMContentLoaded', () => {
  // IntersectionObserver for scroll animations
  const observerOptions = {
    threshold: 0.1,
    rootMargin: '0px 0px -50px 0px'
  };

  const observer = new IntersectionObserver((entries) => {
    entries.forEach(entry => {
      if (entry.isIntersecting) {
        entry.target.classList.add('visible');
      }
    });
  }, observerOptions);

  // Observe all sections
  document.querySelectorAll('.section').forEach(section => {
    observer.observe(section);
  });

  // Hero capsule parallax/scroll effect
  const heroCapsule = document.querySelector('.hero-capsule');
  const heroSection = document.querySelector('.hero');

  if (heroCapsule && heroSection) {
    const handleScroll = () => {
      const scrollY = window.scrollY;
      const heroHeight = heroSection.offsetHeight;
      const progress = scrollY / (heroHeight * 0.3);

      // Clamp progress between 0 and 1
      const clampProgress = Math.min(Math.max(progress, 0), 1);

      // Apply parallax and rotation
      heroCapsule.style.transform = `translateY(${clampProgress * -30}px) rotate(${clampProgress * 20}deg)`;
      heroCapsule.style.opacity = 1 - clampProgress * 0.5;
    };

    window.addEventListener('scroll', handleScroll, { passive: true });
  }

  // Copy button functionality
  document.querySelectorAll('.copy-btn').forEach(btn => {
    btn.addEventListener('click', () => {
      const codeBlock = btn.previousElementSibling || btn.parentElement.querySelector('code');
      if (!codeBlock) return;

      const text = codeBlock.textContent || codeBlock.innerText;

      if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(text).then(() => {
          btn.textContent = 'Copied!';
          btn.classList.add('copied');
          setTimeout(() => {
            btn.textContent = 'Copy';
            btn.classList.remove('copied');
          }, 1500);
        });
      } else {
        // Fallback for older browsers
        const textarea = document.createElement('textarea');
        textarea.value = text;
        textarea.style.position = 'absolute';
        textarea.style.opacity = '0';
        document.body.appendChild(textarea);
        textarea.select();
        document.execCommand('copy');
        document.body.removeChild(textarea);

        btn.textContent = 'Copied!';
        btn.classList.add('copied');
        setTimeout(() => {
          btn.textContent = 'Copy';
          btn.classList.remove('copied');
        }, 1500);
      }
    });
  });

  // Music button functionality
  const musicBtn = document.querySelector('.music-btn');
  let musicPlaying = false;
  let youtubeFrame = null;

  if (musicBtn) {
    musicBtn.addEventListener('click', () => {
      if (!musicPlaying) {
        // Create YouTube iframe
        youtubeFrame = document.createElement('iframe');
        youtubeFrame.src = 'https://www.youtube-nocookie.com/embed/CLiXUT3MS34?start=24&end=62&autoplay=1&rel=0&playsinline=1';
        youtubeFrame.allow = 'autoplay; encrypted-media';
        youtubeFrame.style.width = '1px';
        youtubeFrame.style.height = '1px';
        youtubeFrame.style.border = 'none';
        youtubeFrame.style.opacity = '0';
        youtubeFrame.style.position = 'absolute';
        youtubeFrame.style.top = '-1000px';
        document.body.appendChild(youtubeFrame);

        musicBtn.innerHTML = `
          <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2">
            <rect x="4" y="8" width="2" height="8" rx="1"/>
            <rect x="8" y="4" width="2" height="16" rx="1"/>
            <rect x="12" y="6" width="2" height="12" rx="1"/>
            <rect x="16" y="10" width="2" height="4" rx="1"/>
          </svg>
        `;
        musicPlaying = true;
      } else {
        // Remove iframe to stop music
        if (youtubeFrame) {
          document.body.removeChild(youtubeFrame);
          youtubeFrame = null;
        }
        musicBtn.innerHTML = `
          <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2">
            <rect x="4" y="8" width="2" height="8" rx="1"/>
            <rect x="8" y="4" width="2" height="16" rx="1"/>
            <rect x="12" y="6" width="2" height="12" rx="1"/>
            <rect x="16" y="10" width="2" height="4" rx="1"/>
          </svg>
        `;
        musicPlaying = false;
      }
    });
  }

  // Respect prefers-reduced-motion - disable scroll effects if enabled
  const prefersReducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)');
  if (prefersReducedMotion.matches) {
    // Scroll effects are already disabled via CSS media query
    // But we can also disable the scroll listener if needed
    if (heroCapsule) {
      window.removeEventListener('scroll', () => {});
    }
  }
});

// Note: The YouTube player may not load when the page is opened from file://
// due to browser security restrictions on embedded content.