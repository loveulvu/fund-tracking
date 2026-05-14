import { useEffect, useState } from 'react';
import { goApiUrl } from '../lib/api';
import styles from './VersionBadge.module.css';

const frontendVersion = process.env.NEXT_PUBLIC_APP_VERSION || 'dev';
const showInProduction = process.env.NEXT_PUBLIC_SHOW_VERSION_BADGE === 'true';

export default function VersionBadge() {
  const [backendVersion, setBackendVersion] = useState('loading');
  const [backendCommit, setBackendCommit] = useState('');
  const [open, setOpen] = useState(false);
  const shouldRender = process.env.NODE_ENV !== 'production' || showInProduction;

  useEffect(() => {
    if (!shouldRender) return undefined;

    let cancelled = false;

    const fetchVersion = async () => {
      try {
        const res = await fetch(goApiUrl('/api/version'));
        if (!res.ok) {
          throw new Error(`status ${res.status}`);
        }

        const data = await res.json();
        if (cancelled) return;

        setBackendVersion(data?.version || 'unknown');
        setBackendCommit(data?.commit || '');
      } catch {
        if (!cancelled) {
          setBackendVersion('unreachable');
          setBackendCommit('');
        }
      }
    };

    fetchVersion();
    return () => {
      cancelled = true;
    };
  }, [shouldRender]);

  if (!shouldRender) {
    return null;
  }

  return (
    <div className={styles.wrapper}>
      <button
        className={styles.toggle}
        type="button"
        aria-expanded={open}
        onClick={() => setOpen((value) => !value)}
      >
        系统状态
      </button>
      {open && (
        <div className={styles.badge} aria-label="当前系统版本">
          <div className={styles.line}>前端：{frontendVersion}</div>
          <div className={styles.line}>
            后端：{backendVersion}
            {backendCommit ? ` (${backendCommit})` : ''}
          </div>
        </div>
      )}
    </div>
  );
}
