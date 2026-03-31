import Particles from '../components/Particles';
import Link from 'next/link';
import PillNav from '../components/PillNav';
import { useState, useEffect } from 'react';
import styles from '../../styles/Home.module.css';

export default function About() {
  const [fundsData, setFundsData] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [searchTerm, setSearchTerm] = useState('');
  const [filteredFunds, setFilteredFunds] = useState([]);
  const [user, setUser] = useState(null);
  const [watchlist, setWatchlist] = useState([]);
  const [watchlistLoading, setWatchlistLoading] = useState({});

  // 导航项 - 只显示Home和Account
  const navItems = [
    { label: 'Home', href: '/' },
    { label: user ? 'Account' : 'Login', href: user ? '/profile' : '/login' },
  ];

  // 检查用户登录状态
  useEffect(() => {
    const savedUser = localStorage.getItem('user');
    if (savedUser) {
      setUser(JSON.parse(savedUser));
    }
  }, []);

  // 获取关注列表
  const fetchWatchlist = async () => {
    if (!user) return;
    
    try {
      const token = localStorage.getItem('token');
      const response = await fetch('https://fund-tracking-production.up.railway.app/api/watchlist', {
        headers: {
          'Authorization': `Bearer ${token}`
        }
      });
      
      if (response.ok) {
        const data = await response.json();
        setWatchlist(data);
      }
    } catch (err) {
      console.error('Error fetching watchlist:', err);
    }
  };

  // 用户登录后获取关注列表
  useEffect(() => {
    if (user) {
      fetchWatchlist();
    }
  }, [user]);

  // 检查基金是否已关注
  const isWatched = (fundCode) => {
    return watchlist.some(item => item.fundCode === fundCode);
  };

  // 关注基金
  const handleWatch = async (fund) => {
    if (!user) {
      alert('请先登录');
      return;
    }

    const fundCode = fund.fund_code;
    setWatchlistLoading(prev => ({ ...prev, [fundCode]: true }));

    try {
      const token = localStorage.getItem('token');
      const response = await fetch('https://fund-tracking-production.up.railway.app/api/watchlist', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`
        },
        body: JSON.stringify({
          fundCode: fundCode,
          fundName: fund.fund_name,
          alertThreshold: 5
        })
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
      setWatchlistLoading(prev => ({ ...prev, [fundCode]: false }));
    }
  };

  // 取消关注
  const handleUnwatch = async (fundCode) => {
    if (!user) return;

    setWatchlistLoading(prev => ({ ...prev, [fundCode]: true }));

    try {
      const token = localStorage.getItem('token');
      const response = await fetch(`https://fund-tracking-production.up.railway.app/api/watchlist/${fundCode}`, {
        method: 'DELETE',
        headers: {
          'Authorization': `Bearer ${token}`
        }
      });

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
      setWatchlistLoading(prev => ({ ...prev, [fundCode]: false }));
    }
  };

  // 获取所有基金数据
  useEffect(() => {
    const fetchFundsData = async () => {
      try {
        setLoading(true);
        const response = await fetch('https://fund-tracking-production.up.railway.app/api/funds');
        if (!response.ok) {
          throw new Error(`Failed to fetch funds data: ${response.status}`);
        }
        const data = await response.json();

        // 确保数据是数组
        if (Array.isArray(data)) {
          setFundsData(data);
          setFilteredFunds(data);
        } else {
          console.error('Invalid data format:', data);
          setError('Invalid data format received from server');
        }
        setError(null);
      } catch (err) {
        setError('Error fetching funds data: ' + err.message);
        console.error(err);
      } finally {
        setLoading(false);
      }
    };

    fetchFundsData();
  }, []);

  // 搜索基金
  const handleSearch = async (e) => {
    e.preventDefault();
    if (searchTerm.trim() === '') {
      setFilteredFunds(fundsData);
      return;
    }

    // 检查是否是基金代码（6位数字）
    const isFundCode = /^\d{6}$/.test(searchTerm);

    if (isFundCode) {
      // 如果是基金代码，尝试直接获取该基金数据
      try {
        setLoading(true);
        const response = await fetch(`https://fund-tracking-production.up.railway.app/api/fund/${searchTerm}`);
        if (!response.ok) {
          throw new Error('Failed to fetch fund data');
        }
        const data = await response.json();

        // 检查返回的数据是否有效
        if (!data || !data.fund_code) {
          throw new Error('Invalid fund data received');
        }

        // 只显示搜索到的基金
        setFilteredFunds([data]);
        setError(null);
      } catch (err) {
        setError('Error fetching fund data: ' + err.message);
        console.error(err);
      } finally {
        setLoading(false);
      }
    } else {
      // 如果不是基金代码，尝试从服务器搜索
      try {
        setLoading(true);
        const response = await fetch(`https://fund-tracking-production.up.railway.app/api/funds/search?query=${encodeURIComponent(searchTerm)}`);
        if (response.ok) {
          const data = await response.json();
          if (Array.isArray(data) && data.length > 0) {
            setFilteredFunds(data);
          } else {
            // 如果服务器返回空数组，在本地数据中搜索
            const term = searchTerm.toLowerCase();
            const filtered = fundsData.filter(fund => {
              const fundName = fund.fund_name || '';
              const fundCode = fund.fund_code || '';
              return fundName.toLowerCase().includes(term) ||
                     fundCode.includes(term);
            });
            setFilteredFunds(filtered);
          }
        } else {
          // 如果服务器搜索失败，在本地数据中搜索
          const term = searchTerm.toLowerCase();
          const filtered = fundsData.filter(fund => {
            const fundName = fund.fund_name || '';
            const fundCode = fund.fund_code || '';
            return fundName.toLowerCase().includes(term) ||
                   fundCode.includes(term);
          });
          setFilteredFunds(filtered);
        }
        setError(null);
      } catch (err) {
        // 搜索失败，在本地数据中搜索
        const term = searchTerm.toLowerCase();
        const filtered = fundsData.filter(fund => {
          const fundName = fund.fund_name || '';
          const fundCode = fund.fund_code || '';
          return fundName.toLowerCase().includes(term) ||
                 fundCode.includes(term);
        });
        setFilteredFunds(filtered);
        setError('搜索失败，显示本地数据');
        console.error(err);
      } finally {
        setLoading(false);
      }
    }
  };

  // 清空搜索
  const handleClearSearch = () => {
    setSearchTerm('');
    setFilteredFunds(fundsData);
  };

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
          alphaParticles={false}
          disableRotation={false}
          pixelRatio={1}
        />
      </div>

      {/* 导航栏 */}
      <PillNav
        items={navItems}
        activeHref="/"
        baseColor="#000000"
        pillColor="#ffffff"
        hoveredPillTextColor="#ffffff"
        pillTextColor="#000000"
      />

      {/* 内容层，必须在粒子层上面 */}
      <div style={{ position: 'relative', zIndex: 1, textAlign: 'center', padding: '0 20px', paddingTop: '100px' }}>
        <h1 className={styles.title}>Fund Data</h1>
        <p>
          <Link href="/" className={styles.link}>
            Back to Home
          </Link>
        </p>

        {/* 搜索表单 */}
        <form onSubmit={handleSearch} style={{
          margin: '40px 0',
          display: 'flex',
          gap: '1rem',
          justifyContent: 'center',
          flexWrap: 'wrap'
        }}>
          <input
            type="text"
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            placeholder="输入基金名称或代码"
            style={{
              padding: '0.75rem',
              borderRadius: '4px',
              border: '1px solid #ddd',
              width: '300px',
              backgroundColor: 'rgba(255, 255, 255, 0.9)'
            }}
          />
          <button
            type="submit"
            style={{
              padding: '0.75rem 1.5rem',
              borderRadius: '4px',
              border: 'none',
              backgroundColor: '#0070f3',
              color: 'white',
              cursor: 'pointer'
            }}
          >
            搜索
          </button>
          <button
            type="button"
            onClick={handleClearSearch}
            style={{
              padding: '0.75rem 1.5rem',
              borderRadius: '4px',
              border: 'none',
              backgroundColor: '#666',
              color: 'white',
              cursor: 'pointer'
            }}
          >
            清空
          </button>
        </form>

        {/* 基金数据显示区域 */}
        <div style={{ marginTop: '20px', color: '#ffffff', maxWidth: '1200px', margin: '20px auto 0' }}>
          {loading && <p>Loading fund data...</p>}
          {error && <p>{error}</p>}
          {!loading && !error && (
            <div>
              <h2 style={{ marginBottom: '20px' }}>基金列表 ({filteredFunds.length} 个结果)</h2>
              <div style={{
                display: 'grid',
                gridTemplateColumns: 'repeat(auto-fill, minmax(300px, 1fr))',
                gap: '20px',
                justifyContent: 'center'
              }}>
                {filteredFunds.map((fund) => (
                  <div key={fund.fund_code} style={{
                    textAlign: 'left',
                    backgroundColor: 'rgba(255, 255, 255, 0.1)',
                    padding: '20px',
                    borderRadius: '8px'
                  }}>
                    <h3 style={{ marginBottom: '15px', textAlign: 'center' }}>{fund.fund_name}</h3>
                    <p style={{ margin: '5px 0', fontSize: '0.9rem', opacity: 0.8 }}>基金代码: {fund.fund_code}</p>
                    {fund.net_value && (
                      <p style={{ margin: '5px 0', fontSize: '0.9rem', opacity: 0.8 }}>
                        单位净值: {fund.net_value} {fund.net_value_date && `(${fund.net_value_date})`}
                      </p>
                    )}
                    {fund.day_growth !== undefined && (
                      <p style={{ margin: '5px 0', fontSize: '0.9rem', opacity: 0.8 }}>
                        日涨跌幅: <span style={{ color: fund.day_growth >= 0 ? '#ff4444' : '#00ff00' }}>
                          {fund.day_growth >= 0 ? '+' : ''}{fund.day_growth}%
                        </span>
                      </p>
                    )}
                    <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '10px', marginTop: '15px' }}>
                      <div style={{ padding: '10px', borderBottom: '1px solid rgba(255, 255, 255, 0.2)' }}>
                        <span style={{ display: 'block', fontSize: '0.8rem', opacity: 0.8 }}>近1周收益</span>
                        <span style={{ fontSize: '1.1rem', fontWeight: '300', color: fund.week_growth >= 0 ? '#ff4444' : '#00ff00' }}>
                          {fund.week_growth !== undefined ? `${fund.week_growth >= 0 ? '+' : ''}${fund.week_growth}%` : '-'}
                        </span>
                      </div>
                      <div style={{ padding: '10px', borderBottom: '1px solid rgba(255, 255, 255, 0.2)' }}>
                        <span style={{ display: 'block', fontSize: '0.8rem', opacity: 0.8 }}>近1月收益</span>
                        <span style={{ fontSize: '1.1rem', fontWeight: '300', color: fund.month_growth >= 0 ? '#ff4444' : '#00ff00' }}>
                          {fund.month_growth !== undefined ? `${fund.month_growth >= 0 ? '+' : ''}${fund.month_growth}%` : '-'}
                        </span>
                      </div>
                      <div style={{ padding: '10px', borderBottom: '1px solid rgba(255, 255, 255, 0.2)' }}>
                        <span style={{ display: 'block', fontSize: '0.8rem', opacity: 0.8 }}>近3月收益</span>
                        <span style={{ fontSize: '1.1rem', fontWeight: '300', color: fund.three_month_growth >= 0 ? '#ff4444' : '#00ff00' }}>
                          {fund.three_month_growth !== undefined ? `${fund.three_month_growth >= 0 ? '+' : ''}${fund.three_month_growth}%` : '-'}
                        </span>
                      </div>
                      <div style={{ padding: '10px', borderBottom: '1px solid rgba(255, 255, 255, 0.2)' }}>
                        <span style={{ display: 'block', fontSize: '0.8rem', opacity: 0.8 }}>近6月收益</span>
                        <span style={{ fontSize: '1.1rem', fontWeight: '300', color: fund.six_month_growth >= 0 ? '#ff4444' : '#00ff00' }}>
                          {fund.six_month_growth !== undefined ? `${fund.six_month_growth >= 0 ? '+' : ''}${fund.six_month_growth}%` : '-'}
                        </span>
                      </div>
                      <div style={{ padding: '10px', borderBottom: '1px solid rgba(255, 255, 255, 0.2)' }}>
                        <span style={{ display: 'block', fontSize: '0.8rem', opacity: 0.8 }}>近1年收益</span>
                        <span style={{ fontSize: '1.1rem', fontWeight: '300', color: fund.year_growth >= 0 ? '#ff4444' : '#00ff00' }}>
                          {fund.year_growth !== undefined ? `${fund.year_growth >= 0 ? '+' : ''}${fund.year_growth}%` : '-'}
                        </span>
                      </div>
                      <div style={{ padding: '10px', borderBottom: '1px solid rgba(255, 255, 255, 0.2)' }}>
                        <span style={{ display: 'block', fontSize: '0.8rem', opacity: 0.8 }}>近3年收益</span>
                        <span style={{ fontSize: '1.1rem', fontWeight: '300', color: fund.three_year_growth >= 0 ? '#ff4444' : '#00ff00' }}>
                          {fund.three_year_growth !== undefined ? `${fund.three_year_growth >= 0 ? '+' : ''}${fund.three_year_growth}%` : '-'}
                        </span>
                      </div>
                    </div>
                    {/* 关注按钮 */}
                    <div style={{ marginTop: '15px', textAlign: 'center' }}>
                      {user ? (
                        isWatched(fund.fund_code) ? (
                          <button
                            onClick={() => handleUnwatch(fund.fund_code)}
                            disabled={watchlistLoading[fund.fund_code]}
                            style={{
                              padding: '8px 20px',
                              borderRadius: '4px',
                              border: 'none',
                              backgroundColor: '#ff4444',
                              color: 'white',
                              cursor: watchlistLoading[fund.fund_code] ? 'not-allowed' : 'pointer',
                              opacity: watchlistLoading[fund.fund_code] ? 0.6 : 1
                            }}
                          >
                            {watchlistLoading[fund.fund_code] ? '处理中...' : '取消关注'}
                          </button>
                        ) : (
                          <button
                            onClick={() => handleWatch(fund)}
                            disabled={watchlistLoading[fund.fund_code]}
                            style={{
                              padding: '8px 20px',
                              borderRadius: '4px',
                              border: 'none',
                              backgroundColor: '#4CAF50',
                              color: 'white',
                              cursor: watchlistLoading[fund.fund_code] ? 'not-allowed' : 'pointer',
                              opacity: watchlistLoading[fund.fund_code] ? 0.6 : 1
                            }}
                          >
                            {watchlistLoading[fund.fund_code] ? '处理中...' : '关注'}
                          </button>
                        )
                      ) : (
                        <button
                          onClick={() => alert('请先登录')}
                          style={{
                            padding: '8px 20px',
                            borderRadius: '4px',
                            border: 'none',
                            backgroundColor: '#666',
                            color: 'white',
                            cursor: 'pointer'
                          }}
                        >
                          登录后关注
                        </button>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
