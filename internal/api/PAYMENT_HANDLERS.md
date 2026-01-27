# Payment Handlers

Documentation for Stripe Connect payment integration endpoints.

## Overview

The payment handlers enable scene owners to onboard for direct payment processing via Stripe Connect Express. Scenes can receive payments directly with transparent platform fees.

## Endpoints

### POST /payments/onboard

Creates a Stripe Connect Express onboarding link for a scene owner.

**Authentication**: Required (JWT)

**Request Body**:
```json
{
  "scene_id": "uuid"
}
```

**Response** (200 OK):
```json
{
  "url": "https://connect.stripe.com/setup/s/...",
  "expires_at": "2026-01-27T15:30:00Z"
}
```

**Error Responses**:

- `401 Unauthorized` - Authentication required
  ```json
  {
    "error": {
      "code": "unauthorized",
      "message": "authentication required"
    }
  }
  ```

- `400 Bad Request` - Missing or invalid scene_id
  ```json
  {
    "error": {
      "code": "bad_request",
      "message": "scene_id is required"
    }
  }
  ```

- `404 Not Found` - Scene not found
  ```json
  {
    "error": {
      "code": "not_found",
      "message": "scene not found"
    }
  }
  ```

- `403 Forbidden` - User is not the scene owner
  ```json
  {
    "error": {
      "code": "forbidden",
      "message": "only scene owner can onboard for payments"
    }
  }
  ```

- `400 Bad Request` - Scene already onboarded
  ```json
  {
    "error": {
      "code": "already_onboarded",
      "message": "scene is already onboarded for payments"
    }
  }
  ```

- `500 Internal Server Error` - Stripe API error
  ```json
  {
    "error": {
      "code": "internal_error",
      "message": "failed to create payment account" | "failed to create onboarding link" | "failed to save payment account"
    }
  }
  ```

## Implementation Details

### Authorization

The endpoint verifies that:
1. The request includes a valid JWT token with a user DID
2. The requesting user owns the scene (via `Scene.IsOwner()`)

### Idempotency

The endpoint prevents duplicate onboarding by checking if the scene already has a `connected_account_id`. If present, it returns a `400` error with code `already_onboarded`.

### Stripe Integration

1. Creates a Stripe Connect Express account
2. Generates an account onboarding link with type `account_onboarding`
3. Persists the `connected_account_id` to the scene immediately
4. Returns the onboarding URL and expiry timestamp (30 minutes from creation)

**Note**: Full onboarding completion tracking via webhook is a separate task (see requirements).

### Configuration

The endpoint is only registered when all required Stripe configuration is present:
- `STRIPE_API_KEY` - Stripe secret API key
- `STRIPE_ONBOARDING_RETURN_URL` - URL to redirect to after successful onboarding
- `STRIPE_ONBOARDING_REFRESH_URL` - URL to redirect to if the link expires

If any configuration is missing, the endpoint will not be available and a warning is logged.

### Database Schema

The endpoint uses the `connected_account_id` column added to the `scenes` table:

```sql
ALTER TABLE scenes ADD COLUMN connected_account_id VARCHAR(255);
CREATE INDEX idx_scenes_connected_account ON scenes(connected_account_id) 
    WHERE deleted_at IS NULL AND connected_account_id IS NOT NULL;
```

See migration `000016_add_stripe_connected_account.up.sql`.

## Security Considerations

1. **API Key Security**: Stripe API keys are never logged. The config logging masks them (e.g., `sk_live_****`).
2. **Authorization**: Only scene owners can initiate onboarding for their scenes.
3. **HTTPS URLs**: Onboarding links always use HTTPS.
4. **No Secrets in Response**: The response only contains the onboarding URL and expiry; no sensitive account details.

## Testing

Comprehensive unit tests cover:
- Success case with valid scene owner
- Unauthorized requests (no JWT)
- Non-owner attempts
- Already onboarded scenes
- Missing/invalid scene IDs
- Stripe API failures (account creation and link generation)

All tests use a mock Stripe client for deterministic behavior.

## Example Usage

### cURL

```bash
curl -X POST https://api.subcults.com/payments/onboard \
  -H "Authorization: Bearer <jwt_token>" \
  -H "Content-Type: application/json" \
  -d '{"scene_id": "123e4567-e89b-12d3-a456-426614174000"}'
```

### JavaScript

```javascript
const response = await fetch('/payments/onboard', {
  method: 'POST',
  headers: {
    'Authorization': `Bearer ${jwtToken}`,
    'Content-Type': 'application/json',
  },
  body: JSON.stringify({
    scene_id: sceneId,
  }),
});

const { url, expires_at } = await response.json();
// Redirect user to Stripe onboarding
window.location.href = url;
```

