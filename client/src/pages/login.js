import { useState } from 'react';
import styles from '../../styles/Login.module.css';
import PillNav from '../components/PillNav';
import api from '../lib/api';

export default function Login() {
  const [isRegister, setIsRegister] = useState(false);
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [message, setMessage] = useState('');
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e) => {
    e.preventDefault();
    setLoading(true);
    setMessage('');

    try {
      const response = isRegister 
        ? await api.register(email, password)
        : await api.login(email, password);

      const result = await response.json();

      if (response.ok) {
        if (isRegister) {
          setMessage('Registration successful. Please log in.');
          setIsRegister(false);
          setPassword('');
        } else {
          localStorage.setItem('token', result.token);
          localStorage.setItem('user', JSON.stringify({ email: result.email }));
          window.location.href = '/profile';
        }
      } else {
        setMessage(result.error || '操作失败');
      }
    } catch (error) {
      setMessage('网络错误，请稍后重试');
      console.error('Error:', error);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className={styles.container}>
      <PillNav 
        items={[
          { label: 'Home', href: '/' },
          { label: 'Funds', href: '/about' }
        ]} 
        activeHref="/"
        baseColor="#000000"
        pillColor="#ffffff"
        hoveredPillTextColor="#ffffff"
        pillTextColor="#000000"
        theme="light"
      />
      <div className={styles.formContainer}>
        <h1 className={styles.title}>{isRegister ? '注册' : '登录'}</h1>
        
        {message && (
          <div className={styles.message}>{message}</div>
        )}

        {isRegister ? (
          <form onSubmit={handleSubmit} className={styles.form}>
            <div className={styles.inputGroup}>
              <label>邮箱</label>
              <input
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
              />
            </div>
            <div className={styles.inputGroup}>
              <label>密码</label>
              <input
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
              />
            </div>
            <button type="submit" className={styles.button} disabled={loading}>
              {loading ? '注册中...' : '注册'}
            </button>
          </form>
        ) : (
          <form onSubmit={handleSubmit} className={styles.form}>
            <div className={styles.inputGroup}>
              <label>邮箱</label>
              <input
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
              />
            </div>
            <div className={styles.inputGroup}>
              <label>密码</label>
              <input
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
              />
            </div>
            <button type="submit" className={styles.button} disabled={loading}>
              {loading ? '登录中...' : '登录'}
            </button>
          </form>
        )}

        <div className={styles.switch}>
          {isRegister ? (
            <>
              已有账号？{' '}
              <button
                type="button"
                onClick={() => setIsRegister(false)}
                className={styles.switchButton}
              >
                去登录
              </button>
            </>
          ) : (
            <>
              没有账号？{' '}
              <button
                type="button"
                onClick={() => setIsRegister(true)}
                className={styles.switchButton}
              >
                去注册
              </button>
            </>
          )}
        </div>
      </div>
    </div>
  );
}
