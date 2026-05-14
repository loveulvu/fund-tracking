import { useState } from 'react';
import Link from 'next/link';
import DashboardShell from '../components/DashboardShell';
import api from '../lib/api';
import styles from '../../styles/Dashboard.module.css';

export function AuthPage({ initialMode = 'login' }) {
  const [mode, setMode] = useState(initialMode);
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [message, setMessage] = useState('');
  const [loading, setLoading] = useState(false);

  const isRegister = mode === 'register';

  const handleSubmit = async (event) => {
    event.preventDefault();
    setLoading(true);
    setMessage('');

    try {
      const response = isRegister
        ? await api.register(email, password)
        : await api.login(email, password);

      const result = await response.json();

      if (response.ok) {
        if (isRegister) {
          setMessage('Registration successful. You can log in now.');
          setMode('login');
          setPassword('');
        } else {
          localStorage.setItem('token', result.token);
          localStorage.setItem('user', JSON.stringify({ email: result.email }));
          window.location.href = '/profile';
        }
      } else {
        setMessage(result.error || '请求失败');
      }
    } catch (error) {
      setMessage('网络错误，请稍后重试。');
      console.error('Error:', error);
    } finally {
      setLoading(false);
    }
  };

  return (
    <DashboardShell
      activeHref="/login"
      noteTitle="账户入口"
      noteText="登录后可以管理关注基金和提醒阈值。"
    >
      <header className={styles.pageHeader}>
        <div>
          <p className={styles.eyebrow}>账户</p>
          <h1>{isRegister ? '注册账户' : '登录'}</h1>
          <p>
            {isRegister
              ? '创建账户后，可以直接使用同一邮箱和密码登录。'
              : '登录后查看关注列表，并管理基金提醒阈值。'}
          </p>
        </div>
      </header>

      <section className={styles.authCentered}>
        <article className={styles.formCard}>
          <div className={styles.panelHeader}>
            <div>
              <h2>{isRegister ? '注册' : '欢迎回来'}</h2>
              <p>{isRegister ? '请输入邮箱和密码完成注册。' : '使用邮箱和密码登录账户。'}</p>
            </div>
          </div>

          <form className={styles.formStack} onSubmit={handleSubmit}>
            {message && <div className={styles.messageBox}>{message}</div>}

            <label className={styles.field}>
              <span>邮箱</span>
              <input
                type="email"
                value={email}
                onChange={(event) => setEmail(event.target.value)}
                required
              />
            </label>

            <label className={styles.field}>
              <span>密码</span>
              <input
                type="password"
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                required
              />
            </label>

            <button className={styles.primaryButton} type="submit" disabled={loading}>
              {loading ? '提交中...' : isRegister ? '注册' : '登录'}
            </button>
          </form>

          <div className={styles.authSwitch}>
            {isRegister ? (
              <span>
                已有账户？{' '}
                <Link href="/login" onClick={() => setMode('login')}>
                  去登录
                </Link>
              </span>
            ) : (
              <span>
                还没有账户？{' '}
                <Link href="/register" onClick={() => setMode('register')}>
                  去注册
                </Link>
              </span>
            )}
          </div>
        </article>

        <p className={styles.authHint}>关注、取消关注和阈值修改需要登录，接口使用 JWT 认证。</p>
      </section>
    </DashboardShell>
  );
}

export default function Login() {
  return <AuthPage initialMode="login" />;
}
