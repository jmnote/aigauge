//go:build windows

package ui

import (
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
	"golang.org/x/sys/windows"
)

const (
	spiGetClientAreaAnimation = 0x1042
	awHide                    = 0x00010000
	awSlide                   = 0x00040000
	awHorizontalPositive      = 0x00000001
	awVerticalPositive        = 0x00000004
	hideAnimationDurationMS   = 180
)

var (
	user32                = windows.NewLazySystemDLL("user32.dll")
	animateWindow         = user32.NewProc("AnimateWindow")
	systemParametersInfoW = user32.NewProc("SystemParametersInfoW")
)

func (rt *runtime) hideToTray() {
	if rt.window == nil {
		return
	}

	application.InvokeSync(func() {
		var animationsEnabled int32
		systemParametersInfoW.Call(
			spiGetClientAreaAnimation,
			0,
			uintptr(unsafe.Pointer(&animationsEnabled)),
			0,
		)

		if animationsEnabled != 0 {
			hwnd := uintptr(rt.window.NativeWindow())
			if hwnd != 0 {
				animateWindow.Call(
					hwnd,
					hideAnimationDurationMS,
					awHide|awSlide|awHorizontalPositive|awVerticalPositive,
				)
			}
		}

		rt.window.Hide()
	})
}
