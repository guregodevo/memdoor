package presence

import (
	"testing"
	"time"

	"memdoor/gateway/broadcast"
	"memdoor/pkg/shared"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockBroadcaster captures broadcast events for testing
type mockBroadcaster struct {
	events []broadcast.Event
}

func (m *mockBroadcaster) BroadcastToAll(event broadcast.Event) {
	m.events = append(m.events, event)
}

func (m *mockBroadcaster) BroadcastToSession(sessionID string, event broadcast.Event) {}
func (m *mockBroadcaster) BroadcastToRun(runID string, event broadcast.Event)         {}

func TestNewTracker(t *testing.T) {
	broadcaster := &mockBroadcaster{}
	tracker := NewTracker(broadcaster)

	require.NotNil(t, tracker)
	assert.NotNil(t, tracker.actorConnections)
	assert.NotNil(t, tracker.lastSeen)
	assert.Equal(t, broadcaster, tracker.broadcaster)
}

func TestConnect_FirstConnection(t *testing.T) {
	broadcaster := &mockBroadcaster{}
	tracker := NewTracker(broadcaster)

	actorID := shared.NewHumanActorID("user1")
	clientID := "client1"

	tracker.Connect(actorID, clientID)

	// Should be online
	assert.Equal(t, StatusOnline, tracker.GetStatus(actorID))

	// Should have 1 connection
	assert.Equal(t, 1, tracker.GetConnectionCount(actorID))

	// Should broadcast online event
	require.Len(t, broadcaster.events, 1)
	assert.Equal(t, "presence.updated", broadcaster.events[0].Type)
	assert.Equal(t, actorID.String(), broadcaster.events[0].Data["actor_id"])
	assert.Equal(t, StatusOnline, broadcaster.events[0].Data["status"])
}

func TestConnect_MultipleConnections(t *testing.T) {
	broadcaster := &mockBroadcaster{}
	tracker := NewTracker(broadcaster)

	actorID := shared.NewHumanActorID("user1")

	// First connection
	tracker.Connect(actorID, "client1")
	assert.Len(t, broadcaster.events, 1) // Online broadcast

	// Second connection (same user, different tab)
	tracker.Connect(actorID, "client2")

	// Should still be online
	assert.Equal(t, StatusOnline, tracker.GetStatus(actorID))

	// Should have 2 connections
	assert.Equal(t, 2, tracker.GetConnectionCount(actorID))

	// Should NOT broadcast again (already online)
	assert.Len(t, broadcaster.events, 1)
}

func TestDisconnect_LastConnection(t *testing.T) {
	broadcaster := &mockBroadcaster{}
	tracker := NewTracker(broadcaster)

	actorID := shared.NewHumanActorID("user1")
	clientID := "client1"

	// Connect then disconnect
	tracker.Connect(actorID, clientID)
	broadcaster.events = nil // Clear online event

	tracker.Disconnect(actorID, clientID)

	// Should be offline
	assert.Equal(t, StatusOffline, tracker.GetStatus(actorID))

	// Should have 0 connections
	assert.Equal(t, 0, tracker.GetConnectionCount(actorID))

	// Should broadcast offline event
	require.Len(t, broadcaster.events, 1)
	assert.Equal(t, "presence.updated", broadcaster.events[0].Type)
	assert.Equal(t, actorID.String(), broadcaster.events[0].Data["actor_id"])
	assert.Equal(t, StatusOffline, broadcaster.events[0].Data["status"])
}

func TestDisconnect_OneOfMultipleConnections(t *testing.T) {
	broadcaster := &mockBroadcaster{}
	tracker := NewTracker(broadcaster)

	actorID := shared.NewHumanActorID("user1")

	// Connect two clients
	tracker.Connect(actorID, "client1")
	tracker.Connect(actorID, "client2")
	broadcaster.events = nil // Clear events

	// Disconnect one client
	tracker.Disconnect(actorID, "client1")

	// Should still be online (client2 still connected)
	assert.Equal(t, StatusOnline, tracker.GetStatus(actorID))

	// Should have 1 connection remaining
	assert.Equal(t, 1, tracker.GetConnectionCount(actorID))

	// Should NOT broadcast offline (still has connections)
	assert.Len(t, broadcaster.events, 0)
}

func TestDisconnect_NonExistentConnection(t *testing.T) {
	broadcaster := &mockBroadcaster{}
	tracker := NewTracker(broadcaster)

	actorID := shared.NewHumanActorID("user1")

	// Disconnect without ever connecting (should not panic)
	tracker.Disconnect(actorID, "client1")

	// Should be offline
	assert.Equal(t, StatusOffline, tracker.GetStatus(actorID))

	// Should not broadcast anything
	assert.Len(t, broadcaster.events, 0)
}

func TestGetOnlineActors(t *testing.T) {
	broadcaster := &mockBroadcaster{}
	tracker := NewTracker(broadcaster)

	user1 := shared.NewHumanActorID("user1")
	user2 := shared.NewHumanActorID("user2")
	agent1 := shared.NewAgentActorID("agent1")

	// Initially empty
	assert.Empty(t, tracker.GetOnlineActors())

	// Connect three actors
	tracker.Connect(user1, "client1")
	tracker.Connect(user2, "client2")
	tracker.Connect(agent1, "client3")

	online := tracker.GetOnlineActors()
	assert.Len(t, online, 3)
	assert.Contains(t, online, user1.String())
	assert.Contains(t, online, user2.String())
	assert.Contains(t, online, agent1.String())

	// Disconnect one
	tracker.Disconnect(user1, "client1")

	online = tracker.GetOnlineActors()
	assert.Len(t, online, 2)
	assert.NotContains(t, online, user1.String())
}

func TestGetLastSeen(t *testing.T) {
	broadcaster := &mockBroadcaster{}
	tracker := NewTracker(broadcaster)

	actorID := shared.NewHumanActorID("user1")

	// No last seen initially
	lastSeen := tracker.GetLastSeen(actorID)
	assert.True(t, lastSeen.IsZero())

	// Connect
	before := time.Now()
	tracker.Connect(actorID, "client1")
	after := time.Now()

	lastSeen = tracker.GetLastSeen(actorID)
	assert.False(t, lastSeen.IsZero())
	assert.True(t, lastSeen.After(before) || lastSeen.Equal(before))
	assert.True(t, lastSeen.Before(after) || lastSeen.Equal(after))

	// Disconnect updates last seen
	time.Sleep(10 * time.Millisecond)
	before2 := time.Now()
	tracker.Disconnect(actorID, "client1")
	after2 := time.Now()

	lastSeen2 := tracker.GetLastSeen(actorID)
	assert.True(t, lastSeen2.After(lastSeen))
	assert.True(t, lastSeen2.After(before2) || lastSeen2.Equal(before2))
	assert.True(t, lastSeen2.Before(after2) || lastSeen2.Equal(after2))
}

func TestGetPresenceMap_AllActors(t *testing.T) {
	broadcaster := &mockBroadcaster{}
	tracker := NewTracker(broadcaster)

	user1 := shared.NewHumanActorID("user1")
	user2 := shared.NewHumanActorID("user2")

	tracker.Connect(user1, "client1")
	tracker.Connect(user2, "client2")

	// Get all actors (empty filter)
	presenceMap := tracker.GetPresenceMap(nil)

	assert.Len(t, presenceMap, 2)
	assert.Equal(t, StatusOnline, presenceMap[user1.String()])
	assert.Equal(t, StatusOnline, presenceMap[user2.String()])
}

func TestGetPresenceMap_SpecificActors(t *testing.T) {
	broadcaster := &mockBroadcaster{}
	tracker := NewTracker(broadcaster)

	user1 := shared.NewHumanActorID("user1")
	user2 := shared.NewHumanActorID("user2")
	user3 := shared.NewHumanActorID("user3")

	// Only user1 is online
	tracker.Connect(user1, "client1")

	// Query for all three users
	presenceMap := tracker.GetPresenceMap([]shared.ActorID{user1, user2, user3})

	assert.Len(t, presenceMap, 3)
	assert.Equal(t, StatusOnline, presenceMap[user1.String()])
	assert.Equal(t, StatusOffline, presenceMap[user2.String()])
	assert.Equal(t, StatusOffline, presenceMap[user3.String()])
}

func TestCleanup(t *testing.T) {
	broadcaster := &mockBroadcaster{}
	tracker := NewTracker(broadcaster)

	user1 := shared.NewHumanActorID("user1")

	tracker.Connect(user1, "client1")
	tracker.Disconnect(user1, "client1")

	// Before cleanup, entry exists (but empty)
	tracker.mu.RLock()
	_, exists := tracker.actorConnections[user1.String()]
	tracker.mu.RUnlock()
	assert.False(t, exists) // Disconnect already cleaned up

	// Cleanup should not panic
	tracker.Cleanup()
}

func TestConcurrency(t *testing.T) {
	broadcaster := &mockBroadcaster{}
	tracker := NewTracker(broadcaster)

	actorID := shared.NewHumanActorID("user1")

	// Simulate concurrent connections/disconnections
	done := make(chan bool)

	// Goroutine 1: Connect
	go func() {
		for i := 0; i < 100; i++ {
			tracker.Connect(actorID, "client1")
		}
		done <- true
	}()

	// Goroutine 2: Disconnect
	go func() {
		for i := 0; i < 100; i++ {
			tracker.Disconnect(actorID, "client1")
		}
		done <- true
	}()

	// Goroutine 3: GetStatus
	go func() {
		for i := 0; i < 100; i++ {
			tracker.GetStatus(actorID)
		}
		done <- true
	}()

	// Wait for all goroutines
	<-done
	<-done
	<-done

	// Should not panic, final state should be consistent
	status := tracker.GetStatus(actorID)
	assert.Contains(t, []string{StatusOnline, StatusOffline}, status)
}

func TestNilBroadcaster(t *testing.T) {
	tracker := NewTracker(nil)

	actorID := shared.NewHumanActorID("user1")

	// Should not panic with nil broadcaster
	tracker.Connect(actorID, "client1")
	tracker.Disconnect(actorID, "client1")
}
