import { useEffect, useMemo, useState } from 'react';
import Link from 'next/link';
import api from '../lib/api';
import styles from '../../styles/Dashboard.module.css';

function formatPercent(value) {
  const number = Number(value);
  if (!Number.isFinite(number)) return '暂无数据';
  return `${number > 0 ? '+' : ''}${number.toFixed(2)}%`;
}

function formatValue(value) {
  if (value === null || value === undefined || value === '') return '暂无数据';
  return value;
}

function formatTimestamp(value) {
  const number = Number(value);
  if (!Number.isFinite(number) || number <= 0) return '暂无数据';
  return new Date(number * 1000).toLocaleString();
}

function getChangeClass(value) {
  const number = Number(value);
  if (!Number.isFinite(number) || number === 0) return styles.neutral;
  return number > 0 ? styles.positive : styles.negative;
}

export default function Home() {
  const [funds, setFunds] = useState([]);
  const [query, setQuery] = useState('');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    let active = true;

    async function fetchFunds() {
      try {
        setLoading(true);
        setError('');

        const response = await api.getFunds();
        if (!response.ok) {
          throw new Error(`接口返回 ${response.status}`);
        }

        const data = await response.json();
        if (!Array.isArray(data)) {
          throw new Error('接口返回格式不是基金数组');
        }

        if (active) {
          setFunds(data);
        }
      } catch (err) {
        if (active) {
          setFunds([]);
          setError(err.message || '基金数据加载失败');
        }
      } finally {
        if (active) {
          setLoading(false);
        }
      }
    }

    fetchFunds();

    return () => {
      active = false;
    };
  }, []);

  const filteredFunds = useMemo(() => {
    const keyword = query.trim().toLowerCase();
    if (!keyword) return funds;

    return funds.filter((fund) => {
      const code = String(fund.fund_code || '').toLowerCase();
      const name = String(fund.fund_name || '').toLowerCase();
      const type = String(fund.fund_type || '').toLowerCase();
      return code.includes(keyword) || name.includes(keyword) || type.includes(keyword);
    });
  }, [funds, query]);

  const latestUpdate = useMemo(() => {
    return funds.reduce((latest, fund) => {
      const value = Number(fund.update_time || 0);
      return value > latest ? value : latest;
    }, 0);
  }, [funds]);

  const averageDayChange = useMemo(() => {
    const values = funds
      .map((fund) => Number(fund.day_growth))
      .filter((value) => Number.isFinite(value));

    if (values.length === 0) return null;
    return values.reduce((sum, value) => sum + value, 0) / values.length;
  }, [funds]);

  const watchedCount = useMemo(() => {
    return funds.filter((fund) => fund.is_watched).length;
  }, [funds]);

  const featuredFunds = filteredFunds.slice(0, 3);

  return (
    <main className={styles.shell}>
      <aside className={styles.sidebar}>
        <div className={styles.brand}>
          <span className={styles.brandMark}>FT</span>
          <span>FundTracking</span>
        </div>

        <nav className={styles.nav}>
          <Link href="/" className={styles.navActive}>Dashboard</Link>
          <Link href="/about">Funds</Link>
          <Link href="/profile">Watchlist</Link>
          <Link href="/login">Login</Link>
        </nav>
      </aside>

      <section className={styles.content}>
        <header className={styles.topbar}>
          <label className={styles.search}>
            <span>Search</span>
            <input
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder="输入基金名称、代码或类型"
            />
          </label>
          <div className={styles.status}>
            {loading ? '加载中' : error ? '接口异常' : '真实接口数据'}
          </div>
        </header>

        <section className={styles.heading}>
          <div>
            <p className={styles.eyebrow}>Dashboard</p>
            <h1>基金追踪概览</h1>
            <p>当前页面只展示后端已有接口能返回的数据，持仓和资产收益暂不模拟。</p>
          </div>
        </section>

        <section className={styles.cards}>
          <article className={styles.card}>
            <span>基金数量</span>
            <strong>{loading ? '加载中' : funds.length}</strong>
            <p>来自 /api/funds</p>
          </article>
          <article className={styles.card}>
            <span>平均日涨跌幅</span>
            <strong className={getChangeClass(averageDayChange)}>
              {averageDayChange === null ? '暂无数据' : formatPercent(averageDayChange)}
            </strong>
            <p>按当前列表简单平均</p>
          </article>
          <article className={styles.card}>
            <span>关注数量</span>
            <strong>{watchedCount > 0 ? watchedCount : '暂无数据'}</strong>
            <p>未登录时后端不会返回个人关注</p>
          </article>
          <article className={styles.card}>
            <span>资产与收益</span>
            <strong>需要接入持仓数据</strong>
            <p>不伪造总资产或收益金额</p>
          </article>
        </section>

        <section className={styles.grid}>
          <article className={styles.panel}>
            <div className={styles.panelHeader}>
              <div>
                <h2>基金列表</h2>
                <p>最新更新时间：{formatTimestamp(latestUpdate)}</p>
              </div>
              <span>{filteredFunds.length} 条</span>
            </div>

            {loading ? (
              <div className={styles.empty}>正在加载基金数据</div>
            ) : error ? (
              <div className={styles.empty}>暂无数据：{error}</div>
            ) : filteredFunds.length === 0 ? (
              <div className={styles.empty}>暂无数据</div>
            ) : (
              <div className={styles.tableWrap}>
                <table className={styles.table}>
                  <thead>
                    <tr>
                      <th>基金</th>
                      <th>代码</th>
                      <th>净值</th>
                      <th>日涨跌</th>
                      <th>近1月</th>
                      <th>近1年</th>
                      <th>类型</th>
                    </tr>
                  </thead>
                  <tbody>
                    {filteredFunds.map((fund) => (
                      <tr key={fund.fund_code}>
                        <td>{formatValue(fund.fund_name)}</td>
                        <td>{formatValue(fund.fund_code)}</td>
                        <td>{formatValue(fund.net_value)}</td>
                        <td className={getChangeClass(fund.day_growth)}>
                          {formatPercent(fund.day_growth)}
                        </td>
                        <td className={getChangeClass(fund.month_growth)}>
                          {formatPercent(fund.month_growth)}
                        </td>
                        <td className={getChangeClass(fund.year_growth)}>
                          {formatPercent(fund.year_growth)}
                        </td>
                        <td>{formatValue(fund.fund_type)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </article>

          <aside className={styles.panel}>
            <div className={styles.panelHeader}>
              <div>
                <h2>基础信息</h2>
                <p>前 3 只匹配基金</p>
              </div>
            </div>

            {loading ? (
              <div className={styles.empty}>正在加载基础信息</div>
            ) : featuredFunds.length === 0 ? (
              <div className={styles.empty}>暂无数据</div>
            ) : (
              <div className={styles.fundCards}>
                {featuredFunds.map((fund) => (
                  <article key={fund.fund_code} className={styles.fundCard}>
                    <div>
                      <h3>{formatValue(fund.fund_name)}</h3>
                      <p>{formatValue(fund.fund_code)}</p>
                    </div>
                    <dl>
                      <div>
                        <dt>基金公司</dt>
                        <dd>{formatValue(fund.fund_company)}</dd>
                      </div>
                      <div>
                        <dt>基金经理</dt>
                        <dd>{formatValue(fund.fund_manager)}</dd>
                      </div>
                      <div>
                        <dt>基金规模</dt>
                        <dd>{formatValue(fund.fund_scale)}</dd>
                      </div>
                    </dl>
                  </article>
                ))}
              </div>
            )}
          </aside>
        </section>
      </section>
    </main>
  );
}
