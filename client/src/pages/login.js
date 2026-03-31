import { useState } from 'react';
import styles from '../../styles/Login.module.css';
import PillNav from '../components/PillNav';

export default function Login() {
  const [isRegister, setIsRegister] = useState(false);
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [verificationCode, setVerificationCode] = useState('');
  const [message, setMessage] = useState('');
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e) => {
    e.preventDefault();
    setLoading(true);
    setMessage('');

    try {
      const endpoint = isRegister ? '/api/auth/register' : '/api/auth/login';
      const data = isRegister ? { email, password } : { email, password };

      const response = await fetch(`https://fund-tracking-production.up.railway.app${endpoint}`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(data),
      });

      const result = await response.json();

      if (response.ok) {
        if (isRegister) {
          if (result.emailSent) {
            setMessage('验证码已发送！请检查邮箱获取验证码');
          } else if (result.verificationCode) {
            setMessage(`验证码已生成！邮件发送失败，但验证码是：${result.verificationCode}`);
          } else {
            setMessage('注册成功！请检查邮箱获取验证码');
          }
        } else {
          // 登录成功，保存token和用户信息
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

  const handleVerify = async (e) => {
    e.preventDefault();
    setLoading(true);
    setMessage('');

    try {
      const response = await fetch('http://localhost:3001/api/auth/verify', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ email, code: verificationCode, password }),
      });

      const result = await response.json();

      if (response.ok) {
        setMessage('验证成功！请登录');
        setIsRegister(false);
      } else {
        setMessage(result.error || '验证失败');
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
          <>
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

            <form onSubmit={handleVerify} className={styles.form}>
              <div className={styles.inputGroup}>
                <label>验证码</label>
                <input
                  type="text"
                  value={verificationCode}
                  onChange={(e) => setVerificationCode(e.target.value)}
                  required
                />
              </div>
              <button type="submit" className={styles.button} disabled={loading}>
                {loading ? '验证中...' : '验证'}
              </button>
            </form>
          </>
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
