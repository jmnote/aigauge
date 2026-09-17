package ui

import (
	"fmt"
	"io/fs"

	usageapp "github.com/jmnote/aigauge/internal/app"
	"github.com/jmnote/aigauge/internal/config"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// settingsChangedEvent is the Wails event name both windows listen for to
// stay in sync with settings the other one just saved.
const settingsChangedEvent = "aigauge:config-updated"

var singleInstanceKey = [32]byte{
	0x61, 0x69, 0x67, 0x61, 0x75, 0x67, 0x65, 0x2d,
	0x73, 0x69, 0x6e, 0x67, 0x6c, 0x65, 0x2d, 0x69,
	0x6e, 0x73, 0x74, 0x61, 0x6e, 0x63, 0x65, 0x2d,
	0x6b, 0x65, 0x79, 0x2d, 0x76, 0x31, 0x2d, 0x30,
}

type runtime struct {
	application    *application.App
	appService     *usageapp.App
	window         *application.WebviewWindow
	settingsWindow *application.WebviewWindow
	icon           []byte
	activeHotkey   string
	windowVisible  bool
}

const (
	initialWindowWidth  = 250
	initialWindowHeight = 250
	minWindowWidth      = 200
	maxWindowWidth      = 600
)

func Run(frontendAssets fs.FS, icon []byte, startHidden ...bool) error {
	hidden := len(startHidden) > 0 && startHidden[0]
	rt := &runtime{icon: icon}
	appService := usageapp.NewApp(rt.setContentHeight, rt.setWindowWidth, rt.setAlwaysOnTop, rt.hideToTray, rt.showSettingsWindow, rt.emitSettingsChanged, rt.setGlobalHotkey)
	rt.appService = appService
	appService.SetSettingsContentHeightHandler(rt.setSettingsContentHeight)

	rt.application = application.New(application.Options{
		Name: "AI Gauge",
		Icon: icon,
		Services: []application.Service{
			application.NewService(appService),
		},
		Assets: application.AssetOptions{
			Handler: application.BundledAssetFileServer(frontendAssets),
		},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID:      "com.aigauge.app",
			EncryptionKey: singleInstanceKey,
			OnSecondInstanceLaunch: func(_ application.SecondInstanceData) {
				rt.showWindow()
			},
		},
	})
	rt.window = rt.application.Window.NewWithOptions(application.WebviewWindowOptions{
		Title: "AI Gauge",
		// Wails' asset server expects an index.html at the asset root
		// regardless of which URL a window is actually given (confirmed by a
		// dummy frontend/index.html alone fixing an otherwise-persistent 404)
		// - so the main window's page stays at the frontend root while
		// settings/provider, which are only ever opened by their explicit
		// URL, live under dialogs/.
		URL:           "/index.html",
		Width:         initialWindowWidth,
		Height:        initialWindowHeight,
		MinWidth:      minWindowWidth,
		MinHeight:     initialWindowHeight,
		MaxWidth:      maxWindowWidth,
		MaxHeight:     initialWindowHeight,
		DisableResize: false,
		Frameless:     true,
		Hidden:        hidden,
		Windows: application.WindowsWindow{
			NonClientRegionSupport: true,
		},
	})
	rt.windowVisible = !hidden
	rt.configureWindow()
	rt.configureTray()
	return rt.application.Run()
}

func (rt *runtime) setContentHeight(height int) {
	if rt.window == nil {
		return
	}
	width, currentHeight := rt.window.Size()
	// Resizing is enabled so the user can choose a width, but height remains
	// content-driven. Update the equal min/max height constraints in an order
	// that never temporarily crosses them, then apply the new content height.
	if height >= currentHeight {
		rt.window.SetMaxSize(maxWindowWidth, height)
		rt.window.SetMinSize(minWindowWidth, height)
	} else {
		rt.window.SetMinSize(minWindowWidth, height)
		rt.window.SetMaxSize(maxWindowWidth, height)
	}
	rt.window.SetSize(width, height)
	rt.clampWindow()
}

func (rt *runtime) setSettingsContentHeight(height int) {
	if rt.settingsWindow == nil {
		return
	}
	if height < 300 {
		height = 300
	}
	if height > 900 {
		height = 900
	}
	width, _ := rt.settingsWindow.Size()
	rt.settingsWindow.SetSize(width, height)
}

func (rt *runtime) setWindowWidth(width int) {
	if rt.window == nil {
		return
	}
	_, height := rt.window.Size()
	rt.window.SetSize(width, height)
	rt.clampWindow()
}

// emitSettingsChanged broadcasts a settings update to every open window (main
// and settings) so whichever one did not make the change re-renders from it,
// replacing the previous design where each window's localStorage had to be
// reconciled with the others via a storage-event/Wails-event round trip.
func (rt *runtime) emitSettingsChanged(settings config.Settings) {
	if rt.application == nil {
		return
	}
	rt.application.Event.Emit(settingsChangedEvent, settings)
}

func (rt *runtime) setAlwaysOnTop(alwaysOnTop bool) {
	if rt.window == nil {
		return
	}
	rt.window.SetAlwaysOnTop(alwaysOnTop)
}

func (rt *runtime) setGlobalHotkey(enabled bool, shortcut string) error {
	if !enabled {
		if rt.activeHotkey == "" {
			return nil
		}
		err := rt.application.GlobalShortcut.Unregister(rt.activeHotkey)
		if err == nil {
			rt.activeHotkey = ""
		}
		return err
	}

	if shortcut == "" {
		return fmt.Errorf("a global hotkey must be selected")
	}
	if shortcut == rt.activeHotkey {
		return nil
	}
	if err := rt.application.GlobalShortcut.Register(shortcut, rt.toggleWindow); err != nil {
		return err
	}
	previousHotkey := rt.activeHotkey
	if previousHotkey != "" {
		if err := rt.application.GlobalShortcut.Unregister(previousHotkey); err != nil {
			_ = rt.application.GlobalShortcut.Unregister(shortcut)
			return err
		}
	}
	rt.activeHotkey = shortcut
	return nil
}

func (rt *runtime) showWindow() {
	if rt.window == nil {
		return
	}
	rt.windowVisible = true
	rt.window.Restore()
	rt.window.Show()
	rt.window.Focus()
}

func (rt *runtime) toggleWindow() {
	if rt.windowVisible {
		rt.hideToTray()
		return
	}
	rt.showWindow()
}

func (rt *runtime) hideToTray() {
	if rt.window == nil {
		return
	}
	rt.windowVisible = false
	rt.window.Hide()
}

func (rt *runtime) showSettingsWindow() {
	if rt.settingsWindow == nil {
		rt.createSettingsWindow()
	}
	rt.settingsWindow.Restore()
	rt.settingsWindow.Show()
	rt.settingsWindow.Focus()
}

func (rt *runtime) createSettingsWindow() {
	rt.settingsWindow = rt.application.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:            "settings",
		Title:           "AI Gauge - Settings",
		URL:             "/dialogs/settings.html",
		Width:           320,
		Height:          560,
		MinWidth:        320,
		MinHeight:       300,
		MaxWidth:        320,
		MaxHeight:       900,
		DisableResize:   true,
		Frameless:       true,
		Windows:         application.WindowsWindow{NonClientRegionSupport: true},
		InitialPosition: application.WindowCentered,
		HideOnEscape:    true,
	})

	rt.settingsWindow.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		if rt.appService != nil {
			_ = rt.appService.CleanupPendingProviderInstances()
		}
		event.Cancel()
		rt.settingsWindow.Hide()
	})
}
