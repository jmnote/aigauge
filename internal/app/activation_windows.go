//go:build windows

package app

import (
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/jmnote/aigauge/internal/config"
)

// StartHiddenOnLaunch applies the saved tray preference only to an MSIX startup
// task. Portable startup continues to use the explicit --hidden command line.
func StartHiddenOnLaunch() (bool, error) {
	return startHiddenOnLaunch(isStartupTaskActivation, func() (config.Settings, error) {
		if err := config.MigrateLegacyStartupMode(); err != nil {
			return config.Settings{}, err
		}
		return config.Load()
	})
}

// Packaged desktop activation is available from Windows 10 1809, our minimum
// supported version. Use the OS API; no Windows App SDK runtime is required.
// https://learn.microsoft.com/en-us/windows/apps/desktop/modernize/get-activation-info-for-packaged-apps
func isStartupTaskActivation() (bool, error) {
	if !isPackagedWindowsApp() {
		return false, nil
	}
	finish, err := initializeStartupRuntime()
	if err != nil {
		return false, err
	}
	defer finish()
	statics, err := ole.RoGetActivationFactory("Windows.ApplicationModel.AppInstance", ole.NewGUID("{9D11E77F-9EA6-47AF-A6EC-46784C5BA254}"))
	if err != nil {
		return false, err
	}
	defer release(statics)

	// IAppInstanceStatics.GetActivatedEventArgs returns IActivatedEventArgs.
	var args *ole.IInspectable
	hr := call(vtable(statics)[7], uintptr(unsafe.Pointer(statics)), uintptr(unsafe.Pointer(&args)))
	if hr != 0 {
		return false, hresultError("AppInstance.GetActivatedEventArgs", hr)
	}
	// A direct executable launch can have no activation payload.
	if args == nil {
		return false, nil
	}
	defer release(args)
	var kind int32
	hr = call(vtable(args)[6], uintptr(unsafe.Pointer(args)), uintptr(unsafe.Pointer(&kind)))
	if hr != 0 {
		return false, hresultError("IActivatedEventArgs.Kind", hr)
	}
	const activationKindStartupTask = 1020
	return kind == activationKindStartupTask, nil
}
