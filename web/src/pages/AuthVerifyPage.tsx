import { useEffect, useState } from 'react';
import { Link, useNavigate, useSearchParams } from 'react-router-dom';
import { verifyMagicLink } from '../lib/auth-service';
import { PageMeta } from '../components/PageMeta';

export function AuthVerifyPage() {
  const [params] = useSearchParams();
  const navigate = useNavigate();
  const token = params.get('token');
  const [error, setError] = useState(!token);
  useEffect(() => {
    if (!token) return;
    void verifyMagicLink(token).then((result) => {
      navigate(result.user.onboarding_complete ? (result.return_path || '/me') : '/onboarding', { replace: true });
    }).catch(() => setError(true));
  }, [navigate, token]);
  return <main className="signal-grid grid min-h-[70vh] place-items-center p-6"><PageMeta title="Verify sign-in link"/>
    <section className="panel max-w-lg p-8 text-center" aria-live="polite">
      <p className="eyebrow">One-time sign in</p>
      <h1 className="font-display mt-4 text-4xl uppercase">{error ? 'This link no longer works' : 'Signing you in…'}</h1>
      <p className="mt-4 text-foreground-secondary">{error ? 'It may be invalid, expired, or already used.' : 'Keep this window open while we verify your link.'}</p>
      {error && <Link className="button-primary mt-7" to="/login">Request another link</Link>}
    </section>
  </main>;
}
