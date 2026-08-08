package payment

import (
	"testing"
	"time"
)

func TestConversionStoresModelAndWindow(t *testing.T) {
	clickAt := time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC)
	purchaseAt := clickAt.Add(48 * time.Hour)
	got := AttributePurchase(clickAt, purchaseAt, 7*24*time.Hour)
	if !got.Attributed || got.Model != AttributionModelLastSignalClick || got.Window != 7*24*time.Hour {
		t.Fatalf("got=%+v", got)
	}
}

func TestConversionRejectsPreClickAndExpiredPurchases(t *testing.T) {
	clickAt := time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC)
	if AttributePurchase(clickAt, clickAt.Add(-time.Second), 24*time.Hour).Attributed {
		t.Fatal("purchase before click was attributed")
	}
	if AttributePurchase(clickAt, clickAt.Add(25*time.Hour), 24*time.Hour).Attributed {
		t.Fatal("purchase outside the attribution window was attributed")
	}
}

func TestPaymentDeepCopyPreservesAttributionProvenance(t *testing.T) {
	received := time.Now().UTC()
	signalID := "signal"
	record := &PaymentRecord{
		SignalID: &signalID, AttributionModel: AttributionModelLastSignalClick,
		AttributionWindow: 7 * 24 * time.Hour, ProviderEventID: "evt_1",
		RawPayloadSHA256: "digest", ReceivedAt: &received,
	}
	copy := record.DeepCopy()
	if copy.SignalID == record.SignalID || copy.ReceivedAt == record.ReceivedAt ||
		copy.AttributionModel != record.AttributionModel || copy.AttributionWindow != record.AttributionWindow ||
		copy.ProviderEventID != record.ProviderEventID || copy.RawPayloadSHA256 != record.RawPayloadSHA256 {
		t.Fatalf("copy lost or aliased attribution fields: %#v", copy)
	}
}
