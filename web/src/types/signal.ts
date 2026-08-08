import type { ConsentState } from './audience';

export interface SignalSender {
  id: string;
  name: string;
  type: 'profile' | 'scene' | string;
}

export interface SignalTarget {
  id: string;
  type: string;
  title?: string;
}

/** The public, display-safe representation returned by GET /signals/:id. */
export interface Signal {
  id: string;
  title: string;
  body?: string;
  state: string;
  sender: SignalSender;
  target?: SignalTarget;
  published_at?: string;
  consent?: ConsentState;
  consent_scopes?: ConsentState[];
}

export interface SignalDetailResponse {
  signal: Signal;
  consent?: ConsentState;
  consent_scopes?: ConsentState[];
}
