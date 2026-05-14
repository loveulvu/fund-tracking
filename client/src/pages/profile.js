import { useEffect, useState } from 'react';
import Link from 'next/link';
import DashboardShell from '../components/DashboardShell';
import api from '../lib/api';
import styles from '../../styles/Dashboard.module.css';

function formatPercent(value) {
  const number = Number(value);
  if (!Number.isFinite(number)) return '-';
  return `${number > 0 ? '+' : ''}${number.toFixed(2)}%`;
}

function getChangeClass(value) {
  const number = Number(value);
  if (!Number.isFinite(number) || number === 0) return styles.neutral;
  return number > 0 ? styles.positive : styles.negative;
}

function getToneClass(value) {
  const number = Number(value);
  if (!Number.isFinite(number) || number === 0) return styles.toneNeutral;
  return number > 0 ? styles.tonePositive : styles.toneNegative;
}

function formatDate(value) {
  if (!value) return '-';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '-';
  return date.toLocaleDateString();
}

export default function Profile() {
  const [user, setUser] = useState(null);
  const [watchlist, setWatchlist] = useState([]);
  const [fundData, setFundData] = useState({});
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [editingThreshold, setEditingThreshold] = useState(null);
  const [newThreshold, setNewThreshold] = useState('');

  useEffect(() => {
    const savedUser = localStorage.getItem('user');
    if (savedUser) {
      setUser(JSON.parse(savedUser));
    } else {
      window.location.href = '/login';
    }
  }, []);

  const fetchFundData = async (fundCode) => {
    try {
      const response = await api.getFund(fundCode);
      if (response.ok) {
        const data = await response.json();
        setFundData((previous) => ({
          ...previous,
          [fundCode]: data,
        }));
      }
    } catch (err) {
      console.error('Error fetching fund data:', err);
    }
  };

  useEffect(() => {
    const fetchWatchlist = async () => {
      if (!user) return;

      try {
        const token = localStorage.getItem('token');
        if (!token) {
          throw new Error('No token found');
        }

        const response = await api.getWatchlist(token);

        if (response.ok) {
          const data = await response.json();
          setWatchlist(data);

          for (const item of data) {
            fetchFundData(item.fundCode);
          }
        } else {
          throw new Error('Failed to fetch watchlist');
        }
      } catch (err) {
        setError('Unable to load watchlist.');
        console.error('Error fetching watchlist:', err);
      } finally {
        setLoading(false);
      }
    };

    fetchWatchlist();
  }, [user]);

  const handleUnwatch = async (fundCode) => {
    try {
      const token = localStorage.getItem('token');
      const response = await api.removeFromWatchlist(token, fundCode);

      if (response.ok) {
        setWatchlist(watchlist.filter((item) => item.fundCode !== fundCode));
        setFundData((previous) => {
          const next = { ...previous };
          delete next[fundCode];
          return next;
        });
      } else {
        const data = await response.json();
        alert(data.error || 'Failed to remove fund.');
      }
    } catch (err) {
      console.error('Error removing from watchlist:', err);
      alert('Failed to remove fund.');
    }
  };

  const handleUpdateThreshold = async (fundCode) => {
    if (!newThreshold || Number.isNaN(Number(newThreshold))) {
      alert('Please enter a valid threshold.');
      return;
    }

    try {
      const token = localStorage.getItem('token');
      const response = await api.updateWatchlistThreshold(token, fundCode, parseFloat(newThreshold));

      if (response.ok) {
        const updatedItem = await response.json();
        setWatchlist(watchlist.map((item) => (
          item.fundCode === fundCode ? updatedItem : item
        )));
        setEditingThreshold(null);
        setNewThreshold('');
      } else {
        const data = await response.json();
        alert(data.error || 'Failed to update threshold.');
      }
    } catch (err) {
      console.error('Error updating threshold:', err);
      alert('Failed to update threshold.');
    }
  };

  const handleLogout = () => {
    localStorage.removeItem('token');
    localStorage.removeItem('user');
    window.location.href = '/login';
  };

  return (
    <DashboardShell
      activeHref="/profile"
      noteTitle="Watchlist"
      noteText="Manage followed funds and alert thresholds from one place."
    >
      <header className={styles.pageHeader}>
        <div>
          <p className={styles.eyebrow}>Account</p>
          <h1>Account / Watchlist</h1>
          <p>Review your signed-in account and manage watched funds.</p>
        </div>
        <button className={styles.secondaryButton} type="button" onClick={handleLogout}>
          Logout
        </button>
      </header>

      {error && <div className={styles.messageBox}>{error}</div>}

      <section className={styles.accountGrid}>
        <article className={styles.panel}>
          <div className={styles.panelHeader}>
            <div>
              <h2>User information</h2>
              <p>Current browser session</p>
            </div>
          </div>
          <div className={styles.cardBody}>
            <dl className={styles.infoList}>
              <div>
                <dt>Email</dt>
                <dd>{user?.email || 'N/A'}</dd>
              </div>
              <div>
                <dt>Watchlist items</dt>
                <dd>{watchlist.length}</dd>
              </div>
            </dl>
          </div>
        </article>

        <article className={styles.sourcePanel}>
          <span>Data source</span>
          <strong>GET /api/watchlist</strong>
          <p>Watchlist changes are scoped by the authenticated user and fund code.</p>
        </article>
      </section>

      <article className={styles.panel}>
        <div className={styles.panelHeader}>
          <div>
            <h2>Watched funds</h2>
            <p>{loading ? 'Loading watchlist' : `${watchlist.length} followed funds`}</p>
          </div>
          <Link className={styles.detailLink} href="/about">
            Add funds
          </Link>
        </div>

        {loading ? (
          <div className={styles.loadingCards}>
            <span />
            <span />
            <span />
          </div>
        ) : watchlist.length === 0 ? (
          <div className={styles.emptyState}>
            <div className={styles.emptyMark} aria-hidden="true" />
            <strong>No watched funds yet</strong>
            <p>Open Funds to add items to your watchlist.</p>
            <Link className={styles.detailLink} href="/about">
              Go to Funds
            </Link>
          </div>
        ) : (
          <div className={styles.watchlistGrid}>
            {watchlist.map((item) => {
              const fund = fundData[item.fundCode];

              return (
                <article className={styles.watchCard} key={item.fundCode}>
                  <div className={styles.watchCardHeader}>
                    <div>
                      <h3>{item.fundName || fund?.fund_name || 'Fund'}</h3>
                      <p>{item.fundCode}</p>
                    </div>
                    <span className={[styles.changePill, getToneClass(fund?.day_growth)].join(' ')}>
                      {formatPercent(fund?.day_growth)}
                    </span>
                  </div>

                  <dl className={styles.metricGrid}>
                    <div>
                      <dt>Net value</dt>
                      <dd>{fund?.net_value ?? '-'}</dd>
                    </div>
                    <div>
                      <dt>Date</dt>
                      <dd>{fund?.net_value_date || '-'}</dd>
                    </div>
                    <div>
                      <dt>1 month</dt>
                      <dd className={getChangeClass(fund?.month_growth)}>
                        {formatPercent(fund?.month_growth)}
                      </dd>
                    </div>
                    <div>
                      <dt>1 year</dt>
                      <dd className={getChangeClass(fund?.year_growth)}>
                        {formatPercent(fund?.year_growth)}
                      </dd>
                    </div>
                  </dl>

                  <div className={styles.thresholdBox}>
                    <span>Alert threshold</span>
                    {editingThreshold === item.fundCode ? (
                      <div className={styles.thresholdEditor}>
                        <input
                          type="number"
                          value={newThreshold}
                          onChange={(event) => setNewThreshold(event.target.value)}
                          placeholder="Threshold"
                        />
                        <button
                          className={styles.primaryButtonSmall}
                          type="button"
                          onClick={() => handleUpdateThreshold(item.fundCode)}
                        >
                          Save
                        </button>
                        <button
                          className={styles.secondaryButtonSmall}
                          type="button"
                          onClick={() => {
                            setEditingThreshold(null);
                            setNewThreshold('');
                          }}
                        >
                          Cancel
                        </button>
                      </div>
                    ) : (
                      <div className={styles.thresholdDisplay}>
                        <strong>{item.alertThreshold}%</strong>
                        <button
                          className={styles.detailButton}
                          type="button"
                          onClick={() => {
                            setEditingThreshold(item.fundCode);
                            setNewThreshold(String(item.alertThreshold));
                          }}
                        >
                          Edit
                        </button>
                      </div>
                    )}
                  </div>

                  <div className={styles.cardActions}>
                    <span>Added {formatDate(item.addedAt)}</span>
                    <button
                      className={styles.dangerButton}
                      type="button"
                      onClick={() => handleUnwatch(item.fundCode)}
                    >
                      Remove
                    </button>
                  </div>
                </article>
              );
            })}
          </div>
        )}
      </article>
    </DashboardShell>
  );
}
