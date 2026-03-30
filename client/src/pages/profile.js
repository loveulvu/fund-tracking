import { useState, useEffect } from 'react';
import PillNav from '../components/PillNav';
import styles from '../../styles/Home.module.css';

export default function Profile() {
  const [user, setUser] = useState(null);
  const [watchlist, setWatchlist] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  // 导航项
  const navItems = [
    { label: 'Home', href: '/' },
    { label: 'Funds', href: '/about' },
    { label: '账户', href: '/profile' },
  ];

  // 获取用户信息
  useEffect(() => {
    const savedUser = localStorage.getItem('user');
    if (savedUser) {
      setUser(JSON.parse(savedUser));
    } else {
      // 未登录，重定向到登录页
      window.location.href = '/login';
    }
  }, []);

  // 获取关注列表
  useEffect(() => {
    const fetchWatchlist = async () => {
      if (!user) return;

      try {
        const token = localStorage.getItem('token');
        if (!token) {
          throw new Error('No token found');
        }

        const response = await fetch('http://localhost:3001/api/watchlist', {
          method: 'GET',
          headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${token}`,
          },
        });

        if (response.ok) {
          const data = await response.json();
          setWatchlist(data);
        } else {
          throw new Error('Failed to fetch watchlist');
        }
      } catch (err) {
        setError('无法获取关注列表');
        console.error('Error fetching watchlist:', err);
      } finally {
        setLoading(false);
      }
    };

    fetchWatchlist();
  }, [user]);

  // 退出登录
  const handleLogout = () => {
    localStorage.removeItem('token');
    localStorage.removeItem('user');
    window.location.href = '/login';
  };

  if (loading) {
    return (
      <div className={styles.container}>
        <PillNav items={navItems} activeHref="/profile" />
        <div style={{ position: 'relative', zIndex: 1, paddingTop: '100px', textAlign: 'center' }}>
          <p className={styles.loading}>加载中...</p>
        </div>
      </div>
    );
  }

  return (
    <div className={styles.container}>
      {/* 导航栏 */}
      <PillNav
        items={navItems}
        activeHref="/profile"
        baseColor="#000000"
        pillColor="#ffffff"
        hoveredPillTextColor="#ffffff"
        pillTextColor="#000000"
      />

      {/* 内容层 */}
      <div style={{ position: 'relative', zIndex: 1, paddingTop: '100px', maxWidth: '1200px', margin: '0 auto', padding: '0 20px' }}>
        <h1 className={styles.title}>账户中心</h1>
        
        {error && (
          <div className={styles.message} style={{ marginBottom: '2rem' }}>
            {error}
          </div>
        )}

        {/* 用户信息卡片 */}
        <section className={styles.section} style={{ marginBottom: '3rem' }}>
          <h2 className={styles.sectionTitle}>个人信息</h2>
          <div className={styles.fundsGrid} style={{ gridTemplateColumns: '1fr' }}>
            <div className={styles.fundCard}>
              <h3 className={styles.fundName}>账户信息</h3>
              <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
                <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                  <span className={styles.profitLabel}>邮箱:</span>
                  <span style={{ color: '#ffffff' }}>{user?.email || 'N/A'}</span>
                </div>
                <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                  <span className={styles.profitLabel}>注册时间:</span>
                  <span style={{ color: '#ffffff' }}>{new Date().toLocaleDateString()}</span>
                </div>
                <button 
                  onClick={handleLogout}
                  style={{
                    marginTop: '1rem',
                    padding: '10px 20px',
                    backgroundColor: '#ffffff',
                    color: '#000000',
                    border: 'none',
                    borderRadius: '8px',
                    cursor: 'pointer',
                    fontWeight: 'bold',
                  }}
                >
                  退出登录
                </button>
              </div>
            </div>
          </div>
        </section>

        {/* 关注的基金 */}
        <section className={styles.section}>
          <h2 className={styles.sectionTitle}>关注的基金</h2>
          {watchlist.length > 0 ? (
            <div className={styles.fundsGrid}>
              {watchlist.map((item, index) => (
                <div key={item.fundCode} className={styles.fundCard}>
                  <h3 className={styles.fundName}>{item.fundName}</h3>
                  <p className={styles.fundCode}>{item.fundCode}</p>
                  <div className={styles.fundProfit}>
                    <span className={styles.profitLabel}>提醒阈值:</span>
                    <span className={styles.profitValue}>{item.alertThreshold}%</span>
                  </div>
                  <div className={styles.fundProfit}>
                    <span className={styles.profitLabel}>添加时间:</span>
                    <span style={{ color: '#ffffff' }}>{new Date(item.createdAt).toLocaleDateString()}</span>
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <p style={{ color: 'rgba(255, 255, 255, 0.7)', textAlign: 'center' }}>
              您还没有关注任何基金
            </p>
          )}
        </section>
      </div>
    </div>
  );
}
