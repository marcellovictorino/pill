// Scroll animations with IntersectionObserver
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

// Add fade-in class to sections and cards
document.querySelectorAll('.section, .feature-card, .terminal-block, .tier, .hero-content').forEach(el => {
    el.classList.add('fade-in');
    observer.observe(el);
});

// Hero capsule scroll reaction (parallax + rotation)
const heroCapsule = document.querySelector('.hero-capsule');
const heroSection = document.getElementById('hero');

function updateCapsule() {
    if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
        return;
    }
    
    const heroRect = heroSection.getBoundingClientRect();
    const scrollProgress = -heroRect.top / heroRect.height;
    
    // Clamp scroll progress between 0 and 1
    const progress = Math.max(0, Math.min(1, scrollProgress));
    
    // Rotate and shift the capsule based on scroll
    const rotation = progress * 30; // up to 30 degrees
    const translateY = progress * 50;
    
    heroCapsule.style.transform = `rotate(${rotation}deg) translateY(${translateY}px)`;
    
    // Split the halves slightly
    const leftHalf = document.querySelector('.capsule-left');
    const rightHalf = document.querySelector('.capsule-right');
    const splitOffset = progress * 20;
    
    leftHalf.style.transform = `translateX(${-splitOffset}px)`;
    rightHalf.style.transform = `translateX(${splitOffset}px)`;
}

window.addEventListener('scroll', updateCapsule, { passive: true });
updateCapsule();

// Music toggle
const musicToggle = document.querySelector('.music-toggle');
let musicPlaying = false;
let youtubeFrame = null;

musicToggle.addEventListener('click', () => {
    if (!musicPlaying) {
        // Create YouTube iframe
        youtubeFrame = document.createElement('iframe');
        youtubeFrame.setAttribute('src', 'https://www.youtube-nocookie.com/embed/CLiXUT3MS34?start=24&end=62&autoplay=1&rel=0&playsinline=1');
        youtubeFrame.setAttribute('allow', 'autoplay; encrypted-media');
        youtubeFrame.setAttribute('frameborder', '0');
        youtubeFrame.setAttribute('width', '1');
        youtubeFrame.setAttribute('height', '1');
        youtubeFrame.style.opacity = '0';
        youtubeFrame.style.position = 'absolute';
        youtubeFrame.style.top = '-100px';
        document.body.appendChild(youtubeFrame);
        
        musicToggle.setAttribute('aria-pressed', 'true');
        musicToggle.style.background = 'rgba(127, 220, 255, 0.3)';
        musicPlaying = true;
        
        // Note: The YouTube player may not load when the page is opened from file://
        // due to browser restrictions on autoplay and cross-origin frames.
    } else {
        // Remove iframe to stop music
        if (youtubeFrame) {
            document.body.removeChild(youtubeFrame);
            youtubeFrame = null;
        }
        
        musicToggle.setAttribute('aria-pressed', 'false');
        musicToggle.style.background = 'rgba(127, 220, 255, 0.1)';
        musicPlaying = false;
    }
});

// Copy buttons for code blocks
document.querySelectorAll('.copy-btn').forEach(btn => {
    btn.addEventListener('click', async () => {
        const codeBlock = btn.parentElement;
        const code = codeBlock.querySelector('code');
        const text = code.textContent;
        
        try {
            await navigator.clipboard.writeText(text);
            btn.textContent = 'Copied!';
            setTimeout(() => {
                btn.textContent = 'Copy';
            }, 1500);
        } catch (err) {
            // Fallback for older browsers
            const textarea = document.createElement('textarea');
            textarea.textContent = text;
            textarea.style.position = 'absolute';
            textarea.style.left = '-9999px';
            document.body.appendChild(textarea);
            textarea.select();
            document.execCommand('copy');
            document.body.removeChild(textarea);
            
            btn.textContent = 'Copied!';
            setTimeout(() => {
                btn.textContent = 'Copy';
            }, 1500);
        }
    });
});

// Smooth scroll for anchor links
document.querySelectorAll('a[href^="#"]').forEach(anchor => {
    anchor.addEventListener('click', function (e) {
        e.preventDefault();
        const target = document.querySelector(this.getAttribute('href'));
        if (target) {
            target.scrollIntoView({
                behavior: 'smooth',
                block: 'start'
            });
        }
    });
});
</content>
</tool_call>
<tool_call>
<function=write>
<parameter=path>
script.js