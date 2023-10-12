// SPDX-License-Identifier: Unlicense OR MIT

package app

// DestroyEvent is the last event sent through
// a window event channel.
type DestroyEvent struct {
	// Err is nil for normal window closures. If a
	// window is prematurely closed, Err is the cause.
	Err error
}

// A StageEvent is generated whenever the stage of a
// Window changes.
type StageEvent struct {
	Stage Stage
}

// Stage of a Window.
type Stage uint8

const (
	// StagePaused is the stage for windows that have no on-screen representation.
	// Paused windows don't receive frames.
	StagePaused Stage = iota
	// StageInactive is the stage for windows that are visible, but not active.
	// Inactive windows receive frames.
	StageInactive
	// StageRunning is for active and visible Windows.
	// Running windows receive frames.
	StageRunning
)

// String implements fmt.Stringer.
func (l Stage) String() string {
	switch l {
	case StagePaused:
		return "StagePaused"
	case StageInactive:
		return "StageInactive"
	case StageRunning:
		return "StageRunning"
	default:
		panic("unexpected Stage value")
	}
}

func (StageEvent) ImplementsEvent()   {}
func (DestroyEvent) ImplementsEvent() {}
