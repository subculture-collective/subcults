import { afterEach, describe, expect, it, vi } from 'vitest';
import { createPDSAccount, type ProvisioningResult } from './atproto-service';

describe('createPDSAccount', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('sends the password directly to the PDS endpoint', async () => {
    const fetchMock = vi.fn().mockResolvedValue({ ok: true });
    vi.stubGlobal('fetch', fetchMock);
    const invitation: ProvisioningResult = {
      request_id: 'request', handle: 'night-signal.subcult.tv', invite_code: 'invite-secret',
      pds_url: 'https://pds.subcult.tv', create_endpoint: 'https://pds.subcult.tv/xrpc/com.atproto.server.createAccount', expires_at: new Date().toISOString(),
    };
    await createPDSAccount(invitation, 'artist@example.com', 'pds-password');
    expect(fetchMock).toHaveBeenCalledOnce();
    expect(fetchMock.mock.calls[0][0]).toBe(invitation.create_endpoint);
    expect(JSON.parse(String(fetchMock.mock.calls[0][1].body))).toMatchObject({
      email: 'artist@example.com', handle: invitation.handle, password: 'pds-password', inviteCode: 'invite-secret',
    });
  });
});
