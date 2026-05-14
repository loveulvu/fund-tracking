import { useCallback, useEffect, useState } from 'react';
import Link from 'next/link';
import DashboardShell from '../components/DashboardShell';
import api from '../lib/api';
import styles from '../../styles/Dashboard.module.css';

function formatLastUpdated(value) {
  if (!value) return 'Unknown';

  const numeric = Number(value);
  if (Number.isFinite(numeric) && numeric > 0) {
    return new Date(numeric * 1000).toLocaleString();
  }

  const date = new Date(value);
  if (!Number.isNaN(date.getTime())) {
    return date.toLocaleString();
  }

  return String(value);
}

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

export default function About() {
  const [fundsData, setFundsData] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [searchTerm, setSearchTerm] = useState('');
  const [filteredFunds, setFilteredFunds] = useState([]);
  const [user, setUser] = useState(null);
  const [watchlist, setWatchlist] = useState([]);
  const [watchlistLoading, setWatchlistLoading] = useState({});
  const [lastUpdatedText, setLastUpdatedText] = useState('Unknown');

  useEffect(() => {
    const savedUser = localStorage.getItem('user');
    if (savedUser) {
      setUser(JSON.parse(savedUser));
    }
  }, []);

  const fetchWatchlist = useCallback(async () => {
    if (!user) return;

    try {
      const token = localStorage.getItem('token');
      const response = await api.getWatchlist(token);

      if (response.ok) {
        const data = await response.json();
        setWatchlist(data);
      }
    } catch (err) {
      console.error('Error fetching watchlist:', err);
    }
  }, [user]);

  useEffect(() => {
    if (user) {
      fetchWatchlist();
    }
  }, [user, fetchWatchlist]);

  const isWatched = (fundCode) => {
    return watchlist.some((item) => item.fundCode === fundCode);
  };

  const handleWatch = async (fund) => {
    if (!user) {
      alert('Please log in first.');
      return;
    }

    const fundCode = fund.fund_code;
    setWatchlistLoading((previous) => ({ ...previous, [fundCode]: true }));

    try {
      const token = localStorage.getItem('token');
      const response = await api.addToWatchlist(token, {
        fundCode,
        fundName: fund.fund_name,
        alertThreshold: 5,
      });

      if (response.ok) {
        await fetchWatchlist();
      } else {
        const data = await response.json();
        alert(data.error || 'Failed to follow fund.');
      }
    } catch (err) {
      console.error('Error adding to watchlist:', err);
      alert('Failed to follow fund.');
    } finally {
      setWatchlistLoading((previous) => ({ ...previous, [fundCode]: false }));
    }
  };

  const handleUnwatch = async (fundCode) => {
    if (!user) return;

    setWatchlistLoading((previous) => ({ ...previous, [fundCode]: true }));

    try {
      const token = localStorage.getItem('token');
      const response = await api.removeFromWatchlist(token, fundCode);

      if (response.ok) {
        await fetchWatchlist();
      } else {
        const data = await response.json();
        alert(data.error || 'Failed to remove fund.');
      }
    } catch (err) {
      console.error('Error removing from watchlist:', err);
      alert('Failed to remove fund.');
    } finally {
      setWatchlistLoading((previous) => ({ ...previous, [fundCode]: false }));
    }
  };

  useEffect(() => {
    const fetchFundsData = async () => {
      try {
        setLoading(true);
        const data = await api.getFunds();

        if (Array.isArray(data)) {
          setFundsData(data);
          setFilteredFunds(data);
          if (data.length > 0) {
            const latest = data.reduce((current, item) => {
              const next = item?.update_time;
              if (!current) return next;
              return String(next || '') > String(current || '') ? next : current;
            }, '');
            setLastUpdatedText(formatLastUpdated(latest));
          } else {
            setLastUpdatedText('Unknown');
          }
        } else {
          console.error('Invalid data format:', data);
          setError('Invalid data format received from server.');
        }
        setError(null);
      } catch (err) {
        setError(`Error fetching funds data: ${err.message}`);
        console.error(err);
      } finally {
        setLoading(false);
      }
    };

    fetchFundsData();
  }, []);

  const handleSearch = async (event) => {
    event.preventDefault();
    const keyword = searchTerm.trim();

    if (keyword === '') {
      setFilteredFunds(fundsData);
      return;
    }

    const isFundCode = /^\d{6}$/.test(keyword);

    if (isFundCode) {
      try {
        setLoading(true);
        const response = await api.getFund(keyword);
        if (!response.ok) {
          throw new Error('Failed to fetch fund data');
        }
        const data = await response.json();

        if (!data || !data.fund_code) {
          throw new Error('Invalid fund data received');
        }

        setFilteredFunds([data]);
        setError(null);
      } catch (err) {
        setError(`Error fetching fund data: ${err.message}`);
        console.error(err);
      } finally {
        setLoading(false);
      }
    } else {
      try {
        setLoading(true);
        const response = await api.searchFunds(keyword);
        if (response.ok) {
          const data = await response.json();
          if (Array.isArray(data) && data.length > 0) {
            setFilteredFunds(data);
          } else {
            const term = keyword.toLowerCase();
            setFilteredFunds(fundsData.filter((fund) => {
              const fundName = fund.fund_name || '';
              const fundCode = fund.fund_code || '';
              return fundName.toLowerCase().includes(term) || fundCode.includes(term);
            }));
          }
        } else {
          const term = keyword.toLowerCase();
          setFilteredFunds(fundsData.filter((fund) => {
            const fundName = fund.fund_name || '';
            const fundCode = fund.fund_code || '';
            return fundName.toLowerCase().includes(term) || fundCode.includes(term);
          }));
        }
        setError(null);
      } catch (err) {
        const term = keyword.toLowerCase();
        setFilteredFunds(fundsData.filter((fund) => {
          const fundName = fund.fund_name || '';
          const fundCode = fund.fund_code || '';
          return fundName.toLowerCase().includes(term) || fundCode.includes(term);
        }));
        setError('Search failed. Showing local matches.');
        console.error(err);
      } finally {
        setLoading(false);
      }
    }
  };

  const handleClearSearch = () => {
    setSearchTerm('');
    setFilteredFunds(fundsData);
    setError(null);
  };

  return (
    <DashboardShell
      activeHref="/about"
      noteTitle="Funds"
      noteText="Search live fund data and add funds to your watchlist."
    >
      <header className={styles.pageHeader}>
        <div>
          <p className={styles.eyebrow}>Funds</p>
          <h1>Funds</h1>
          <p>Search the live fund list and manage watched funds.</p>
        </div>
        <div className={styles.heroMeta}>
          <span>Last updated</span>
          <strong>{lastUpdatedText}</strong>
        </div>
      </header>

      <form className={styles.searchPanel} onSubmit={handleSearch}>
        <label className={styles.search}>
          <span aria-hidden="true" />
          <input
            type="text"
            value={searchTerm}
            onChange={(event) => setSearchTerm(event.target.value)}
            placeholder="Search by fund name or 6-digit code"
          />
        </label>
        <button className={styles.primaryButton} type="submit">
          Search
        </button>
        <button className={styles.secondaryButton} type="button" onClick={handleClearSearch}>
          Clear
        </button>
      </form>

      {error && <div className={styles.messageBox}>{error}</div>}

      <article className={styles.panel}>
        <div className={styles.panelHeader}>
          <div>
            <h2>Fund list</h2>
            <p>{loading ? 'Loading fund data' : `${filteredFunds.length} results`}</p>
          </div>
          <span className={styles.panelBadge}>Live API</span>
        </div>

        {loading ? (
          <div className={styles.loadingList}>
            <span />
            <span />
            <span />
            <span />
          </div>
        ) : filteredFunds.length === 0 ? (
          <div className={styles.emptyState}>
            <div className={styles.emptyMark} aria-hidden="true" />
            <strong>No funds found</strong>
            <p>Try a fund name or a 6-digit fund code.</p>
          </div>
        ) : (
          <div className={styles.tableWrap}>
            <table className={styles.table}>
              <thead>
                <tr>
                  <th>Fund</th>
                  <th>Code</th>
                  <th>Net value</th>
                  <th>Daily change</th>
                  <th>Date</th>
                  <th>Watchlist</th>
                </tr>
              </thead>
              <tbody>
                {filteredFunds.map((fund) => (
                  <tr key={fund.fund_code}>
                    <td>
                      <div className={styles.fundIdentity}>
                        <span>{String(fund.fund_name || 'F').slice(0, 1)}</span>
                        <div>
                          <strong>{fund.fund_name || 'Fund'}</strong>
                          <small>{fund.fund_company || fund.fund_type || 'Fund data'}</small>
                        </div>
                      </div>
                    </td>
                    <td>{fund.fund_code}</td>
                    <td>{fund.net_value ?? '-'}</td>
                    <td>
                      <span className={[styles.changePill, getToneClass(fund.day_growth)].join(' ')}>
                        {formatPercent(fund.day_growth)}
                      </span>
                    </td>
                    <td>{fund.net_value_date || '-'}</td>
                    <td>
                      {user ? (
                        isWatched(fund.fund_code) ? (
                          <button
                            className={styles.dangerButton}
                            type="button"
                            onClick={() => handleUnwatch(fund.fund_code)}
                            disabled={watchlistLoading[fund.fund_code]}
                          >
                            {watchlistLoading[fund.fund_code] ? 'Saving...' : 'Remove'}
                          </button>
                        ) : (
                          <button
                            className={styles.detailButton}
                            type="button"
                            onClick={() => handleWatch(fund)}
                            disabled={watchlistLoading[fund.fund_code]}
                          >
                            {watchlistLoading[fund.fund_code] ? 'Saving...' : 'Follow'}
                          </button>
                        )
                      ) : (
                        <Link className={styles.detailLink} href="/login">
                          Login
                        </Link>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </article>

      <section className={styles.fundCardsCompact}>
        {filteredFunds.slice(0, 3).map((fund) => (
          <article className={styles.fundCard} key={`card-${fund.fund_code}`}>
            <div className={styles.fundCardTop}>
              <div>
                <h3>{fund.fund_name}</h3>
                <p>{fund.fund_code}</p>
              </div>
              <span className={[styles.changePill, getToneClass(fund.day_growth)].join(' ')}>
                {formatPercent(fund.day_growth)}
              </span>
            </div>
            <dl>
              <div>
                <dt>Month</dt>
                <dd className={getChangeClass(fund.month_growth)}>
                  {formatPercent(fund.month_growth)}
                </dd>
              </div>
              <div>
                <dt>Year</dt>
                <dd className={getChangeClass(fund.year_growth)}>
                  {formatPercent(fund.year_growth)}
                </dd>
              </div>
            </dl>
          </article>
        ))}
      </section>
    </DashboardShell>
  );
}
