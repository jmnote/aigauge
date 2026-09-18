//go:build windows

package app

import (
	"fmt"
	"runtime"
	"syscall"
	"time"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/jmnote/aigauge/internal/config"
	"golang.org/x/sys/windows"
)

const (
	appModelErrorNoPackage           = 15700
	asyncStatusCompleted             = 1
	asyncStatusCanceled              = 2
	asyncStatusError                 = 3
	startupTaskStateDisabled         = 1
	startupTaskStateEnabled          = 2
	startupTaskStateDisabledByPolicy = 3
	startupTaskStateEnabledByPolicy  = 4
	startupAsyncTimeout              = 30 * time.Second
)

var (
	kernel32                  = windows.NewLazySystemDLL("kernel32.dll")
	getCurrentPackageFullName = kernel32.NewProc("GetCurrentPackageFullName")
	combase                   = windows.NewLazySystemDLL("combase.dll")
	roInitialize              = combase.NewProc("RoInitialize")
	roUninitialize            = combase.NewProc("RoUninitialize")
)

// Pin the complete COM lifetime, including Release, to the initialized thread.
// S_OK and S_FALSE both acquire a reference that must be balanced. A different
// existing apartment can be used, but does not acquire a reference here.
func initializeStartupRuntime() (func(), error) {
	runtime.LockOSThread()
	hr, _, _ := roInitialize.Call(1) // RO_INIT_MULTITHREADED
	initialized, err := startupRuntimeResult(uint32(hr))
	if err != nil {
		runtime.UnlockOSThread()
		return nil, err
	}
	return func() {
		if initialized {
			roUninitialize.Call()
		}
		runtime.UnlockOSThread()
	}, nil
}

func startupRuntimeResult(hr uint32) (bool, error) {
	if hr == 0 || hr == 1 { // S_OK / S_FALSE
		return true, nil
	}
	if hr == 0x80010106 { // RPC_E_CHANGED_MODE: use the existing apartment
		return false, nil
	}
	return false, hresultError("RoInitialize", uintptr(hr))
}

func isPackagedWindowsApp() bool {
	// GetCurrentPackageFullName returns APPMODEL_ERROR_NO_PACKAGE for the
	// portable build. This avoids relying on the executable's install path,
	// which is not stable for MSIX packages.
	var length uint32
	r, _, _ := getCurrentPackageFullName.Call(uintptr(unsafe.Pointer(&length)), 0)
	return r != appModelErrorNoPackage
}

func getPackagedStartWithWindowsState() (string, error) {
	finish, err := initializeStartupRuntime()
	if err != nil {
		return StartWithWindowsOff, err
	}
	defer finish()
	task, err := getStartupTask()
	if err != nil {
		return StartWithWindowsOff, err
	}
	defer release(task)

	state, err := startupTaskState(task)
	if err != nil {
		return StartWithWindowsOff, err
	}
	if !isStartupTaskEnabled(state) {
		return StartWithWindowsOff, nil
	}

	settings, err := config.Load()
	if err != nil {
		return StartWithWindowsOff, err
	}
	if settings.StartupMode == StartWithWindowsInTray {
		return StartWithWindowsInTray, nil
	}
	return StartWithWindowsShow, nil
}

func setPackagedStartWithWindows(state string) error {
	finish, err := initializeStartupRuntime()
	if err != nil {
		return err
	}
	defer finish()

	task, err := getStartupTask()
	if err != nil {
		return err
	}
	defer release(task)
	if state == StartWithWindowsOff {
		if err := startupTaskDisable(task); err != nil {
			return err
		}
		actual, err := startupTaskState(task)
		if err != nil {
			return err
		}
		if isStartupTaskEnabled(actual) {
			return fmt.Errorf("Windows policy keeps this startup task enabled")
		}
		return nil
	}
	actual, err := startupTaskEnable(task)
	if err != nil {
		return err
	}
	return checkStartupEnabled(actual)
}

