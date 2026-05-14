import { useEffect, useMemo, useState } from 'react';
import Link from 'next/link';
import api from '../lib/api';
import styles from '../../styles/Dashboard.module.css';

const NAV_ITEMS = [
  { label: '总览', href: '/', active: true },
  { label: '基金列表', href: '/about' },
  { label: '登录', href: '/login' },
];

const DETAIL_FIELDS = [
  { label: '基金名称', key: 'fund_name' },
  { label: '基金代码', key: 'fund_code' },
  { label: '基金类型', key: 'fund_type' },
  { label: '基金公司', key: 'fund_company' },
  { label: '基金经理', key: 'fund_manager' },
  { label: '基金规模', key: 'fund_scale' },
  { label: '当前净值', key: 'net_value' },
  { label: '净值日期', key: 'net_value_date' },
  { label: '日涨跌幅', key: 'day_growth', format: formatPercent },
  { label: '近1周收益', key: 'week_growth', format: formatPercent },
  { label: '近1月收益', key: 'month_growth', format: formatPercent },
  { label: '近3月收益', key: 'three_month_growth', format: formatPercent },
  { label: '近6月收益', key: 'six_month_growth', format: formatPercent },
  { label: '近1年收益', key: 'year_growth', format: formatPercent },
  { label: '近3年收益', key: 'three_year_growth', format: formatPercent },
  { label: '更新时间', key: 'update_time', format: formatTimestamp },
];

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

function getToneClass(value) {
  const number = Number(value);
  if (!Number.isFinite(number) || number === 0) return styles.toneNeutral;
  return number > 0 ? styles.tonePositive : styles.toneNegative;
}

function StatCard({ label, value, hint, tone = 'blue', blocked = false }) {
  return (
    <article className={[styles.statCard, blocked ? styles.statCardMuted : ''].join(' ')}>
      <div className={[styles.statIcon, styles[`statIcon_${tone}`]].join(' ')} aria-hidden="true" />
      <div>
        <span>{label}</span>
        <strong>{value}</strong>
        <p>{hint}</p>
      </div>
    </article>
  );
}

function EmptyState({ title, message }) {
  return (
    <div className={styles.emptyState}>
      <div className={styles.emptyMark} aria-hidden="true" />
      <strong>{title}</strong>
      <p>{message}</p>
    </div>
  );
}

function FundDetailPanel({ fund, onClose }) {
  if (!fund) return null;

  return (
    <div className={styles.detailLayer} role="presentation">
      <button
        className={styles.detailBackdrop}
        type="button"
        aria-label="关闭基金详情"
        onClick={onClose}
      />
      <aside className={styles.detailPanel} aria-label="基金详情">
        <div className={styles.detailHeader}>
          <div>
            <span>Fund details</span>
            <h2>{formatValue(fund.fund_name)}</h2>
            <p>{formatValue(fund.fund_code)}</p>
          </div>
          <button className={styles.closeButton} type="button" onClick={onClose}>
            关闭
          </button>
        </div>

        <div className={styles.detailSummary}>
          <div>
            <span>当前净值</span>
            <strong>{formatValue(fund.net_value)}</strong>
            <p>{formatValue(fund.net_value_date)}</p>
          </div>
          <div>
            <span>日涨跌幅</span>
            <strong className={getChangeClass(fund.day_growth)}>
              {formatPercent(fund.day_growth)}
            </strong>
            <p>来自 /api/funds</p>
          </div>
        </div>

        <dl className={styles.detailList}>
          {DETAIL_FIELDS.map((field) => (
            <div key={field.key}>
              <dt>{field.label}</dt>
              <dd>
                {field.format
                  ? field.format(fund[field.key])
                  : formatValue(fund[field.key])}
              </dd>
            </div>
          ))}
        </dl>

        <div className={styles.detailNote}>
          <strong>需要接入持仓数据</strong>
          <p>总资产、持仓金额、资产配置图和投资组合曲线当前没有真实数据来源。</p>
        </div>
      </aside>
    </div>
  );
}

