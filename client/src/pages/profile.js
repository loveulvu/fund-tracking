import { useState, useEffect } from 'react';
import PillNav from '../components/PillNav';
import styles from '../../styles/Home.module.css';

export default function Profile() {
  const [user, setUser] = useState(null);
  const [watchlist, setWatchlist] = useState([]);
  const [fundData, setFundData] = useState({});
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [editingThreshold, setEditingThreshold] = useState(null);
  const [newThreshold, setNewThreshold] = useState('');

  // 导航项 - 移除Account选项
  const navItems = [
    { label: 'Home', href: '/' },
    { label: 'Funds', href: '/about' },
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

  // 暂时禁用关注功能，后端 API 未实现
  useEffect(() => {
    // 后端 API 未实现，暂时返回空数组
    setWatchlist([]);
    setLoading(false);
  }, [user]);

  // 暂时禁用获取基金数据功能
  const fetchFundData = async (fundCode) => {
    // 后端 API 未实现
  };

  // 暂时禁用取消关注功能，后端 API 未实现
  const handleUnwatch = async (fundCode) => {
    alert('取消关注功能暂未实现');
  };

  // 暂时禁用更新阈值功能，后端 API 未实现
  const handleUpdateThreshold = async (fundCode) => {
    if (!newThreshold || isNaN(newThreshold)) {
      alert('请输入有效的阈值');
      return;
    }
    alert('更新阈值功能暂未实现');
  };

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
        activeHref="/"
        baseColor="#000000"
        pillColor="#ffffff"
        hoveredPillTextColor="#ffffff"
        pillTextColor="#000000"
      />

      {/* 内容层 */}
      <div style={{ position: 'relative', zIndex: 1, paddingTop: '100px', maxWidth: '1200px', margin: '0 auto', padding: '100px 20px 0' }}>
        <h1 className={styles.title}>账户中心</h1>
        
        {error && (
          <div className={styles.message} style={{ marginBottom: '2rem', color: '#ff4444' }}>
            {error}
          </div>
        )}

        {/* 用户信息卡片 */}
        <section style={{ marginBottom: '3rem' }}>
          <h2 style={{ marginBottom: '1rem', color: '#ffffff' }}>个人信息</h2>
          <div style={{
            backgroundColor: 'rgba(255, 255, 255, 0.1)',
            padding: '20px',
            borderRadius: '8px'
          }}>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
              <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                <span style={{ opacity: 0.8 }}>邮箱:</span>
                <span style={{ color: '#ffffff' }}>{user?.email || 'N/A'}</span>
              </div>
              <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                <span style={{ opacity: 0.8 }}>注册时间:</span>
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
        </section>

        {/* 关注的基金 */}
        <section>
          <h2 style={{ marginBottom: '1rem', color: '#ffffff' }}>关注的基金 ({watchlist.length})</h2>
          {watchlist.length > 0 ? (
            <div style={{
              display: 'grid',
              gridTemplateColumns: 'repeat(auto-fill, minmax(300px, 1fr))',
              gap: '20px'
            }}>
              {watchlist.map((item) => {
                const fund = fundData[item.fundCode];
                return (
                  <div key={item.fundCode} style={{
                    backgroundColor: 'rgba(255, 255, 255, 0.1)',
                    padding: '20px',
                    borderRadius: '8px'
                  }}>
                    <h3 style={{ marginBottom: '10px', textAlign: 'center' }}>{item.fundName}</h3>
                    <p style={{ margin: '5px 0', fontSize: '0.9rem', opacity: 0.8, textAlign: 'center' }}>
                      基金代码: {item.fundCode}
                    </p>
                    
                    {/* 实时收益数据 */}
                    {fund && (
                      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '10px', marginTop: '15px' }}>
                        <div style={{ padding: '10px', borderBottom: '1px solid rgba(255, 255, 255, 0.2)' }}>
                          <span style={{ display: 'block', fontSize: '0.8rem', opacity: 0.8 }}>近1月收益</span>
                          <span style={{ fontSize: '1.1rem', fontWeight: '300' }}>{fund.one_month_profit}</span>
                        </div>
                        <div style={{ padding: '10px', borderBottom: '1px solid rgba(255, 255, 255, 0.2)' }}>
                          <span style={{ display: 'block', fontSize: '0.8rem', opacity: 0.8 }}>近3月收益</span>
                          <span style={{ fontSize: '1.1rem', fontWeight: '300' }}>{fund.three_month_profit}</span>
                        </div>
                      </div>
                    )}

                    {/* 提醒阈值设置 */}
                    <div style={{ marginTop: '15px', padding: '10px', backgroundColor: 'rgba(255, 255, 255, 0.05)', borderRadius: '4px' }}>
                      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '10px' }}>
                        <span style={{ fontSize: '0.9rem', opacity: 0.8 }}>提醒阈值:</span>
                        {editingThreshold === item.fundCode ? (
                          <div style={{ display: 'flex', gap: '5px' }}>
                            <input
                              type="number"
                              value={newThreshold}
                              onChange={(e) => setNewThreshold(e.target.value)}
                              placeholder="输入阈值"
                              style={{
                                width: '80px',
                                padding: '5px',
                                borderRadius: '4px',
                                border: '1px solid #ddd'
                              }}
                            />
                            <button
                              onClick={() => handleUpdateThreshold(item.fundCode)}
                              style={{
                                padding: '5px 10px',
                                borderRadius: '4px',
                                border: 'none',
                                backgroundColor: '#4CAF50',
                                color: 'white',
                                cursor: 'pointer'
                              }}
                            >
                              保存
                            </button>
                          </div>
                        ) : (
                          <span style={{ color: '#4CAF50', fontWeight: 'bold' }}>{item.alertThreshold}%</span>
                        )}
                      </div>
                      {editingThreshold !== item.fundCode && (
                        <button
                          onClick={() => {
                            setEditingThreshold(item.fundCode);
                            setNewThreshold(item.alertThreshold.toString());
                          }}
                          style={{
                            width: '100%',
                            padding: '5px',
                            borderRadius: '4px',
                            border: 'none',
                            backgroundColor: 'rgba(255, 255, 255, 0.1)',
                            color: 'white',
                            cursor: 'pointer',
                            fontSize: '0.85rem'
                          }}
                        >
                          修改阈值
                        </button>
                      )}
                    </div>

                    {/* 添加时间 */}
                    <div style={{ marginTop: '10px', fontSize: '0.8rem', opacity: 0.6, textAlign: 'center' }}>
                      添加时间: {new Date(item.addedAt).toLocaleDateString()}
                    </div>

                    {/* 取消关注按钮 */}
                    <button
                      onClick={() => handleUnwatch(item.fundCode)}
                      style={{
                        width: '100%',
                        marginTop: '15px',
                        padding: '8px',
                        borderRadius: '4px',
                        border: 'none',
                        backgroundColor: '#ff4444',
                        color: 'white',
                        cursor: 'pointer'
                      }}
                    >
                      取消关注
                    </button>
                  </div>
                );
              })}
            </div>
          ) : (
            <p style={{ color: 'rgba(255, 255, 255, 0.7)', textAlign: 'center', padding: '40px' }}>
              您还没有关注任何基金，<a href="/about" style={{ color: '#4CAF50' }}>去关注</a>
            </p>
          )}
        </section>
      </div>
    </div>
  );
}
