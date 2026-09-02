package bleveocr

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOutboxTriggerCoalescesNotificationsAndSchedulesRecovery(t *testing.T) {
	trigger := NewOutboxTrigger()
	start := time.Date(2026, time.August, 17, 12, 0, 0, 0, time.UTC)
	recoveryInterval := time.Minute

	require.True(t, trigger.ShouldSchedule(start, recoveryInterval), "startup must recover a durable outbox")
	require.False(t, trigger.ShouldSchedule(start.Add(time.Second), recoveryInterval))

	trigger.Notify()
	trigger.Notify()
	require.True(t, trigger.ShouldSchedule(start.Add(2*time.Second), recoveryInterval))
	require.False(t, trigger.ShouldSchedule(start.Add(3*time.Second), recoveryInterval), "duplicate notifications must coalesce")

	require.False(t, trigger.ShouldSchedule(start.Add(61*time.Second), recoveryInterval))
	require.True(t, trigger.ShouldSchedule(start.Add(62*time.Second), recoveryInterval), "missed notifications must recover on the fallback interval")
}

func TestOutboxTriggerConsumePendingIsNonBlockingAndCoalesced(t *testing.T) {
	trigger := NewOutboxTrigger()
	require.False(t, trigger.ConsumePending())

	trigger.Notify()
	trigger.Notify()
	require.True(t, trigger.ConsumePending())
	require.False(t, trigger.ConsumePending())
}
