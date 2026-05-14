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
        setMessage(result.error || 'Request failed');
      }
    } catch (error) {
      setMessage('Network error. Please try again later.');
      console.error('Error:', error);
    } finally {
      setLoading(false);
    }
  };

  return (
    <DashboardShell
      activeHref="/login"
      noteTitle="Account access"
      noteText="Sign in to manage your watchlist and alert thresholds."
    >
      <header className={styles.pageHeader}>
        <div>
          <p className={styles.eyebrow}>Account</p>
          <h1>{isRegister ? 'Create Account' : 'Login'}</h1>
          <p>
            {isRegister
              ? 'Create an account, then log in directly with the same credentials.'
              : 'Sign in to view your watchlist and manage alert thresholds.'}
          </p>
        </div>
      </header>

      <section className={styles.authLayout}>
        <article className={styles.formCard}>
          <div className={styles.panelHeader}>
            <div>
              <h2>{isRegister ? 'Register' : 'Welcome back'}</h2>
              <p>{isRegister ? 'Email and password are required.' : 'Use your account email and password.'}</p>
            </div>
          </div>

          <form className={styles.formStack} onSubmit={handleSubmit}>
            {message && <div className={styles.messageBox}>{message}</div>}

            <label className={styles.field}>
              <span>Email</span>
              <input
                type="email"
                value={email}
                onChange={(event) => setEmail(event.target.value)}
                required
              />
            </label>

            <label className={styles.field}>
              <span>Password</span>
              <input
                type="password"
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                required
              />
            </label>

            <button className={styles.primaryButton} type="submit" disabled={loading}>
              {loading ? 'Submitting...' : isRegister ? 'Create account' : 'Login'}
            </button>
          </form>

          <div className={styles.authSwitch}>
            {isRegister ? (
              <span>
                Already have an account?{' '}
                <Link href="/login" onClick={() => setMode('login')}>
                  Login
                </Link>
              </span>
            ) : (
              <span>
                No account yet?{' '}
                <Link href="/register" onClick={() => setMode('register')}>
                  Register
                </Link>
              </span>
            )}
          </div>
        </article>

        <aside className={styles.authAside}>
          <span>Protected features</span>
          <strong>Watchlist uses JWT authentication.</strong>
          <p>
            Login is required before adding funds, removing funds, or updating alert thresholds.
          </p>
        </aside>
      </section>
    </DashboardShell>
  );
}

export default function Login() {
  return <AuthPage initialMode="login" />;
}
