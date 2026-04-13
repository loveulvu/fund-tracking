import { useEffect, useRef, useState } from 'react';
import Link from 'next/link';
import { gsap } from 'gsap';
import styles from './PillNav.module.css';

const PillNav = ({
  items = [],
  activeHref,
  className = '',
  ease = 'power3.easeOut',
  baseColor = '#fff',
  pillColor = '#060010',
  hoveredPillTextColor = '#060010',
  pillTextColor,
  onMobileMenuClick,
  initialLoadAnimation = true
}) => {
  const resolvedPillTextColor = pillTextColor ?? baseColor;
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false);
  const circleRefs = useRef([]);
  const tlRefs = useRef([]);
  const hamburgerRef = useRef(null);
  const mobileMenuRef = useRef(null);
  const navItemsRef = useRef(null);

  const isExternalLink = href =>
    href && (href.startsWith('http://') ||
    href.startsWith('https://') ||
    href.startsWith('//') ||
    href.startsWith('mailto:') ||
    href.startsWith('tel:') ||
    href.startsWith('#'));

  const isRouterLink = href => href && !isExternalLink(href);

  useEffect(() => {
    const layout = () => {
      circleRefs.current.forEach(circle => {
        if (!circle?.parentElement) return;

        const pill = circle.parentElement;
        const rect = pill.getBoundingClientRect();
        const { width: w, height: h } = rect;
        if (h === 0) return;

        const R = ((w * w) / 4 + h * h) / (2 * h);
        const D = Math.ceil(2 * R) + 2;
        const delta = Math.ceil(R - Math.sqrt(Math.max(0, R * R - (w * w) / 4))) + 1;
        const originY = D - delta;

        circle.style.width = `${D}px`;
        circle.style.height = `${D}px`;
        circle.style.bottom = `-${delta}px`;

        gsap.set(circle, {
          xPercent: -50,
          scale: 0,
          transformOrigin: `50% ${originY}px`
        });

        const label = pill.querySelector(`.${styles['pill-label']}`);
        const labelHover = pill.querySelector(`.${styles['pill-label-hover']}`);

        if (label) gsap.set(label, { y: 0 });
        if (labelHover) gsap.set(labelHover, { y: h + 12, opacity: 0 });

        const index = circleRefs.current.indexOf(circle);
        if (index === -1) return;

        tlRefs.current[index]?.kill();
        const tl = gsap.timeline({ paused: true });

        tl.to(circle, { scale: 1.2, xPercent: -50, duration: 0.3, ease }, 0);

        if (label) {
          tl.to(label, { y: -(h + 8), duration: 0.3, ease }, 0);
        }

        if (labelHover) {
          gsap.set(labelHover, { y: Math.ceil(h + 100), opacity: 0 });
          tl.to(labelHover, { y: 0, opacity: 1, duration: 0.3, ease }, 0);
        }

        tlRefs.current[index] = tl;
      });
    };

    const initLayout = () => requestAnimationFrame(() => layout());
    initLayout();

    window.addEventListener('resize', initLayout);

    if (document.fonts?.ready) {
      document.fonts.ready.then(initLayout).catch(() => {});
    }

    const menu = mobileMenuRef.current;
    if (menu) {
      gsap.set(menu, { visibility: 'hidden', opacity: 0, scaleY: 1 });
    }

    if (initialLoadAnimation) {
      const navItemsEl = navItemsRef.current;
      if (navItemsEl) {
        gsap.set(navItemsEl, { width: 0, overflow: 'hidden' });
        gsap.to(navItemsEl, { width: 'auto', duration: 0.6, ease });
      }
    }

    const timelines = tlRefs.current;
    return () => {
      window.removeEventListener('resize', initLayout);
      timelines.forEach(tl => tl?.kill());
    };
  }, [items, ease, initialLoadAnimation]);

  const handleEnter = i => {
    tlRefs.current[i]?.play();
  };

  const handleLeave = i => {
    tlRefs.current[i]?.reverse();
  };

  const toggleMobileMenu = () => {
    const newState = !isMobileMenuOpen;
    setIsMobileMenuOpen(newState);

    const hamburger = hamburgerRef.current;
    const menu = mobileMenuRef.current;

    if (hamburger) {
      const lines = hamburger.querySelectorAll(`.${styles['hamburger-line']}`);
      if (newState) {
        gsap.to(lines[0], { rotation: 45, y: 3, duration: 0.3, ease });
        gsap.to(lines[1], { rotation: -45, y: -3, duration: 0.3, ease });
      } else {
        gsap.to(lines[0], { rotation: 0, y: 0, duration: 0.3, ease });
        gsap.to(lines[1], { rotation: 0, y: 0, duration: 0.3, ease });
      }
    }

    if (menu) {
      if (newState) {
        gsap.set(menu, { visibility: 'visible' });
        gsap.fromTo(
          menu,
          { opacity: 0, y: 10, scaleY: 1 },
          { opacity: 1, y: 0, scaleY: 1, duration: 0.3, ease, transformOrigin: 'top center' }
        );
      } else {
        gsap.to(menu, {
          opacity: 0, y: 10, scaleY: 1, duration: 0.2, ease, transformOrigin: 'top center',
          onComplete: () => gsap.set(menu, { visibility: 'hidden' })
        });
      }
    }
    onMobileMenuClick?.();
  };

  const cssVars = {
    ['--base']: baseColor,
    ['--pill-bg']: pillColor,
    ['--hover-text']: hoveredPillTextColor,
    ['--pill-text']: resolvedPillTextColor
  };

  return (
    <div className={`${styles['pill-nav-container']} ${className}`} style={cssVars}>
      <nav className={styles['pill-nav']} aria-label="Primary">
        
        <div className={`${styles['pill-nav-items']} ${styles['desktop-only']}`} ref={navItemsRef}>
          <ul className={styles['pill-list']} role="menubar">
            {items.map((item, i) => (
              <li key={item.href || `item-${i}`} role="none">
                {isRouterLink(item.href) ? (
                  <Link
                    role="menuitem"
                    href={item.href || '/'}
                    className={`${styles['pill']}${activeHref === item.href ? ' ' + styles['is-active'] : ''}`}
                    aria-label={item.ariaLabel || item.label}
                    onMouseEnter={() => handleEnter(i)}
                    onMouseLeave={() => handleLeave(i)}
                  >
                    <span className={styles['hover-circle']} aria-hidden="true" ref={el => { circleRefs.current[i] = el; }} />
                    <span className={styles['label-stack']}>
                      <span className={styles['pill-label']}>{item.label}</span>
                      <span className={styles['pill-label-hover']} aria-hidden="true">{item.label}</span>
                    </span>
                  </Link>
                ) : (
                  <a
                    role="menuitem"
                    href={item.href || '#'}
                    className={`${styles['pill']}${activeHref === item.href ? ' ' + styles['is-active'] : ''}`}
                    aria-label={item.ariaLabel || item.label}
                    onMouseEnter={() => handleEnter(i)}
                    onMouseLeave={() => handleLeave(i)}
                  >
                    <span className={styles['hover-circle']} aria-hidden="true" ref={el => { circleRefs.current[i] = el; }} />
                    <span className={styles['label-stack']}>
                      <span className={styles['pill-label']}>{item.label}</span>
                      <span className={styles['pill-label-hover']} aria-hidden="true">{item.label}</span>
                    </span>
                  </a>
                )}
              </li>
            ))}
          </ul>
        </div>

        <button
          className={`${styles['mobile-menu-button']} ${styles['mobile-only']}`}
          onClick={toggleMobileMenu}
          aria-label="Toggle menu"
          ref={hamburgerRef}
        >
          <span className={styles['hamburger-line']} />
          <span className={styles['hamburger-line']} />
        </button>
      </nav>

      <div className={`${styles['mobile-menu-popover']} ${styles['mobile-only']}`} ref={mobileMenuRef}>
        <ul className={styles['mobile-menu-list']}>
          {items.map((item, i) => (
            <li key={item.href || `mobile-item-${i}`}>
              {isRouterLink(item.href) ? (
                <Link
                  href={item.href || '/'}
                  className={`${styles['mobile-menu-link']}${activeHref === item.href ? ' ' + styles['is-active'] : ''}`}
                  onClick={() => setIsMobileMenuOpen(false)}
                >
                  {item.label}
                </Link>
              ) : (
                <a
                  href={item.href || '#'}
                  className={`${styles['mobile-menu-link']}${activeHref === item.href ? ' ' + styles['is-active'] : ''}`}
                  onClick={() => setIsMobileMenuOpen(false)}
                >
                  {item.label}
                </a>
              )}
            </li>
          ))}
        </ul>
      </div>
    </div>
  );
};

export default PillNav;
