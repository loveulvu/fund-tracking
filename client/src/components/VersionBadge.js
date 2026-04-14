import { useEffect, useState } from 'react';
import { API_BASE_URL } from '../lib/api';
import styles from './VersionBadge.module.css';

const frontendVersion = process.env.NEXT_PUBLIC_APP_VERSION || 'dev';

export default function VersionBadge() {
  const [backendVersion, setBackendVersion] = useState('loading');
  const [backendCommit, setBackendCommit] = useState('');

  useEffect(() => {
    let cancelled = false;

    const fetchVersion = async () => {
      try {
        const res = await fetch(`${API_BASE_URL}/api/version`);
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
  }, []);

  return (
    <div className={styles.badge} aria-label="Current version">
      <div className={styles.line}>FE: {frontendVersion}</div>
      <div className={styles.line}>
        BE: {backendVersion}
        {backendCommit ? ` (${backendCommit})` : ''}
      </div>
    </div>
  );
}