export default function Home() {
  const [funds, setFunds] = useState([]);
  const [query, setQuery] = useState('');
  const [selectedFund, setSelectedFund] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [isLoggedIn] = useState(() => (
    typeof window !== 'undefined' && Boolean(localStorage.getItem('token'))
  ));

  useEffect(() => {
    let active = true;

    async function fetchFunds() {
      try {
        setLoading(true);
        setError('');

        const data = await api.getFunds();

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
      const company = String(fund.fund_company || '').toLowerCase();
      return (
        code.includes(keyword) ||
        name.includes(keyword) ||
        type.includes(keyword) ||
        company.includes(keyword)
      );
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

  const positiveCount = useMemo(() => {
    return funds.filter((fund) => Number(fund.day_growth) > 0).length;
  }, [funds]);

  const topDailyFunds = useMemo(() => {
    return [...funds]
      .filter((fund) => Number.isFinite(Number(fund.day_growth)))
      .sort((a, b) => Number(b.day_growth) - Number(a.day_growth))
      .slice(0, 4);
  }, [funds]);

  const featuredFunds = filteredFunds.slice(0, 4);

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
          {NAV_ITEMS.map((item) => {
            const navItem = item.href === '/login' && isLoggedIn
              ? { ...item, label: '账户', href: '/profile' }
              : item;

            return (
            <Link
              key={navItem.href}
              href={navItem.href}
              className={item.active ? styles.navActive : ''}
            >
              <span className={styles.navDot} aria-hidden="true" />
              {navItem.label}
            </Link>
            );
          })}
        </nav>

        <div className={styles.sidebarNote}>
          <span>持仓数据</span>
          <strong>需要接入持仓数据</strong>
          <p>总资产、收益和配置图暂不展示真实数值。</p>
        </div>
      </aside>

      <section className={styles.content}>
        <header className={styles.topbar}>
          <label className={styles.search}>
            <span aria-hidden="true" />
            <input
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder="搜索基金名称、代码、公司或类型"
            />
          </label>
          <div className={[styles.status, error ? styles.statusError : ''].join(' ')}>
            {loading ? '加载中' : error ? '接口异常' : '真实接口数据'}
          </div>
        </header>

        <section className={styles.hero}>
          <div>
            <p className={styles.eyebrow}>基金总览</p>
            <h1>基金追踪概览</h1>
            <p>
              基于现有 <code>/api/funds</code> 接口展示基金净值和阶段表现。
              持仓、资产和收益类模块保持占位，不模拟真实资产。
            </p>
          </div>
          <div className={styles.heroMeta}>
            <span>最新更新时间</span>
            <strong>{formatTimestamp(latestUpdate)}</strong>
          </div>
        </section>

        <section className={styles.statsGrid}>
          <StatCard
            label="基金数量"
            value={loading ? '加载中' : funds.length}
            hint="来自 /api/funds"
            tone="blue"
          />
          <StatCard
            label="平均日涨跌幅"
            value={averageDayChange === null ? '暂无数据' : formatPercent(averageDayChange)}
            hint="按当前基金列表简单平均"
            tone={Number(averageDayChange) >= 0 ? 'green' : 'red'}
          />
          <StatCard
            label="上涨基金"
            value={loading ? '加载中' : positiveCount}
            hint="基于 day_growth 统计"
            tone="green"
          />
          <StatCard
            label="最新更新时间"
            value={formatTimestamp(latestUpdate)}
            hint="基于 update_time"
            tone="blue"
          />
        </section>

        <section className={styles.mainGrid}>
          <section className={styles.mainColumn}>
            <article className={styles.chartPanel}>
              <div className={styles.panelHeader}>
                <div>
                  <h2>组合收益趋势</h2>
                  <p>需要接入持仓份额和历史净值快照后展示</p>
                </div>
                <span className={styles.panelBadgeMuted}>暂未接入</span>
              </div>
              <div className={[styles.chartPlaceholder, styles.trendEmpty].join(' ')}>
                <div className={styles.placeholderCopy}>
                  <strong>暂无组合收益数据</strong>
                  <p>当前系统只展示基金净值和阶段表现，暂未记录用户持仓份额与历史收益曲线。</p>
                </div>
              </div>
            </article>

            <article className={styles.panel}>
              <div className={styles.panelHeader}>
                <div>
                  <h2>基金列表</h2>
                  <p>{filteredFunds.length} 条匹配结果</p>
                </div>
                <span className={styles.panelBadge}>实时接口</span>
              </div>

              {loading ? (
                <div className={styles.loadingList} aria-label="正在加载基金数据">
                  <span />
                  <span />
                  <span />
                  <span />
                </div>
              ) : error ? (
                <EmptyState title="暂无数据" message={error} />
              ) : filteredFunds.length === 0 ? (
                <EmptyState title="暂无数据" message="没有找到匹配的基金，请换个关键词试试。" />
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
                        <th>操作</th>
                      </tr>
                    </thead>
                    <tbody>
                      {filteredFunds.map((fund) => (
                        <tr
                          key={fund.fund_code || fund.fund_name}
                          className={styles.clickableRow}
                          onClick={() => setSelectedFund(fund)}
                        >
                          <td>
                            <div className={styles.fundIdentity}>
                              <span>{String(fund.fund_name || '基金').slice(0, 1)}</span>
                              <div>
                                <strong>{formatValue(fund.fund_name)}</strong>
                                <small>{formatValue(fund.fund_company)}</small>
                              </div>
                            </div>
                          </td>
                          <td>{formatValue(fund.fund_code)}</td>
                          <td>{formatValue(fund.net_value)}</td>
                          <td>
                            <span className={[styles.changePill, getToneClass(fund.day_growth)].join(' ')}>
                              {formatPercent(fund.day_growth)}
                            </span>
                          </td>
                          <td className={getChangeClass(fund.month_growth)}>
                            {formatPercent(fund.month_growth)}
                          </td>
                          <td className={getChangeClass(fund.year_growth)}>
                            {formatPercent(fund.year_growth)}
                          </td>
                          <td>
                            <span className={styles.typeBadge}>{formatValue(fund.fund_type)}</span>
                          </td>
                          <td>
                            <div className={styles.rowActions}>
                            <button
                              className={styles.detailButton}
                              type="button"
                              onClick={(event) => {
                                event.stopPropagation();
                                setSelectedFund(fund);
                              }}
                            >
                              快速查看
                            </button>
                              <Link
                                className={styles.detailLink}
                                href={fund.fund_code ? `/fund/${fund.fund_code}` : '#'}
                                onClick={(event) => {
                                  event.stopPropagation();
                                  if (!fund.fund_code) {
                                    event.preventDefault();
                                  }
                                }}
                              >
                                详情页
                              </Link>
                            </div>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </article>
          </section>

          <aside className={styles.sideColumn}>
            <article className={styles.panel}>
              <div className={styles.panelHeader}>
                <div>
                  <h2>日涨跌幅靠前</h2>
                  <p>基于 /api/funds 的 day_growth</p>
                </div>
              </div>

              {loading ? (
                <div className={styles.loadingCards}>
                  <span />
                  <span />
                  <span />
                </div>
              ) : topDailyFunds.length === 0 ? (
                <EmptyState title="暂无数据" message="接口未返回可计算的日涨跌幅。" />
              ) : (
                <div className={styles.topFundList}>
                  {topDailyFunds.map((fund, index) => (
                    <button
                      key={fund.fund_code || fund.fund_name}
                      className={styles.topFundItem}
                      type="button"
                      onClick={() => setSelectedFund(fund)}
                    >
                      <span className={styles.rankBadge}>{index + 1}</span>
                      <span>
                        <strong>{formatValue(fund.fund_name)}</strong>
                        <small>{formatValue(fund.fund_code)}</small>
                      </span>
                      <em className={getChangeClass(fund.day_growth)}>
                        {formatPercent(fund.day_growth)}
                      </em>
                    </button>
                  ))}
                </div>
              )}
            </article>

            <article className={styles.panel}>
              <div className={styles.panelHeader}>
                <div>
                  <h2>基础信息</h2>
                  <p>前 4 只匹配基金</p>
                </div>
              </div>

              {loading ? (
                <div className={styles.loadingCards}>
                  <span />
                  <span />
                  <span />
                </div>
              ) : featuredFunds.length === 0 ? (
                <EmptyState title="暂无数据" message="基金基础信息来自 /api/funds。" />
              ) : (
                <div className={styles.fundCards}>
                  {featuredFunds.map((fund) => (
                    <article key={fund.fund_code || fund.fund_name} className={styles.fundCard}>
                      <div className={styles.fundCardTop}>
                        <div>
                          <h3>{formatValue(fund.fund_name)}</h3>
                          <p>{formatValue(fund.fund_code)}</p>
                        </div>
                        <span className={[styles.changePill, getToneClass(fund.day_growth)].join(' ')}>
                          {formatPercent(fund.day_growth)}
                        </span>
                      </div>
                      <dl>
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
            </article>

            <article className={styles.placeholderPanel}>
              <div>
                <span>Asset Allocation</span>
                <strong>需要接入持仓数据</strong>
                <p>需要接入持仓金额、基金分类和用户份额后展示资产配置。</p>
              </div>
            </article>

            <article className={styles.sourcePanel}>
              <span>数据来源</span>
              <strong>GET /api/funds</strong>
              <p>本页行情、统计、详情和榜单均来自当前基金列表接口；持仓类数据源未接入。</p>
            </article>
          </aside>
        </section>
      </section>
      <FundDetailPanel fund={selectedFund} onClose={() => setSelectedFund(null)} />
    </main>
  );
}