func getStartupTask() (*ole.IInspectable, error) {
	statics, err := ole.RoGetActivationFactory("Windows.ApplicationModel.StartupTask", ole.NewGUID("{EE5B60BD-A148-41A7-B26E-E8B88A1E62F8}"))
	if err != nil {
		return nil, err
	}
	defer release(statics)

	h, err := ole.NewHString(startupTaskID)
	if err != nil {
		return nil, err
	}
	defer ole.DeleteHString(h)

	var operation *ole.IInspectable
	hr := call(vtable(statics)[7], uintptr(unsafe.Pointer(statics)), uintptr(h), uintptr(unsafe.Pointer(&operation)))
	if hr != 0 {
		return nil, hresultError("StartupTask.GetAsync", hr)
	}
	defer release(operation)
	if err := waitAsync(operation); err != nil {
		return nil, err
	}

	var task *ole.IInspectable
	hr = call(vtable(operation)[8], uintptr(unsafe.Pointer(operation)), uintptr(unsafe.Pointer(&task)))
	if hr != 0 {
		return nil, hresultError("StartupTask.GetResults", hr)
	}
	return task, nil
}

func startupTaskState(task *ole.IInspectable) (int32, error) {
	var state int32
	hr := call(vtable(task)[8], uintptr(unsafe.Pointer(task)), uintptr(unsafe.Pointer(&state)))
	if hr != 0 {
		return 0, hresultError("StartupTask.State", hr)
	}
	return state, nil
}

func startupTaskEnable(task *ole.IInspectable) (int32, error) {
	var operation *ole.IInspectable
	hr := call(vtable(task)[6], uintptr(unsafe.Pointer(task)), uintptr(unsafe.Pointer(&operation)))
	if hr != 0 {
		return 0, hresultError("StartupTask.RequestEnableAsync", hr)
	}
	defer release(operation)
	if err := waitAsync(operation); err != nil {
		return 0, err
	}
	var state int32
	hr = call(vtable(operation)[8], uintptr(unsafe.Pointer(operation)), uintptr(unsafe.Pointer(&state)))
	if hr != 0 {
		return 0, hresultError("StartupTask.RequestEnableAsync.GetResults", hr)
	}
	return state, nil
}

func checkStartupEnabled(state int32) error {
	switch state {
	case startupTaskStateEnabled, startupTaskStateEnabledByPolicy:
		return nil
	case startupTaskStateDisabled:
		return fmt.Errorf("startup is disabled by the user; enable AI Gauge in Windows Settings or Task Manager")
	case startupTaskStateDisabledByPolicy:
		return fmt.Errorf("startup is disabled by Windows policy")
	default:
		return fmt.Errorf("Windows did not enable the startup task (state %d)", state)
	}
}

func isStartupTaskEnabled(state int32) bool {
	return state == startupTaskStateEnabled || state == startupTaskStateEnabledByPolicy
}

func startupTaskDisable(task *ole.IInspectable) error {
	hr := call(vtable(task)[7], uintptr(unsafe.Pointer(task)))
	if hr != 0 {
		return hresultError("StartupTask.Disable", hr)
	}
	return nil
}

func waitAsync(operation *ole.IInspectable) error {
	var info *ole.IInspectable
	iid := ole.NewGUID("{00000036-0000-0000-C000-000000000046}")
	hr := call(vtable(operation)[0], uintptr(unsafe.Pointer(operation)), uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&info)))
	if hr != 0 {
		return hresultError("IAsyncInfo.QueryInterface", hr)
	}
	defer release(info)
	deadline := time.Now().Add(startupAsyncTimeout)
	for {
		var status int32
		hr = call(vtable(info)[7], uintptr(unsafe.Pointer(info)), uintptr(unsafe.Pointer(&status)))
		if hr != 0 {
			return hresultError("IAsyncInfo.Status", hr)
		}
		if status == asyncStatusCompleted {
			return nil
		}
		if status == asyncStatusCanceled {
			return fmt.Errorf("startup task operation was canceled")
		}
		if status == asyncStatusError {
			var code int32
			_ = call(vtable(info)[8], uintptr(unsafe.Pointer(info)), uintptr(unsafe.Pointer(&code)))
			return hresultError("StartupTask async operation", uintptr(code))
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("startup task operation timed out after %s", startupAsyncTimeout)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func vtable(value *ole.IInspectable) []uintptr {
	table := *(*unsafe.Pointer)(unsafe.Pointer(value))
	return unsafe.Slice((*uintptr)(table), 11)
}

func call(function uintptr, args ...uintptr) uintptr {
	r, _, _ := syscall.SyscallN(function, args...)
	return r
}

func release(value *ole.IInspectable) {
	if value != nil {
		call(vtable(value)[2], uintptr(unsafe.Pointer(value)))
	}
}

func hresultError(operation string, hr uintptr) error {
	return fmt.Errorf("%s failed (HRESULT 0x%08x)", operation, uint32(hr))
}
