package components

import (
	"math"
	"testing"
	"time"
)

func TestLogoGeometry(t *testing.T) {
	lg := DefaultLogo()
	if lg.Width != defaultLogoWidth {
		t.Errorf("DefaultLogo width = %d, want %d", lg.Width, defaultLogoWidth)
	}
	if lg.Height != defaultLogoHeight {
		t.Errorf("DefaultLogo height = %d, want %d", lg.Height, defaultLogoHeight)
	}
}

func TestLogoAnimationLoops(t *testing.T) {
	lg := DefaultLogo()
	now := time.Unix(0, 0)

	if lg.Animate(now) {
		t.Fatal("first Animate should only initialize the animation clock")
	}
	interval := lg.frameInterval(now)
	if !lg.Animate(now.Add(interval)) {
		t.Fatal("animation should advance after one base frame interval")
	}
	if lg.phase <= 0 {
		t.Fatal("animation phase should advance")
	}
}

func TestLogoAnimationSpeedsUpWithTypingAndDecays(t *testing.T) {
	lg := DefaultLogo()
	now := time.Unix(100, 0)
	lg.Animate(now)
	base := lg.frameInterval(now)

	for i := 1; i <= 8; i++ {
		lg.Typed(now.Add(time.Duration(i) * 40 * time.Millisecond))
	}
	fast := lg.frameInterval(now.Add(320 * time.Millisecond))
	if fast >= base {
		t.Fatalf("typing should speed up the animation: base=%s fast=%s", base, fast)
	}

	slow := lg.frameInterval(now.Add(10 * time.Second))
	if slow != base {
		t.Fatalf("animation should return to base speed when idle: base=%s slow=%s", base, slow)
	}
}

func TestLogoPhaseAdvancesFasterWhileTyping(t *testing.T) {
	now := time.Unix(200, 0)

	base := DefaultLogo()
	base.Animate(now)
	base.Animate(now.Add(500 * time.Millisecond))

	typed := DefaultLogo()
	typed.Animate(now)
	for i := 1; i <= 8; i++ {
		typed.Typed(now.Add(time.Duration(i) * 40 * time.Millisecond))
	}
	typed.Animate(now.Add(500 * time.Millisecond))

	if typed.phase <= base.phase {
		t.Fatalf("typing should advance phase faster: base=%.2f typed=%.2f", base.phase, typed.phase)
	}
	if math.IsNaN(typed.phase) || math.IsInf(typed.phase, 0) {
		t.Fatalf("invalid phase: %v", typed.phase)
	}
}
