import { useEffect, useState } from 'react';
import Link from 'next/link';
import DashboardShell from '../components/DashboardShell';
import api from '../lib/api';
import styles from '../../styles/Dashboard.module.css';

function formatPercent(value) {
  const number = Number(value);
  if (!Number.isFinite(number)) return '暂无行情数据';
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

          let mergedFundData = {};
          try {
            const funds = await api.getFunds();
            if (Array.isArray(funds)) {
              for (const fund of funds) {
                if (fund?.fund_code) {
                  mergedFundData[fund.fund_code] = fund;
                }
              }
              setFundData(mergedFundData);
            }
          } catch (fundErr) {
            console.error('Error fetching funds for watchlist merge:', fundErr);
          }

          for (const item of data) {
            if (!mergedFundData[item.fundCode]) {
              fetchFundData(item.fundCode);
            }
          }
        } else {
          throw new Error('Failed to fetch watchlist');
        }
      } catch (err) {
        setError('无法加载关注列表。');
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
        alert(data.error || '取消关注失败。');
      }
    } catch (err) {
      console.error('Error removing from watchlist:', err);
      alert('取消关注失败。');
    }
  };

  const handleUpdateThreshold = async (fundCode) => {
    if (!newThreshold || Number.isNaN(Number(newThreshold))) {
      alert('请输入有效的提醒阈值。');
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
        alert(data.error || '阈值修改失败。');
      }
    } catch (err) {
      console.error('Error updating threshold:', err);
      alert('阈值修改失败。');
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
      noteTitle="关注列表"
      noteText="集中管理已关注基金和提醒阈值。"
    >
      <header className={styles.pageHeader}>
        <div>
          <p className={styles.eyebrow}>账户</p>
          <h1>账户 / 关注列表</h1>
          <p>查看当前账户信息，并管理关注基金。</p>
        </div>
        <button className={styles.secondaryButton} type="button" onClick={handleLogout}>
          退出登录
        </button>
      </header>

      {error && <div className={styles.messageBox}>{error}</div>}

      <section className={styles.accountGrid}>
        <article className={styles.panel}>
          <div className={styles.panelHeader}>
            <div>
              <h2>用户信息</h2>
              <p>当前登录状态</p>
            </div>
          </div>
          <div className={styles.cardBody}>
            <dl className={styles.infoList}>
              <div>
                <dt>邮箱</dt>
                <dd>{user?.email || 'N/A'}</dd>
              </div>
              <div>
                <dt>关注基金</dt>
                <dd>{watchlist.length}</dd>
              </div>
            </dl>
          </div>
        </article>

        <article className={styles.sourcePanel}>
          <span>数据来源</span>
          <strong>GET /api/watchlist</strong>
          <p>关注列表按登录用户和基金代码定位，避免跨用户误操作。</p>
        </article>
      </section>

      <article className={styles.panel}>
        <div className={styles.panelHeader}>
          <div>
            <h2>关注基金</h2>
            <p>{loading ? '正在加载关注列表' : `${watchlist.length} 只基金`}</p>
          </div>
          <Link className={styles.detailLink} href="/about">
            添加基金
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
            <strong>还没有关注基金</strong>
            <p>前往基金列表添加你想跟踪的基金。</p>
            <Link className={styles.detailLink} href="/about">
              去添加基金
            </Link>
          </div>
        ) : (
          <div className={styles.tableWrap}>
            <table className={styles.table}>
              <thead>
                <tr>
                  <th>基金</th>
                  <th>净值</th>
                  <th>日涨跌</th>
                  <th>近 1 月</th>
                  <th>近 1 年</th>
                  <th>提醒阈值</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                {watchlist.map((item) => {
                  const fund = fundData[item.fundCode];
                  const hasMarketData = Boolean(fund?.fund_code);

                  return (
                    <tr key={item.fundCode}>
                      <td>
                        <div className={styles.fundIdentity}>
                          <span>{String(item.fundName || fund?.fund_name || '基').slice(0, 1)}</span>
                          <div>
                            <strong>{item.fundName || fund?.fund_name || '基金'}</strong>
                            <small>{item.fundCode}</small>
                          </div>
                        </div>
                      </td>
                      <td>
                        {hasMarketData ? (
                          <>
                            <strong>{fund.net_value ?? '暂无行情数据'}</strong>
                            <small className={styles.mutedBlock}>{fund.net_value_date || '暂无日期'}</small>
                          </>
                        ) : (
                          <span className={styles.mutedText}>暂无行情数据</span>
                        )}
                      </td>
                      <td>
                        {hasMarketData ? (
                          <span className={[styles.changePill, getToneClass(fund.day_growth)].join(' ')}>
                            {formatPercent(fund.day_growth)}
                          </span>
                        ) : (
                          <span className={styles.mutedText}>暂无行情数据</span>
                        )}
                      </td>
                      <td className={hasMarketData ? getChangeClass(fund.month_growth) : styles.neutral}>
                        {hasMarketData ? formatPercent(fund.month_growth) : '暂无行情数据'}
                      </td>
                      <td className={hasMarketData ? getChangeClass(fund.year_growth) : styles.neutral}>
                        {hasMarketData ? formatPercent(fund.year_growth) : '暂无行情数据'}
                      </td>
                      <td>
                        {editingThreshold === item.fundCode ? (
                          <div className={styles.thresholdEditorInline}>
                            <input
                              type="number"
                              value={newThreshold}
                              onChange={(event) => setNewThreshold(event.target.value)}
                              placeholder="阈值"
                            />
                            <button
                              className={styles.primaryButtonSmall}
                              type="button"
                              onClick={() => handleUpdateThreshold(item.fundCode)}
                            >
                              保存
                            </button>
                            <button
                              className={styles.secondaryButtonSmall}
                              type="button"
                              onClick={() => {
                                setEditingThreshold(null);
                                setNewThreshold('');
                              }}
                            >
                              取消
                            </button>
                          </div>
                        ) : (
                          <button
                            className={styles.thresholdButton}
                            type="button"
                            onClick={() => {
                              setEditingThreshold(item.fundCode);
                              setNewThreshold(String(item.alertThreshold));
                            }}
                          >
                            {item.alertThreshold}% 修改
                          </button>
                        )}
                        <small className={styles.mutedBlock}>添加于 {formatDate(item.addedAt)}</small>
                      </td>
                      <td>
                        <button
                          className={styles.dangerButton}
                          type="button"
                          onClick={() => handleUnwatch(item.fundCode)}
                        >
                          取消关注
                        </button>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </article>
    </DashboardShell>
  );
}
