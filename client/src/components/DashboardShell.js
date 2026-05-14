import Link from 'next/link';
import styles from '../../styles/Dashboard.module.css';

const NAV_ITEMS = [
  { label: 'Dashboard', href: '/' },
  { label: 'Funds', href: '/about' },
  { label: 'Watchlist', href: '/profile' },
  { label: 'Login', href: '/login' },
];

export default function DashboardShell({ activeHref, children, noteTitle, noteText }) {
  return (
    <main className={styles.shell}>
      <aside className={styles.sidebar}>
        <div className={styles.brand}>
          <span className={styles.brandMark}>FT</span>
          <div>
            <strong>FundTracking</strong>
            <small>Fund dashboard</small>
          </div>
        </div>

        <nav className={styles.nav} aria-label="Main navigation">
          {NAV_ITEMS.map((item) => (
            <Link
              key={item.href}
              href={item.href}
              className={item.href === activeHref ? styles.navActive : ''}
            >
              <span className={styles.navDot} aria-hidden="true" />
              {item.label}
            </Link>
          ))}
        </nav>

        <div className={styles.sidebarNote}>
          <span>Workspace</span>
          <strong>{noteTitle || 'Live fund data'}</strong>
          <p>{noteText || 'Track funds, search data, and manage a personal watchlist.'}</p>
        </div>
      </aside>

      <section className={styles.content}>{children}</section>
    </main>
  );
}
