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
  if (value === null || value === undefined) return '暂无数据';
  if (typeof value === 'string' && value.trim() === '') return '暂无数据';

  const number = Number(value);
  if (!Number.isFinite(number)) return '暂无数据';
  return `${number > 0 ? '+' : ''}${number.toFixed(2)}%`;
}

function formatValue(value) {
  if (value === null || value === undefined) return '暂无数据';
  if (typeof value === 'string') {
    const trimmed = value.trim();
    if (trimmed === '' || trimmed === '0') return '暂无数据';
    return trimmed;
  }
  if (value === 0) return '暂无数据';
  return value;
}

function formatTimestamp(value) {
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

  const number = Number(value);
  if (Number.isFinite(number) && number > 0) {
    return number < 1000000000000 ? number * 1000 : number;
  }

  const date = new Date(value);
  if (!Number.isNaN(date.getTime())) {
    return date.getTime();
  }

  return null;
}

function getChangeClass(value) {
  if (value === null || value === undefined) return styles.neutral;
  if (typeof value === 'string' && value.trim() === '') return styles.neutral;

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
  const [fund, setFund] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [notFound, setNotFound] = useState(false);

  useEffect(() => {
    if (!router.isReady && !code) return;

    let active = true;

    async function fetchFundDetail() {
      try {
        setLoading(true);
        setError('');
        setNotFound(false);

        const response = await api.getFund(code);

        if (response.status === 404) {
          if (active) {
            setFund(null);
            setNotFound(true);
          }
          return;
        }

        if (!response.ok) {
          throw new Error(`接口返回 ${response.status}`);
        }

        const data = await response.json();

        if (active) {
          if (data?.fund_code) {
            setFund(data);
          } else {
            setFund(null);
            setNotFound(true);
          }
        }
      } catch (err) {
        if (active) {
          setFund(null);
          setError(err.message || '基金数据加载失败');
        }
      } finally {
        if (active) {
          setLoading(false);
        }
      }
    }

    fetchFundDetail();

    return () => {
      active = false;
    };
  }, [router.isReady, code]);

  return (
    <main className={styles.fundPageShell}>
      <section className={styles.fundPage}>
        <header className={styles.fundPageTopbar}>
          <Link href="/" className={styles.backLink}>
            返回 Dashboard
          </Link>
          <span>数据源：GET /api/fund/{code}</span>
        </header>

        {loading ? (
          <div className={styles.fundPageState}>正在加载基金详情</div>
        ) : error ? (
          <div className={styles.fundPageState}>接口异常：{error}</div>
        ) : notFound || !fund ? (
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
                    <p>全部字段来自 /api/fund/{code}</p>
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
                  <strong>GET /api/fund/{code}</strong>
                  <p>本页直接读取单只基金详情；如果当前数据库未收录该代码，会显示未找到该基金。</p>
                </article>
              </aside>
            </section>
          </>
        )}
      </section>
    </main>
  );
}
