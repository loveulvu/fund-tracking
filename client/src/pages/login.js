import { useEffect, useState } from 'react';
import Link from 'next/link';
import DashboardShell from '../components/DashboardShell';
import api from '../lib/api';
import styles from '../../styles/Dashboard.module.css';

export function AuthPage({ initialMode = 'login' }) {
  const [mode, setMode] = useState(initialMode);
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [verificationCode, setVerificationCode] = useState('');
  const [pendingEmail, setPendingEmail] = useState('');
  const [message, setMessage] = useState('');
  const [loading, setLoading] = useState(false);
  const [resendCooldown, setResendCooldown] = useState(0);

  const isRegister = mode === 'register';
  const isVerification = mode === 'verify';

  useEffect(() => {
    if (resendCooldown <= 0) return undefined;
    const timer = window.setInterval(() => {
      setResendCooldown((current) => Math.max(0, current - 1));
    }, 1000);
    return () => window.clearInterval(timer);
  }, [resendCooldown]);

  const switchMode = (nextMode) => {
    setMode(nextMode);
    setMessage('');
    setLoading(false);
    if (nextMode !== 'verify') {
      setVerificationCode('');
    }
  };

  const readErrorMessage = async (response, fallback) => {
    try {
      const result = await response.json();
      return { result, message: result.error || fallback };
    } catch {
      return { result: {}, message: fallback };
    }
  };

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
          const normalizedEmail = result.email || email;
          setPendingEmail(normalizedEmail);
          setEmail(normalizedEmail);
          setPassword('');
          setVerificationCode('');
          setResendCooldown(60);
          setMessage('验证码已发送到邮箱，请在 10 分钟内完成验证。');
          setMode('verify');
        } else {
          localStorage.setItem('token', result.token);
          localStorage.setItem('user', JSON.stringify({ email: result.email }));
          window.location.href = '/profile';
        }
      } else if (response.status === 403 && result.requiresVerification) {
        const normalizedEmail = result.email || email;
        setPendingEmail(normalizedEmail);
        setEmail(normalizedEmail);
        setPassword('');
        setVerificationCode('');
        setMessage('邮箱尚未验证，请输入验证码后再登录。');
        setMode('verify');
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

  const handleVerifyCode = async (event) => {
    event.preventDefault();
    setLoading(true);
    setMessage('');

    try {
      const response = await api.verifyEmailCode(pendingEmail || email, verificationCode);
      const { message: errorMessage } = await readErrorMessage(response, '验证码无效或已过期');

      if (response.ok) {
        setMessage('邮箱验证成功，请登录。');
        setMode('login');
        setPassword('');
        setVerificationCode('');
        setResendCooldown(0);
      } else {
        setMessage(errorMessage);
      }
    } catch (error) {
      setMessage('网络错误，请稍后重试。');
      console.error('Error:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleResendCode = async () => {
    if (resendCooldown > 0 || loading) return;
    setLoading(true);
    setMessage('');

    try {
      const response = await api.resendEmailCode(pendingEmail || email);
      const { result, message: errorMessage } = await readErrorMessage(response, '验证码发送失败');

      if (response.ok) {
        setResendCooldown(60);
        setMessage('新的验证码已发送，请查看邮箱。');
      } else if (response.status === 429) {
        const retryAfter = Number(result.retryAfterSeconds || 60);
        setResendCooldown(Number.isFinite(retryAfter) ? retryAfter : 60);
        setMessage(errorMessage);
      } else {
        setMessage(errorMessage);
      }
    } catch (error) {
      setMessage('网络错误，请稍后重试。');
      console.error('Error:', error);
    } finally {
      setLoading(false);
    }
  };

  const title = isVerification ? '邮箱验证' : isRegister ? '注册账户' : '登录';
  const subtitle = isVerification
    ? '输入邮箱收到的 6 位数字验证码。'
    : isRegister
      ? '创建账户后需要先完成邮箱验证。'
      : '登录后可管理关注基金和提醒阈值。';

  return (
    <DashboardShell
      activeHref="/login"
      noteTitle="账户入口"
      noteText="登录后可以管理关注基金和提醒阈值。"
    >
      <header className={[styles.pageHeader, styles.authPageHeader].join(' ')}>
        <div>
          <p className={styles.eyebrow}>账户</p>
          <h1>{title}</h1>
          <p>{subtitle}</p>
        </div>
      </header>

      <section className={styles.authCentered}>
        <article className={styles.formCard}>
          <div className={styles.panelHeader}>
            <div>
              <h2>{title}</h2>
              <p>{subtitle}</p>
            </div>
          </div>

          {isVerification ? (
            <form className={styles.formStack} onSubmit={handleVerifyCode}>
              {message && <div className={styles.messageBox}>{message}</div>}

              <label className={styles.field}>
                <span>邮箱</span>
                <input
                  type="email"
                  value={pendingEmail || email}
                  onChange={(event) => {
                    setPendingEmail(event.target.value);
                    setEmail(event.target.value);
                  }}
                  required
                />
              </label>

              <label className={styles.field}>
                <span>验证码</span>
                <input
                  type="text"
                  inputMode="numeric"
                  pattern="[0-9]{6}"
                  maxLength={6}
                  value={verificationCode}
                  onChange={(event) => setVerificationCode(event.target.value.replace(/\D/g, '').slice(0, 6))}
                  required
                />
              </label>

              <button className={styles.primaryButton} type="submit" disabled={loading}>
                {loading ? '提交中...' : '验证邮箱'}
              </button>

              <button
                className={styles.secondaryButton}
                type="button"
                onClick={handleResendCode}
                disabled={loading || resendCooldown > 0}
              >
                {resendCooldown > 0 ? `${resendCooldown} 秒后可重新发送` : '重新发送验证码'}
              </button>
            </form>
          ) : (
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
          )}

          <div className={styles.authSwitch}>
            {isVerification ? (
              <span>
                已完成验证？{' '}
                <Link href="/login" onClick={() => switchMode('login')}>
                  去登录
                </Link>
              </span>
            ) : isRegister ? (
              <span>
                已有账户？{' '}
                <Link href="/login" onClick={() => switchMode('login')}>
                  去登录
                </Link>
              </span>
            ) : (
              <span>
                还没有账户？{' '}
                <Link href="/register" onClick={() => switchMode('register')}>
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
