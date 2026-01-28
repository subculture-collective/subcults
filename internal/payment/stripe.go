// Package payment provides Stripe integration for payment processing and Connect onboarding.
package payment

import (
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/account"
	"github.com/stripe/stripe-go/v81/accountlink"
)

// Client is an interface for Stripe operations to enable testing with mocks.
type Client interface {
	CreateConnectAccount() (*stripe.Account, error)
	CreateAccountLink(accountID, returnURL, refreshURL string) (*stripe.AccountLink, error)
}

// StripeClient implements the Client interface using the real Stripe SDK.
type StripeClient struct{}

// NewStripeClient creates a new Stripe client with the given API key.
func NewStripeClient(apiKey string) *StripeClient {
	stripe.Key = apiKey
	return &StripeClient{}
}

// CreateConnectAccount creates a new Stripe Connect Express account.
func (c *StripeClient) CreateConnectAccount() (*stripe.Account, error) {
	params := &stripe.AccountParams{
		Type: stripe.String(string(stripe.AccountTypeExpress)),
	}

	return account.New(params)
}

// CreateAccountLink creates an account onboarding link for a Stripe Connect account.
func (c *StripeClient) CreateAccountLink(accountID, returnURL, refreshURL string) (*stripe.AccountLink, error) {
	params := &stripe.AccountLinkParams{
		Account:    stripe.String(accountID),
		ReturnURL:  stripe.String(returnURL),
		RefreshURL: stripe.String(refreshURL),
		Type:       stripe.String("account_onboarding"),
	}

	return accountlink.New(params)
}