## Related Documentation

- [Stripe Connect Express Documentation](https://stripe.com/docs/connect/express-accounts)
- [Scene Handlers](SCENE_HANDLERS.md)
- Internal package: `internal/payment/stripe.go`

### POST /payments/checkout

Creates a Stripe Checkout Session for event tickets or merchandise with platform fee.

**Authentication**: Required (JWT)

**Request Body**:
```json
{
  "scene_id": "uuid",
  "event_id": "uuid (optional)",
  "items": [
    {
      "price_id": "price_xxx",
      "quantity": 2
    }
  ],
  "success_url": "https://example.com/success",
  "cancel_url": "https://example.com/cancel"
}
```

**Response** (200 OK):
```json
{
  "session_url": "https://checkout.stripe.com/pay/cs_test_...",
  "session_id": "cs_test_..."
}
```

**Error Responses**:

- `401 Unauthorized` - Authentication required
  ```json
  {
    "error": {
      "code": "unauthorized",
      "message": "authentication required"
    }
  }
  ```

- `400 Bad Request` - Missing required fields
  ```json
  {
    "error": {
      "code": "bad_request",
      "message": "scene_id is required" | "items list cannot be empty" | "success_url is required" | "cancel_url is required"
    }
  }
  ```

- `400 Bad Request` - Scene not onboarded
  ```json
  {
    "error": {
      "code": "not_onboarded",
      "message": "scene must be onboarded for payments before creating checkout session"
    }
  }
  ```

- `404 Not Found` - Scene not found
  ```json
  {
    "error": {
      "code": "not_found",
      "message": "scene not found"
    }
  }
  ```

- `500 Internal Server Error` - Stripe API error
  ```json
  {
    "error": {
      "code": "internal_error",
      "message": "failed to create checkout session"
    }
  }
  ```

## Implementation Details

### Platform Fee Calculation

The platform fee is calculated as a percentage of the total amount (default 5.0%):
- Configurable via `STRIPE_APPLICATION_FEE_PERCENT` environment variable
- Fee is specified in `application_fee_amount` field when creating checkout session
- Uses Stripe Connect's application fees with `on_behalf_of` parameter

### Payment Record Tracking

A provisional payment record is created with:
- `status`: `pending` (updated via webhook on completion)
- `session_id`: Stripe Checkout Session ID
- `amount`: Total amount in cents (placeholder until webhook confirms)
- `fee`: Platform fee in cents
- `user_did`: Authenticated user's DID
- `scene_id`: Scene receiving payment
- `event_id`: Optional event ID

### Security Considerations

1. **Price Validation**: Only Stripe Price IDs accepted; no client-submitted amounts
2. **Quantity Limits**: Maximum 100 items per line item to prevent abuse
3. **Scene Validation**: Scene must have `connected_account_id` before accepting payments
4. **Authentication**: JWT required for all checkout session creation

## Configuration

Required environment variables:
- `STRIPE_API_KEY` - Stripe secret API key
- `STRIPE_ONBOARDING_RETURN_URL` - Return URL after onboarding
- `STRIPE_ONBOARDING_REFRESH_URL` - Refresh URL for expired links
- `STRIPE_APPLICATION_FEE_PERCENT` - Platform fee percentage (default: 5.0)

## Example Usage

### cURL

```bash
curl -X POST https://api.subcults.com/payments/checkout \
  -H "Authorization: Bearer <jwt_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "scene_id": "123e4567-e89b-12d3-a456-426614174000",
    "items": [
      {
        "price_id": "price_1234567890",
        "quantity": 2
      }
    ],
    "success_url": "https://example.com/success",
    "cancel_url": "https://example.com/cancel"
  }'
```

### JavaScript

```javascript
const response = await fetch('/payments/checkout', {
  method: 'POST',
  headers: {
    'Authorization': `Bearer ${jwtToken}`,
    'Content-Type': 'application/json',
  },
  body: JSON.stringify({
    scene_id: sceneId,
    items: [
      { price_id: priceId, quantity: 1 }
    ],
    success_url: window.location.origin + '/success',
    cancel_url: window.location.origin + '/cancel',
  }),
});

const { session_url, session_id } = await response.json();
// Redirect to Stripe Checkout
window.location.href = session_url;
```

## Related Documentation

- [Stripe Checkout Sessions](https://stripe.com/docs/api/checkout/sessions)
- [Stripe Connect Application Fees](https://stripe.com/docs/connect/charges#application-fees)
- Internal packages: `internal/payment/stripe.go`, `internal/payment/model.go`
