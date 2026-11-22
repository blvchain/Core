package db

import (
	"strings"
	"testing"
)

// TestBlockStructValidatorRoleBounds pins the sender/receiver role range that
// BlockStructValidator enforces: the inclusive interval [1, 10_000_000]. This
// must match the gRPC layer (protos.validateAddDataRequest, gt=0 lt=10000001),
// otherwise a block accepted over one path would be rejected over the other and
// two honest nodes could disagree on validity. See AUDIT_REPORT.md (VAL-2).
func TestBlockStructValidatorRoleBounds(t *testing.T) {
	sender := newIdentity(t)
	receiver := newIdentity(t)

	cases := []struct {
		name    string
		sender  int64
		receiv  int64
		wantErr bool
	}{
		{"roles at lower bound", 1, 1, false},
		{"roles at upper bound", 10000000, 10000000, false},
		{"sender role zero", 0, 1, true},
		{"sender role too large", 10000001, 1, true},
		{"receiver role zero", 1, 0, true},
		{"receiver role too large", 1, 10000001, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			block := makeBlock(t, sender, receiver.uid, "payload", strings.Repeat("0", 64))
			block.BlockData.SenderRole = tc.sender
			block.BlockData.ReceiverRole = tc.receiv

			err := BlockStructValidator(block)
			if tc.wantErr && err == nil {
				t.Errorf("BlockStructValidator accepted out-of-range roles (sender=%d receiver=%d)", tc.sender, tc.receiv)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("BlockStructValidator rejected in-range roles (sender=%d receiver=%d): %v", tc.sender, tc.receiv, err)
			}
		})
	}
}
