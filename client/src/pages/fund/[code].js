import { useEffect, useMemo, useState } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/router';
import api from '../../lib/api';
import styles from '../../../styles/Dashboard.module.css';

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

function getCodeFromPath(asPath) {
  const path = String(asPath || '').split('?')[0].split('#')[0];
  const [, maybeCode] = path.split('/').filter(Boolean);

  if (maybeCode && maybeCode !== '[code]') {
    return decodeURIComponent(maybeCode).trim();
  }

  if (typeof window !== 'undefined') {
    const [, codeFromLocation] = window.location.pathname.split('/').filter(Boolean);
    return decodeURIComponent(codeFromLocation || '').trim();
  }

  return '';
}

export default function FundDetailPage() {
  const router = useRouter();
  const code = useMemo(() => {
    const queryCode = Array.isArray(router.query.code)
      ? router.query.code[0]
      : router.query.code;

    return String(queryCode || getCodeFromPath(router.asPath)).trim();
  }, [router.asPath, router.query.code]);
  const [funds, setFunds] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    if (!router.isReady && !code) return;

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
  }, [router.isReady, code]);

  const fund = useMemo(() => {
    return funds.find((item) => String(item.fund_code || '').trim() === code) || null;
  }, [funds, code]);

  return (
    <main className={styles.fundPageShell}>
      <section className={styles.fundPage}>
        <header className={styles.fundPageTopbar}>
          <Link href="/" className={styles.backLink}>
            返回 Dashboard
          </Link>
          <span>数据源：GET /api/funds</span>
        </header>

        {loading ? (
          <div className={styles.fundPageState}>正在加载基金详情</div>
        ) : error ? (
          <div className={styles.fundPageState}>接口异常：{error}</div>
        ) : !fund ? (
          <div className={styles.fundPageState}>未找到该基金</div>
        ) : (
          <>
            <section className={styles.fundHero}>
              <div>
                <p className={styles.eyebrow}>Fund Detail</p>
                <h1>{formatValue(fund.fund_name)}</h1>
                <p>
                  {formatValue(fund.fund_code)} · {formatValue(fund.fund_type)}
                </p>
              </div>
              <div className={styles.fundHeroMetric}>
                <span>当前净值</span>
                <strong>{formatValue(fund.net_value)}</strong>
                <p>{formatValue(fund.net_value_date)}</p>
              </div>
            </section>

            <section className={styles.fundMetricGrid}>
              <article>
                <span>日涨跌幅</span>
                <strong className={getChangeClass(fund.day_growth)}>
                  {formatPercent(fund.day_growth)}
                </strong>
              </article>
              <article>
                <span>近1月收益</span>
                <strong className={getChangeClass(fund.month_growth)}>
                  {formatPercent(fund.month_growth)}
                </strong>
              </article>
              <article>
                <span>近1年收益</span>
                <strong className={getChangeClass(fund.year_growth)}>
                  {formatPercent(fund.year_growth)}
                </strong>
              </article>
            </section>

            <section className={styles.fundDetailGrid}>
              <article className={styles.panel}>
                <div className={styles.panelHeader}>
                  <div>
                    <h2>基金详情</h2>
                    <p>全部字段来自 /api/funds</p>
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
              </article>

              <aside className={styles.fundAside}>
                <article className={styles.placeholderPanel}>
                  <div>
                    <span>Portfolio data</span>
                    <strong>需要接入持仓数据</strong>
                    <p>当前不展示总资产、持仓金额、收益金额、组合曲线或资产配置图。</p>
                  </div>
                </article>
                <article className={styles.sourcePanel}>
                  <span>Data source</span>
                  <strong>GET /api/funds</strong>
                  <p>本页从基金列表接口中按 fund_code 匹配当前基金，没有新增接口或 mock 数据。</p>
                </article>
              </aside>
            </section>
          </>
        )}
      </section>
    </main>
  );
}
