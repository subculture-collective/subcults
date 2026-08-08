import { useEffect, useState } from 'react';
import { Link, useNavigate, useSearchParams } from 'react-router-dom';
import { verifyMagicLink } from '../lib/auth-service';

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
  return <main className="signal-grid grid min-h-[70vh] place-items-center p-6">
    <section className="panel max-w-lg p-8 text-center" aria-live="polite">
      <p className="eyebrow">Identity handshake</p>
      <h1 className="font-display mt-4 text-4xl uppercase">{error ? 'Link dissolved' : 'Resolving access…'}</h1>
      <p className="mt-4 text-foreground-secondary">{error ? 'This link is invalid, expired, or already used.' : 'Keep this window open while the one-time signal is verified.'}</p>
      {error && <Link className="button-primary mt-7" to="/login">Request another link</Link>}
    </section>
  </main>;
}
