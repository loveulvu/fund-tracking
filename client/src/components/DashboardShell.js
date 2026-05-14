import Link from 'next/link';
import { useState } from 'react';
import styles from '../../styles/Dashboard.module.css';

export default function DashboardShell({ activeHref, children, noteTitle, noteText }) {
  const [isLoggedIn] = useState(() => (
    typeof window !== 'undefined' && Boolean(localStorage.getItem('token'))
  ));

  const navItems = [
    { label: '总览', href: '/' },
    { label: '基金列表', href: '/about' },
    { label: '关注列表', href: '/profile' },
    { label: isLoggedIn ? '账户' : '登录', href: isLoggedIn ? '/profile' : '/login' },
  ];

  return (
    <main className={styles.shell}>
      <aside className={styles.sidebar}>
        <div className={styles.brand}>
          <span className={styles.brandMark}>FT</span>
          <div>
            <strong>FundTracking</strong>
            <small>基金跟踪面板</small>
          </div>
        </div>

        <nav className={styles.nav} aria-label="Main navigation">
          {navItems.map((item) => (
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
          <span>工作区</span>
          <strong>{noteTitle || '实时基金数据'}</strong>
          <p>{noteText || '查看基金行情，搜索基金，并管理个人关注列表。'}</p>
        </div>
      </aside>

      <section className={styles.content}>{children}</section>
    </main>
  );
}
