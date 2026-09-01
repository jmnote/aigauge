package ui

import "testing"

func TestClampAxisLeavesPositionUnchangedFarFromEdges(t *testing.T) {
	// origin=0, extent=1000, size=200: window spans [400, 600], well clear of
	// both edges (more than edgeSnapThreshold from either).
	if got := clampAxis(400, 200, 0, 1000); got != 400 {
		t.Errorf("clampAxis() = %d, want 400 (unchanged)", got)
	}
}

func TestClampAxisSnapsToLowEdgeWithinThreshold(t *testing.T) {
	// pos=10 is within edgeSnapThreshold (20) of origin=0.
	if got := clampAxis(10, 200, 0, 1000); got != 0 {
		t.Errorf("clampAxis() = %d, want 0 (snapped to low edge)", got)
	}
}

func TestClampAxisSnapsToHighEdgeWithinThreshold(t *testing.T) {
	// window spans [780, 980]; screen right edge is at 1000, a 20px gap
	// within edgeSnapThreshold, and pos=780 is far from the left edge.
	if got := clampAxis(780, 200, 0, 1000); got != 800 {
		t.Errorf("clampAxis() = %d, want 800 (snapped to high edge)", got)
	}
}

func TestClampAxisDoesNotSnapJustOutsideThreshold(t *testing.T) {
	// pos=25 is just past edgeSnapThreshold (20) from origin=0.
	if got := clampAxis(25, 200, 0, 1000); got != 25 {
		t.Errorf("clampAxis() = %d, want 25 (unchanged, outside snap range)", got)
	}
}

func TestClampAxisPullsBackFromNegativeOverhang(t *testing.T) {
	// Window dragged partly off-screen to the left; should be pulled fully
	// on-screen rather than snapped based on distance.
	if got := clampAxis(-50, 200, 0, 1000); got != 0 {
		t.Errorf("clampAxis() = %d, want 0 (pulled back on-screen)", got)
	}
}

func TestClampAxisPullsBackFromPositiveOverhang(t *testing.T) {
	// Window dragged partly off-screen to the right (pos+size > origin+extent).
	if got := clampAxis(950, 200, 0, 1000); got != 800 {
		t.Errorf("clampAxis() = %d, want 800 (pulled back on-screen)", got)
	}
}

func TestClampAxisPinsToOriginWhenWindowLargerThanScreen(t *testing.T) {
	if got := clampAxis(100, 1200, 0, 1000); got != 0 {
		t.Errorf("clampAxis() = %d, want 0 (pinned to origin)", got)
	}
}
