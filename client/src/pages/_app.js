import '../styles/globals.css';
import VersionBadge from '../components/VersionBadge';

export default function App({ Component, pageProps }) {
  return (
    <>
      <Component {...pageProps} />
      <VersionBadge />
    </>
  );
}
