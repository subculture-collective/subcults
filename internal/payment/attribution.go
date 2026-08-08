package payment

import "time"

const AttributionModelLastSignalClick = "last_signal_click"

// Attribution describes a modeled temporal relationship. Attributed never
// means that the Signal caused the purchase.
type Attribution struct {
	Attributed bool
	Model      string
	Window     time.Duration
	ClickAt    time.Time
	PurchaseAt time.Time
}

// AttributePurchase applies a bounded last-Signal-click model.
func AttributePurchase(clickAt, purchaseAt time.Time, window time.Duration) Attribution {
	result := Attribution{
		Model: AttributionModelLastSignalClick, Window: window,
		ClickAt: clickAt, PurchaseAt: purchaseAt,
	}
	if window < 0 || clickAt.IsZero() || purchaseAt.IsZero() || purchaseAt.Before(clickAt) {
		return result
	}
	result.Attributed = purchaseAt.Sub(clickAt) <= window
	return result
}
