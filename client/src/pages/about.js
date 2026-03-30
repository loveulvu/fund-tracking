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

  // 获取所有基金数据
  useEffect(() => {
    const fetchFundsData = async () => {
      try {
        setLoading(true);
        const response = await fetch('http://localhost:3001/api/funds/all');
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

  // 搜索功能
  useEffect(() => {
    if (searchTerm.trim() === '') {
      setFilteredFunds(fundsData);
    } else {
      const term = searchTerm.toLowerCase();
      const filtered = fundsData.filter(fund => {
        // 确保fund_name和fund_code存在
        const fundName = fund.fund_name || '';
        const fundCode = fund.fund_code || '';
        return fundName.toLowerCase().includes(term) || 
               fundCode.includes(term);
      });
      setFilteredFunds(filtered);
    }
  }, [searchTerm, fundsData]);

  // 搜索基金
  const handleSearch = async (e) => {
    e.preventDefault();
    if (searchTerm.trim() === '') return;
    
    // 检查是否是基金代码（6位数字）
    const isFundCode = /^\d{6}$/.test(searchTerm);
    
    if (isFundCode) {
      // 如果是基金代码，尝试直接获取该基金数据
      try {
        setLoading(true);
        const response = await fetch(`http://localhost:3001/api/fund/${searchTerm}`);
        if (!response.ok) {
          throw new Error('Failed to fetch fund data');
        }
        const data = await response.json();
        
        // 检查返回的数据是否有效
        if (!data || !data.fund_code) {
          throw new Error('Invalid fund data received');
        }
        
        // 检查该基金是否已在列表中
        const exists = fundsData.some(fund => fund.fund_code === data.fund_code);
        if (!exists) {
          // 如果不在列表中，添加到列表
          const updatedFunds = [...fundsData, data];
          setFundsData(updatedFunds);
          setFilteredFunds(updatedFunds);
        } else {
          // 如果在列表中，只显示该基金
          setFilteredFunds([data]);
        }
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
        const response = await fetch(`http://localhost:3001/api/funds/search?query=${encodeURIComponent(searchTerm)}`);
        if (response.ok) {
          const data = await response.json();
          if (Array.isArray(data)) {
            setFilteredFunds(data);
          } else {
            // 如果服务器返回的不是数组，在本地数据中搜索
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
                    <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '10px', marginTop: '15px' }}>
                      <div style={{ padding: '10px', borderBottom: '1px solid rgba(255, 255, 255, 0.2)' }}>
                        <span style={{ display: 'block', fontSize: '0.8rem', opacity: 0.8 }}>近1月收益</span>
                        <span style={{ fontSize: '1.1rem', fontWeight: '300' }}>{fund.one_month_profit}</span>
                      </div>
                      <div style={{ padding: '10px', borderBottom: '1px solid rgba(255, 255, 255, 0.2)' }}>
                        <span style={{ display: 'block', fontSize: '0.8rem', opacity: 0.8 }}>近3月收益</span>
                        <span style={{ fontSize: '1.1rem', fontWeight: '300' }}>{fund.three_month_profit}</span>
                      </div>
                      <div style={{ padding: '10px', borderBottom: '1px solid rgba(255, 255, 255, 0.2)' }}>
                        <span style={{ display: 'block', fontSize: '0.8rem', opacity: 0.8 }}>近6月收益</span>
                        <span style={{ fontSize: '1.1rem', fontWeight: '300' }}>{fund.six_month_profit}</span>
                      </div>
                      <div style={{ padding: '10px', borderBottom: '1px solid rgba(255, 255, 255, 0.2)' }}>
                        <span style={{ display: 'block', fontSize: '0.8rem', opacity: 0.8 }}>近1年收益</span>
                        <span style={{ fontSize: '1.1rem', fontWeight: '300' }}>{fund.one_year_profit}</span>
                      </div>
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