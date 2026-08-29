//go:build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

// Win32 Lazy Loaders
var (
	user32                      = syscall.NewLazyDLL("user32.dll")
	getDisplayConfigBufferSizes = user32.NewProc("GetDisplayConfigBufferSizes")
	queryDisplayConfig          = user32.NewProc("QueryDisplayConfig")
	displayConfigGetDeviceInfo  = user32.NewProc("DisplayConfigGetDeviceInfo")
	enumDisplayDevices          = user32.NewProc("EnumDisplayDevicesW")
)

// Win32 Constants
const (
	QDC_ONLY_ACTIVE_PATHS                     = 0x0002
	DISPLAYCONFIG_DEVICE_INFO_GET_TARGET_NAME = 2
)

// LUID represents 64-bit Locally Unique Identifier
type LUID struct {
	LowPart  uint32
	HighPart int32
}

type DISPLAYCONFIG_HEADER struct {
	Type      uint32
	Size      uint32
	AdapterId LUID
	Id        uint32
}

type DISPLAYCONFIG_PATH_SOURCE_INFO struct {
	AdapterId   LUID
	Id          uint32
	ModeInfoIdx uint32
	StatusFlags uint32
}

type DISPLAYCONFIG_PATH_TARGET_INFO struct {
	AdapterId              LUID
	Id                     uint32
	ModeInfoIdx            uint32
	OutputTechnology       uint32
	Rotation               uint32
	Scaling                uint32
	RefreshRateNumerator   uint32
	RefreshRateDenominator uint32
	ScanLineOrdering       uint32
	TargetAvailable        int32
	StatusFlags            uint32
}

type DISPLAYCONFIG_PATH_INFO struct {
	SourceInfo DISPLAYCONFIG_PATH_SOURCE_INFO
	TargetInfo DISPLAYCONFIG_PATH_TARGET_INFO
	Flags      uint32
}

// 64-byte Mode Info Placeholder Struct to satisfy buffer allocations
type DISPLAYCONFIG_MODE_INFO struct {
	InfoType  uint32
	Id        uint32
	AdapterId LUID
	Dummy     [48]byte
}

type DISPLAYCONFIG_TARGET_DEVICE_NAME struct {
	Header                    DISPLAYCONFIG_HEADER
	Flags                     uint32
	OutputTechnology          uint32
	EdidManufactureId         uint16
	EdidProductCodeId         uint16
	ConnectorInstance         uint32
	MonitorFriendlyDeviceName [64]uint16
	MonitorDevicePath         [128]uint16
}

type DISPLAY_DEVICE struct {
	Cb           uint32
	DeviceName   [32]uint16
	DeviceString [128]uint16
	StateFlags   uint32
	DeviceID     [128]uint16
	DeviceKey    [128]uint16
}

type MonitorInfo struct {
	ID          uint32
	MonitorName string
	DeviceName  string
	IsHDROn     bool
}

// GetMonitors uses native CCD APIs to list active physical display names
func GetMonitors() []MonitorInfo {
	var monitors []MonitorInfo

	var numPathArrayItems, numModeInfoArrayItems uint32

	// 1. Get buffer sizes for paths AND mode arrays
	ret, _, _ := getDisplayConfigBufferSizes.Call(
		uintptr(QDC_ONLY_ACTIVE_PATHS),
		uintptr(unsafe.Pointer(&numPathArrayItems)),
		uintptr(unsafe.Pointer(&numModeInfoArrayItems)),
	)

	if ret != 0 || numPathArrayItems == 0 {
		fmt.Printf("[CCD Debug] GetDisplayConfigBufferSizes failed with status: 0x%X\n", ret)
		return fallbackMonitors()
	}

	// 2. Allocate both buffers as required by Windows CCD API
	pathInfoArray := make([]DISPLAYCONFIG_PATH_INFO, numPathArrayItems)
	modeInfoArray := make([]DISPLAYCONFIG_MODE_INFO, numModeInfoArrayItems)

	// 3. Query active topology path info
	ret, _, _ = queryDisplayConfig.Call(
		uintptr(QDC_ONLY_ACTIVE_PATHS),
		uintptr(unsafe.Pointer(&numPathArrayItems)),
		uintptr(unsafe.Pointer(&pathInfoArray[0])),
		uintptr(unsafe.Pointer(&numModeInfoArrayItems)),
		uintptr(unsafe.Pointer(&modeInfoArray[0])),
		0,
	)

	if ret != 0 {
		fmt.Printf("[CCD Debug] QueryDisplayConfig failed with status: 0x%X\n", ret)
		return fallbackMonitors()
	}

	// 4. Retrieve physical names from targets
	for i, path := range pathInfoArray {
		var targetName DISPLAYCONFIG_TARGET_DEVICE_NAME
		targetName.Header.Type = DISPLAYCONFIG_DEVICE_INFO_GET_TARGET_NAME
		targetName.Header.Size = uint32(unsafe.Sizeof(targetName))
		targetName.Header.AdapterId = path.TargetInfo.AdapterId
		targetName.Header.Id = path.TargetInfo.Id

		retInfo, _, _ := displayConfigGetDeviceInfo.Call(uintptr(unsafe.Pointer(&targetName)))

		monitorName := ""
		if retInfo == 0 {
			monitorName = syscall.UTF16ToString(targetName.MonitorFriendlyDeviceName[:])
		} else {
			fmt.Printf("[CCD Debug] DisplayConfigGetDeviceInfo failed for path %d with status: 0x%X\n", i, retInfo)
		}

		if monitorName == "" {
			monitorName = fmt.Sprintf("Display %d (Generic)", i+1)
		}

		// Grab Video Card / Adapter name via EnumDisplayDevices
		var adapter DISPLAY_DEVICE
		adapter.Cb = uint32(unsafe.Sizeof(adapter))
		enumDisplayDevices.Call(0, uintptr(i), uintptr(unsafe.Pointer(&adapter)), 0)

		deviceName := syscall.UTF16ToString(adapter.DeviceString[:])
		monitorID := path.SourceInfo.Id
		monitors = append(monitors, MonitorInfo{
			ID:          monitorID,
			MonitorName: monitorName,
			DeviceName:  deviceName,
			IsHDROn:     CheckHDRStatus(uint32(i)),
		})
	}

	return monitors
}

func fallbackMonitors() []MonitorInfo {
	return []MonitorInfo{
		{ID: 0, MonitorName: "Display 1 (Fallback)", DeviceName: "Device 1 (Fallback)", IsHDROn: false},
	}
}

// CheckHDRStatus stub
func CheckHDRStatus(displayID uint32) bool {
	return false
}

// ToggleHDR stub
func ToggleHDR(displayID uint32, enable bool) {
	fmt.Printf("Setting HDR state for Display %d to %t\n", displayID, enable)
}
