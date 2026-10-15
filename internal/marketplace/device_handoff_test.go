package marketplace

import (
	"errors"
	"testing"
)

func TestOrderHandoffControlsStatusWriter(t *testing.T) {
	tests := []struct {
		name      string
		handoff   bool
		accountID string
		wantErr   error
	}{
		{name: "seller before handoff", accountID: "seller-7"},
		{name: "buyer after handoff", handoff: true, accountID: "buyer-9"},
		{name: "seller rejected after handoff", handoff: true, accountID: "seller-7", wantErr: ErrNotOwner},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewRegistry([]Device{{ID: "sensor-42", OwnerID: "seller-7", Status: "listed"}})
			if tt.handoff {
				if _, err := registry.Handoff("sensor-42", "seller-7", "buyer-9"); err != nil {
					t.Fatalf("handoff: %v", err)
				}
			}
			got, err := registry.UpdateStatus("sensor-42", tt.accountID, "online")
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("UpdateStatus error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil && got.Status != "online" {
				t.Fatalf("status = %q, want online", got.Status)
			}
		})
	}
}
