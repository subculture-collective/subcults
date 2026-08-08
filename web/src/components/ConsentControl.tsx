import { useState } from 'react';
import type { ConsentAction, ConsentScope, ConsentState } from '../types/audience';

interface ConsentControlProps {
  consent: ConsentState;
  onChange: (scope: ConsentScope, action: ConsentAction) => Promise<void> | void;
}

function valueOrNotSpecified(value?: string) {
  return value || 'Not specified';
}

/**
 * An explicit, purpose-limited opt-in control. Its caller owns persistence so
 * the component can be used with a real API or a local/fixture Signal safely.
 */
export function ConsentControl({ consent, onChange }: ConsentControlProps) {
  const [pendingAction, setPendingAction] = useState<ConsentAction | null>(null);
  const [error, setError] = useState<string | null>(null);
  const { scope } = consent;
  const isGranted = consent.status === 'granted';

  const changeConsent = async (action: ConsentAction) => {
    setPendingAction(action);
    setError(null);
    try {
      await onChange(scope, action);
    } catch {
      setError('Your consent could not be updated. Please try again.');
    } finally {
      setPendingAction(null);
    }
  };

  return (
    <section aria-labelledby={`consent-${scope.id}`} className="border border-border bg-background-secondary p-4">
      <h3 id={`consent-${scope.id}`} className="m-0 text-lg">Delivery consent</h3>
      <dl className="mt-3 grid gap-2 text-sm sm:grid-cols-2">
        <div><dt className="text-foreground-muted">Sender</dt><dd>{scope.sender.name}</dd></div>
        <div><dt className="text-foreground-muted">Channel</dt><dd>{scope.channel}</dd></div>
        <div><dt className="text-foreground-muted">Purpose</dt><dd>{scope.purpose}</dd></div>
        <div><dt className="text-foreground-muted">Frequency</dt><dd>{valueOrNotSpecified(scope.frequency)}</dd></div>
        <div><dt className="text-foreground-muted">Disclosure version</dt><dd>{scope.disclosure_version}</dd></div>
        <div><dt className="text-foreground-muted">Region</dt><dd>{valueOrNotSpecified(scope.region)}</dd></div>
        {scope.tour && <div><dt className="text-foreground-muted">Tour</dt><dd>{scope.tour.name}</dd></div>}
        {scope.place && <div><dt className="text-foreground-muted">Place</dt><dd>{scope.place.name}</dd></div>}
        <div><dt className="text-foreground-muted">Verification</dt><dd>{consent.verification_state}</dd></div>
        <div><dt className="text-foreground-muted">Consent status</dt><dd>{consent.status.replace('_', ' ')}</dd></div>
      </dl>
      <p className="mt-3 text-sm text-foreground-secondary">
        Verification, RSVPs, and membership do not grant delivery consent. You may change this choice at any time.
      </p>
      <div className="mt-4 flex flex-wrap gap-3" aria-live="polite">
        <button
          type="button"
          onClick={() => changeConsent('grant')}
          disabled={isGranted || pendingAction !== null}
          className="bg-brand-primary px-4 py-2 font-bold text-white disabled:cursor-not-allowed disabled:opacity-50"
        >
          {pendingAction === 'grant' ? 'Granting…' : `Grant ${scope.channel} consent`}
        </button>
        <button
          type="button"
          onClick={() => changeConsent('revoke')}
          disabled={!isGranted || pendingAction !== null}
          className="border border-status-error px-4 py-2 font-bold text-status-error disabled:cursor-not-allowed disabled:opacity-50"
        >
          {pendingAction === 'revoke' ? 'Revoking…' : `Revoke ${scope.channel} consent`}
        </button>
      </div>
      {error && <p role="alert" className="mt-3 text-sm text-status-error">{error}</p>}
    </section>
  );
}
