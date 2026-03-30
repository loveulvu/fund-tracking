import Particles from '../components/Particles';
import Link from 'next/link';
import PillNav from '../components/PillNav';
import styles from '../../styles/Home.module.css';
import { useState, useEffect } from 'react';

export default function Home() {
  const [funds, setFunds] = useState([]);
  const [loading, setLoading] = useState(true);
  const [user, setUser] = useState(null);

  // 导航项
  const navItems = [
    { label: 'Home', href: '/' },
    { label: 'Funds', href: '/about' },
    { label: user ? 'Profile' : 'Login', href: user ? '/profile' : '/login' },
  ];

  // 模拟用户数据
  useEffect(() => {
    // 检查本地存储中的用户信息
    const savedUser = localStorage.getItem('user');
    if (savedUser !== null && savedUser !== undefined) {
      setUser(JSON.parse(savedUser));
    }
  }, []);

  // 加载基金数据
  useEffect(() => {
    const fetchFunds = async () => {
      try {
        const controller = new AbortController();
        const timeoutId = setTimeout(() => controller.abort(), 5000); // 5秒超时
        
        const response = await fetch('http://localhost:3001/api/funds/all', {
          method: 'GET',
          headers: {
            'Content-Type': 'application/json',
          },
          signal: controller.signal
        });
        
        clearTimeout(timeoutId);
        
        if (response.ok) {
          const data = await response.json();
          setFunds(data.slice(0, 10)); // 只显示前10个
        } else {
          throw new Error(`HTTP error! status: ${response.status}`);
        }
      } catch (error) {
        console.error('Error fetching funds:', error.message);
        // 即使后端服务器没有运行，也显示模拟数据
        setFunds([
          { fund_code: '000001', fund_name: '华夏成长混合', one_month_profit: '5.23%', one_year_profit: '21.56%' },
          { fund_code: '000002', fund_name: '华夏债券A', one_month_profit: '1.23%', one_year_profit: '5.67%' },
          { fund_code: '000003', fund_name: '华夏现金增利A', one_month_profit: '0.34%', one_year_profit: '1.89%' },
          { fund_code: '000004', fund_name: '华夏回报混合A', one_month_profit: '3.45%', one_year_profit: '15.67%' },
          { fund_code: '000005', fund_name: '华夏大盘精选混合', one_month_profit: '6.78%', one_year_profit: '28.90%' }
        ]);
      } finally {
        setLoading(false);
      }
    };

    fetchFunds();
  }, []);

  return (
    <div className={styles.container}>
      {/* 粒子背景层 */}
      <div style={{
        width: '100%',
        height: '100%',
        position: 'absolute',
        top: 0,
        left: 0,
        zIndex: 0
      }}>
        <Particles
          particleColors={['#ffffff']}
          particleCount={200}
          particleSpread={10}
          speed={0.1}
          particleBaseSize={100}
          moveParticlesOnHover={true}
          alphaParticles={true}
          disableRotation={false}
          pixelRatio={1}
        />
      </div>

      {/* 导航栏 */}
      <PillNav
        logo="https://trae-api-cn.mchost.guru/api/ide/v1/text_to_image?prompt=fund%20tracking%20logo%20minimal%20design&image_size=square"
        logoAlt="Fund Tracking Logo"
        items={navItems}
        activeHref="/"
        baseColor="#000000"
        pillColor="#ffffff"
        hoveredPillTextColor="#ffffff"
        pillTextColor="#000000"
        initialLoadAnimation={false}
      />

      {/* 内容层，必须在粒子层上面 */}
      <div style={{ position: 'relative', zIndex: 1, paddingTop: '100px' }}>
        <h1 className={styles.title}>Fund Tracking System</h1>
        <p className={styles.description}>
          实时追踪基金数据，智能分析投资机会
        </p>

        {/* 基金市场概览 */}
        <section className={styles.section}>
          <h2 className={styles.sectionTitle}>基金市场概览</h2>
          {loading ? (
            <p className={styles.loading}>Loading funds data...</p>
          ) : funds.length > 0 ? (
            <div className={styles.fundsGrid}>
              {funds.map((fund, index) => (
                <div key={fund.fund_code} className={styles.fundCard}>
                  <h3 className={styles.fundName}>{fund.fund_name}</h3>
                  <p className={styles.fundCode}>{fund.fund_code}</p>
                  <div className={styles.fundProfit}>
                    <span className={styles.profitLabel}>近1月:</span>
                    <span className={[
                      styles.profitValue,
                      fund.one_month_profit && fund.one_month_profit.includes('-') ? styles.negative : styles.positive
                    ].join(' ')}>
                      {fund.one_month_profit || 'N/A'}
                    </span>
                  </div>
                  <div className={styles.fundProfit}>
                    <span className={styles.profitLabel}>近1年:</span>
                    <span className={[
                      styles.profitValue,
                      fund.one_year_profit && fund.one_year_profit.includes('-') ? styles.negative : styles.positive
                    ].join(' ')}>
                      {fund.one_year_profit || 'N/A'}
                    </span>
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <p className={styles.error}>Failed to load funds data</p>
          )}
        </section>

        {/* 功能介绍 */}
        <section className={styles.section}>
          <h2 className={styles.sectionTitle}>核心功能</h2>
          <div className={styles.featuresGrid}>
            <div className={styles.featureCard}>
              <h3>实时数据</h3>
              <p>定时从天天基金网抓取最新数据</p>
            </div>
            <div className={styles.featureCard}>
              <h3>智能分析</h3>
              <p>自动分析基金涨跌幅，提供投资建议</p>
            </div>
            <div className={styles.featureCard}>
              <h3>涨幅提醒</h3>
              <p>设置阈值，涨幅超过时自动发送邮件</p>
            </div>
            <div className={styles.featureCard}>
              <h3>个性化追踪</h3>
              <p>标记关注的基金，实时监控动态</p>
            </div>
          </div>
        </section>

        <div className={styles.cta}>
          <Link href="/about" className={styles.ctaButton}>
            查看全部基金
          </Link>
        </div>
      </div>
    </div>
  );
}
