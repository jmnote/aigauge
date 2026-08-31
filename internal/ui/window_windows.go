//go:build windows

package ui

import (
	"time"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
	"golang.org/x/sys/windows"
)

const (
	spiGetClientAreaAnimation = 0x1042
	swMinimize                = 6
	minimizeAnimationDuration = 200 * time.Millisecond
)

var (
	user32                = windows.NewLazySystemDLL("user32.dll")
	showWindowProc        = user32.NewProc("ShowWindow")
	systemParametersInfoW = user32.NewProc("SystemParametersInfoW")
)

func (rt *runtime) hideToTray() {
	if rt.window == nil {
		return
	}

	application.InvokeSync(func() {
		hwnd := uintptr(rt.window.NativeWindow())
		if hwnd == 0 {
			rt.window.Hide()
			return
		}

		var animationsEnabled int32
		systemParametersInfoW.Call(
			spiGetClientAreaAnimation,
			0,
			uintptr(unsafe.Pointer(&animationsEnabled)),
			0,
		)

		if animationsEnabled == 0 {
			rt.window.Hide()
			return
		}

		showWindowProc.Call(hwnd, uintptr(swMinimize))
		time.AfterFunc(minimizeAnimationDuration, func() {
			application.InvokeSync(func() {
				if rt.window != nil {
					rt.window.Hide()
				}
			})
		})
	})
}
