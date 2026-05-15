import { useCallback, useEffect, useState } from 'react';
import Link from 'next/link';
import DashboardShell from '../components/DashboardShell';
import api from '../lib/api';
import styles from '../../styles/Dashboard.module.css';

function formatLastUpdated(value) {
  const timestampMs = parseTimestampMs(value);
  if (timestampMs === null) return '暂无数据';

  return new Date(timestampMs).toLocaleString();
}

function parseTimestampMs(value) {
  if (value === null || value === undefined) return null;
  if (typeof value === 'string' && (value.trim() === '' || value.trim() === '0')) {
    return null;
  }
  if (value === 0) return null;

  const numeric = Number(value);
  if (Number.isFinite(numeric) && numeric > 0) {
    return numeric < 1000000000000 ? numeric * 1000 : numeric;
  }

  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return null;

  return date.getTime();
}

function formatPercent(value) {
  if (value === null || value === undefined) return '-';
  if (typeof value === 'string' && value.trim() === '') return '-';

  const number = Number(value);
  if (!Number.isFinite(number)) return '-';
  return `${number > 0 ? '+' : ''}${number.toFixed(2)}%`;
}

function getChangeClass(value) {
  if (value === null || value === undefined) return styles.neutral;
  if (typeof value === 'string' && value.trim() === '') return styles.neutral;

  const number = Number(value);
  if (!Number.isFinite(number) || number === 0) return styles.neutral;
  return number > 0 ? styles.positive : styles.negative;
}

function getToneClass(value) {
  if (value === null || value === undefined) return styles.toneNeutral;
  if (typeof value === 'string' && value.trim() === '') return styles.toneNeutral;

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
  const [lastUpdatedText, setLastUpdatedText] = useState('暂无数据');

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
      alert('请先登录。');
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
        alert(data.error || '关注失败');
      }
    } catch (err) {
      console.error('Error adding to watchlist:', err);
      alert('关注失败');
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
        alert(data.error || '取消关注失败');
      }
    } catch (err) {
      console.error('Error removing from watchlist:', err);
      alert('取消关注失败');
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
              const next = parseTimestampMs(item?.update_time);
              return next !== null && next > current ? next : current;
            }, 0);
            setLastUpdatedText(formatLastUpdated(latest));
          } else {
            setLastUpdatedText('暂无数据');
          }
        } else {
          console.error('Invalid data format:', data);
          setError('接口返回的数据格式不正确。');
        }
        setError(null);
      } catch (err) {
        setError(`基金数据加载失败：${err.message}`);
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
        if (response.status === 404) {
          setFilteredFunds([]);
          setError(null);
          return;
        }

        if (!response.ok) {
          throw new Error('基金数据请求失败');
        }
        const data = await response.json();

        if (!data || !data.fund_code) {
          throw new Error('接口返回的基金数据无效');
        }

        setFilteredFunds([data]);
        setError(null);
      } catch (err) {
        setError(`基金数据加载失败：${err.message}`);
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
        setError('搜索接口暂时不可用，已显示本地匹配结果。');
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
      noteTitle="基金列表"
      noteText="搜索当前数据库已收录的基金，并把常看的基金加入关注列表。"
    >
      <header className={styles.pageHeader}>
        <div>
          <p className={styles.eyebrow}>基金</p>
          <h1>基金列表</h1>
          <p>搜索当前数据库已收录的基金，并管理关注状态。</p>
        </div>
        <div className={styles.heroMeta}>
          <span>最近更新</span>
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
            placeholder="输入基金名称或 6 位基金代码"
          />
        </label>
        <button className={styles.primaryButton} type="submit">
          搜索
        </button>
        <button className={styles.secondaryButton} type="button" onClick={handleClearSearch}>
          清空
        </button>
      </form>

      {error && <div className={styles.messageBox}>{error}</div>}

      <article className={styles.panel}>
        <div className={styles.panelHeader}>
          <div>
            <h2>基金列表</h2>
            <p>{loading ? '正在加载基金数据' : `${filteredFunds.length} 条结果`}</p>
          </div>
          <span className={styles.panelBadge}>实时接口</span>
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
            <strong>当前数据库暂无匹配基金</strong>
            <p>请尝试已收录的基金名称或 6 位基金代码。</p>
          </div>
        ) : (
          <div className={styles.tableWrap}>
            <table className={styles.table}>
              <thead>
                <tr>
                  <th>基金</th>
                  <th>代码</th>
                  <th>净值</th>
                  <th>日涨跌</th>
                  <th>净值日期</th>
                  <th>关注</th>
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
                            {watchlistLoading[fund.fund_code] ? '处理中...' : '取消关注'}
                          </button>
                        ) : (
                          <button
                            className={styles.detailButton}
                            type="button"
                            onClick={() => handleWatch(fund)}
                            disabled={watchlistLoading[fund.fund_code]}
                          >
                            {watchlistLoading[fund.fund_code] ? '处理中...' : '关注'}
                          </button>
                        )
                      ) : (
                        <Link className={styles.detailLink} href="/login">
                          登录后关注
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
                <dt>近 1 月</dt>
                <dd className={getChangeClass(fund.month_growth)}>
                  {formatPercent(fund.month_growth)}
                </dd>
              </div>
              <div>
                <dt>近 1 年</dt>
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
