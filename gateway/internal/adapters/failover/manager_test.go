package failover

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/tu-org/iptv-ecosystem/gateway/internal/domain"
)

func makeLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, nil))
}

func TestManager_BestMirror_SelectsHealthiest(t *testing.T) {
	mgr := NewManager(makeLogger())

	mirrors := []domain.LiveStreamMirror{
		{
			ID:       "mirror-1",
			Priority: 1,
			IsAlive:  true,
			Health:   domain.MirrorHealth{LatencyMs: 300, BitrateKbps: 2000, BufferRatio: 0.5},
		},
		{
			ID:       "mirror-2",
			Priority: 2,
			IsAlive:  true,
			Health:   domain.MirrorHealth{LatencyMs: 100, BitrateKbps: 5000, BufferRatio: 0.3},
		},
	}

	mgr.RegisterMirrors("event-123", mirrors)

	best, found := mgr.BestMirror(context.Background(), "event-123")
	if !found {
		t.Fatal("Expected a mirror to be found")
	}
	// El mirror de prioridad 1 viene primero (menor número = más prioritario)
	if best.ID != "mirror-1" {
		t.Errorf("Expected mirror-1 (priority 1), got %s", best.ID)
	}
}

func TestManager_MarkDead_Fallback(t *testing.T) {
	mgr := NewManager(makeLogger())

	mirrors := []domain.LiveStreamMirror{
		{ID: "m1", Priority: 1, IsAlive: true, Health: domain.MirrorHealth{LatencyMs: 50, BufferRatio: 0.3}},
		{ID: "m2", Priority: 2, IsAlive: true, Health: domain.MirrorHealth{LatencyMs: 150, BufferRatio: 0.4}},
	}

	mgr.RegisterMirrors("event-456", mirrors)
	mgr.MarkDead("event-456", "m1")

	best, found := mgr.BestMirror(context.Background(), "event-456")
	if !found {
		t.Fatal("Expected fallback mirror to be found")
	}
	if best.ID != "m2" {
		t.Errorf("Expected failover to m2, got %s", best.ID)
	}
}

func TestManager_AllDead_NoMirror(t *testing.T) {
	mgr := NewManager(makeLogger())

	mirrors := []domain.LiveStreamMirror{
		{ID: "m1", Priority: 1, IsAlive: false},
		{ID: "m2", Priority: 2, IsAlive: false},
	}

	mgr.RegisterMirrors("event-789", mirrors)

	_, found := mgr.BestMirror(context.Background(), "event-789")
	if found {
		t.Error("Expected no mirror when all are dead")
	}
}

func TestManager_MarkAlive_UpdatesHealth(t *testing.T) {
	mgr := NewManager(makeLogger())

	mirrors := []domain.LiveStreamMirror{
		{ID: "m1", Priority: 1, IsAlive: false},
	}
	mgr.RegisterMirrors("event-000", mirrors)
	mgr.MarkDead("event-000", "m1")

	newHealth := domain.MirrorHealth{
		LatencyMs:   50,
		BitrateKbps: 3000,
		BufferRatio: 0.2,
		LastChecked: time.Now(),
	}
	mgr.MarkAlive("event-000", "m1", newHealth)

	best, found := mgr.BestMirror(context.Background(), "event-000")
	if !found {
		t.Fatal("Expected mirror to be alive after MarkAlive")
	}
	if best.Health.BitrateKbps != 3000 {
		t.Errorf("Expected BitrateKbps=3000, got %d", best.Health.BitrateKbps)
	}
}

func TestManager_ConcurrentAccess_NoRace(t *testing.T) {
	mgr := NewManager(makeLogger())
	mirrors := []domain.LiveStreamMirror{
		{ID: "m1", Priority: 1, IsAlive: true, Health: domain.MirrorHealth{BufferRatio: 0.4}},
	}
	mgr.RegisterMirrors("race-event", mirrors)

	done := make(chan struct{})

	// Lecturas concurrentes
	for i := 0; i < 20; i++ {
		go func() {
			mgr.BestMirror(context.Background(), "race-event")
			done <- struct{}{}
		}()
	}

	// Escrituras concurrentes
	for i := 0; i < 5; i++ {
		go func() {
			mgr.MarkDead("race-event", "m1")
			mgr.MarkAlive("race-event", "m1", domain.MirrorHealth{BufferRatio: 0.3})
			done <- struct{}{}
		}()
	}

	// Esperar a todos
	for i := 0; i < 25; i++ {
		<-done
	}
}
