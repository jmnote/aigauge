//go:build windows

package app

import (
	"errors"
	"fmt"
	"syscall"
	"time"
	"unsafe"

	"github.com/go-ole/go-ole"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	appModelErrorNoPackage = 15700
	asyncStatusCompleted   = 1
)

var (
	kernel32                  = windows.NewLazySystemDLL("kernel32.dll")
	getCurrentPackageFullName = kernel32.NewProc("GetCurrentPackageFullName")
)

func isPackagedWindowsApp() bool {
	// GetCurrentPackageFullName returns APPMODEL_ERROR_NO_PACKAGE for the
	// portable build. This avoids relying on the executable's install path,
	// which is not stable for MSIX packages.
	var length uint32
	r, _, _ := getCurrentPackageFullName.Call(uintptr(unsafe.Pointer(&length)), 0)
	return r != appModelErrorNoPackage
}

func getPackagedStartWithWindowsState() (string, error) {
	task, err := getStartupTask()
	if err != nil {
		return StartWithWindowsOff, err
	}
	defer release(task)

	state, err := startupTaskState(task)
	if err != nil {
		return StartWithWindowsOff, err
	}
	if state != 2 && state != 4 { // Enabled / EnabledByPolicy
		return StartWithWindowsOff, nil
	}

	mode := StartWithWindowsShow
	key, err := registry.OpenKey(registry.CURRENT_USER, runRegistryKey, registry.QUERY_VALUE)
	if err == nil {
		if value, _, readErr := key.GetStringValue(startupModeName); readErr == nil && value == StartWithWindowsInTray {
			mode = StartWithWindowsInTray
		}
		key.Close()
	}
	return mode, nil
}

func setPackagedStartWithWindows(state string) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, runRegistryKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	if err := key.SetStringValue(startupModeName, state); err != nil {
		return err
	}

	task, err := getStartupTask()
	if err != nil {
		return err
	}
	defer release(task)
	if state == StartWithWindowsOff {
		return startupTaskDisable(task)
	}
	return startupTaskEnable(task)
}

func getStartupTask() (*ole.IInspectable, error) {
	if err := ole.RoInitialize(1); err != nil && !isRoAlreadyInitialized(err) {
		return nil, err
	}
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

func startupTaskEnable(task *ole.IInspectable) error {
	var operation *ole.IInspectable
	hr := call(vtable(task)[6], uintptr(unsafe.Pointer(task)), uintptr(unsafe.Pointer(&operation)))
	if hr != 0 {
		return hresultError("StartupTask.RequestEnableAsync", hr)
	}
	defer release(operation)
	return waitAsync(operation)
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
	for range 100 {
		var status int32
		hr = call(vtable(info)[7], uintptr(unsafe.Pointer(info)), uintptr(unsafe.Pointer(&status)))
		if hr != 0 {
			return hresultError("IAsyncInfo.Status", hr)
		}
		if status == asyncStatusCompleted {
			return nil
		}
		if status == 3 { // Error
			var code int32
			_ = call(vtable(info)[8], uintptr(unsafe.Pointer(info)), uintptr(unsafe.Pointer(&code)))
			return hresultError("StartupTask async operation", uintptr(code))
		}
		time.Sleep(10 * time.Millisecond)
	}
	return fmt.Errorf("startup task operation timed out")
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

func isRoAlreadyInitialized(err error) bool {
	var oleErr *ole.OleError
	return errors.As(err, &oleErr) && oleErr.Code() == 0x80010106
}
