//go:build windows

package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

var (
	user32                      = syscall.NewLazyDLL("user32.dll")
	getDisplayConfigBufferSizes = user32.NewProc("GetDisplayConfigBufferSizes")
	queryDisplayConfig          = user32.NewProc("QueryDisplayConfig")
	displayConfigGetDeviceInfo  = user32.NewProc("DisplayConfigGetDeviceInfo")
	displayConfigSetDeviceInfo  = user32.NewProc("DisplayConfigSetDeviceInfo")
	enumDisplayDevices          = user32.NewProc("EnumDisplayDevicesW")
)

const (
	QDC_ONLY_ACTIVE_PATHS                              = 0x0002
	ERROR_INSUFFICIENT_BUFFER                          = 0x7A
	DISPLAYCONFIG_DEVICE_INFO_GET_SOURCE_NAME          = 1
	DISPLAYCONFIG_DEVICE_INFO_GET_TARGET_NAME          = 2
	DISPLAYCONFIG_DEVICE_INFO_GET_ADVANCED_COLOR_INFO  = 9
	DISPLAYCONFIG_DEVICE_INFO_SET_ADVANCED_COLOR_STATE = 10
)

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

type DISPLAYCONFIG_SOURCE_DEVICE_NAME struct {
	Header            DISPLAYCONFIG_HEADER
	ViewGdiDeviceName [32]uint16
}

type DISPLAYCONFIG_GET_ADVANCED_COLOR_INFO struct {
	Header              DISPLAYCONFIG_HEADER
	Value               uint32
	ColorEncoding       uint32
	BitsPerColorChannel uint32
}

type DISPLAYCONFIG_GET_ADVANCED_COLOR_STATE struct {
	Header DISPLAYCONFIG_HEADER
	Value  uint32
}

type DISPLAYCONFIG_SET_ADVANCED_COLOR_STATE struct {
	Header DISPLAYCONFIG_HEADER
	Value  uint32
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

type MappedPath struct {
	GdiNumber int
	Path      DISPLAYCONFIG_PATH_INFO
}

func GetMonitors() []MonitorInfo {
	var monitors []MonitorInfo

	paths, _, err := getActivePaths()
	if err != nil {
		fmt.Printf("[CCD Debug] Failed to query display config: %v\n", err)
		return fallbackMonitors()
	}

	// Map paths to GDI Display numbers (e.g. \\.\DISPLAY1 -> 1)
	var mappedPaths []MappedPath
	for _, path := range paths {
		var sourceName DISPLAYCONFIG_SOURCE_DEVICE_NAME
		sourceName.Header.Type = DISPLAYCONFIG_DEVICE_INFO_GET_SOURCE_NAME
		sourceName.Header.Size = uint32(unsafe.Sizeof(sourceName))
		sourceName.Header.AdapterId = path.SourceInfo.AdapterId
		sourceName.Header.Id = path.SourceInfo.Id

		gdiNum := 999
		if ret, _, _ := displayConfigGetDeviceInfo.Call(uintptr(unsafe.Pointer(&sourceName))); ret == 0 {
			gdiName := syscall.UTF16ToString(sourceName.ViewGdiDeviceName[:])
			numStr := strings.TrimPrefix(gdiName, `\\.\DISPLAY`)
			if val, err := strconv.Atoi(numStr); err == nil {
				gdiNum = val
			}
		}

		mappedPaths = append(mappedPaths, MappedPath{
			GdiNumber: gdiNum,
			Path:      path,
		})
	}

	// Guarantee Display 1 is index 0, Display 2 is index 1, etc.
	sort.Slice(mappedPaths, func(i, j int) bool {
		return mappedPaths[i].GdiNumber < mappedPaths[j].GdiNumber
	})

	for idx, item := range mappedPaths {
		path := item.Path

		var targetName DISPLAYCONFIG_TARGET_DEVICE_NAME
		targetName.Header.Type = DISPLAYCONFIG_DEVICE_INFO_GET_TARGET_NAME
		targetName.Header.Size = uint32(unsafe.Sizeof(targetName))
		targetName.Header.AdapterId = path.TargetInfo.AdapterId
		targetName.Header.Id = path.TargetInfo.Id

		monitorName := ""
		if retInfo, _, _ := displayConfigGetDeviceInfo.Call(uintptr(unsafe.Pointer(&targetName))); retInfo == 0 {
			monitorName = syscall.UTF16ToString(targetName.MonitorFriendlyDeviceName[:])
		}

		if monitorName == "" {
			monitorName = fmt.Sprintf("Display %d", item.GdiNumber)
		}

		deviceName := "Generic Display"
		gdiNameStr := fmt.Sprintf(`\\.\DISPLAY%d`, item.GdiNumber)
		for devIdx := uint32(0); ; devIdx++ {
			var dev DISPLAY_DEVICE
			dev.Cb = uint32(unsafe.Sizeof(dev))
			ret, _, _ := enumDisplayDevices.Call(0, uintptr(devIdx), uintptr(unsafe.Pointer(&dev)), 0)
			if ret == 0 {
				break
			}
			if syscall.UTF16ToString(dev.DeviceName[:]) == gdiNameStr {
				deviceName = syscall.UTF16ToString(dev.DeviceString[:])
				break
			}
		}

		isHDR := queryHDRState(path.TargetInfo.AdapterId, path.TargetInfo.Id)

		monitors = append(monitors, MonitorInfo{
			ID:          uint32(idx),
			MonitorName: monitorName,
			DeviceName:  deviceName,
			IsHDROn:     isHDR,
		})
	}

	return monitors
}

func queryHDRState(adapterID LUID, targetID uint32) bool {
	var colorInfo DISPLAYCONFIG_GET_ADVANCED_COLOR_INFO
	colorInfo.Header.Type = DISPLAYCONFIG_DEVICE_INFO_GET_ADVANCED_COLOR_INFO
	colorInfo.Header.Size = 32 // Header (16) + Value (4) + ColorEncoding (4) + BitsPerColorChannel (4) + Padding (4)
	colorInfo.Header.AdapterId = adapterID
	colorInfo.Header.Id = targetID

	ret, _, _ := displayConfigGetDeviceInfo.Call(uintptr(unsafe.Pointer(&colorInfo)))
	if ret != 0 {
		return false
	}

	// Win32 Bitfield Layout for Value:
	// Bit 0 (0x01): advancedColorSupported
	// Bit 1 (0x02): advancedColorEnabled
	// Bit 2 (0x04): wideColorEnforced
	// Bit 3 (0x08): advancedColorForceDisabled
	// Bit 4 (0x10): advancedColorActive (Active HDR scanout stream)
	supported := (colorInfo.Value & 0x01) != 0
	enabled := (colorInfo.Value & 0x02) != 0
	forceDisabled := (colorInfo.Value & 0x08) != 0
	active := (colorInfo.Value & 0x10) != 0

	fmt.Printf("[HDR Detect Debug] TargetID %d: Info Value=0x%X (Supp=%t, Ena=%t, Active=%t, Dis=%t)\n",
		targetID, colorInfo.Value, supported, enabled, active, forceDisabled)

	if forceDisabled {
		return false
	}

	// Trust the active pipeline state bit (0x10) if asserted by the driver,
	// otherwise fall back to user-enabled state (excluding pure WCG 0x5/0x7 profile flags)
	if active {
		return true
	}

	// If value is 0x5 or 0x7, Windows has wide color enhancements enabled without actual HDR stream active
	if colorInfo.Value == 0x5 || colorInfo.Value == 0x7 {
		return false
	}

	return supported && enabled
}

func ToggleHDR(displayID uint32, enable bool) {
	paths, _, err := getActivePaths()
	if err != nil {
		fmt.Printf("[HDR Toggle Debug] Failed to query active paths: %v\n", err)
		return
	}

	// Match sorted UI index back to correct GDI display path
	var mappedPaths []MappedPath
	for _, path := range paths {
		var sourceName DISPLAYCONFIG_SOURCE_DEVICE_NAME
		sourceName.Header.Type = DISPLAYCONFIG_DEVICE_INFO_GET_SOURCE_NAME
		sourceName.Header.Size = uint32(unsafe.Sizeof(sourceName))
		sourceName.Header.AdapterId = path.SourceInfo.AdapterId
		sourceName.Header.Id = path.SourceInfo.Id

		gdiNum := 999
		if ret, _, _ := displayConfigGetDeviceInfo.Call(uintptr(unsafe.Pointer(&sourceName))); ret == 0 {
			gdiName := syscall.UTF16ToString(sourceName.ViewGdiDeviceName[:])
			numStr := strings.TrimPrefix(gdiName, `\\.\DISPLAY`)
			if val, err := strconv.Atoi(numStr); err == nil {
				gdiNum = val
			}
		}

		mappedPaths = append(mappedPaths, MappedPath{
			GdiNumber: gdiNum,
			Path:      path,
		})
	}

	sort.Slice(mappedPaths, func(i, j int) bool {
		return mappedPaths[i].GdiNumber < mappedPaths[j].GdiNumber
	})

	if displayID >= uint32(len(mappedPaths)) {
		fmt.Printf("[HDR Toggle Debug] Invalid display index %d\n", displayID)
		return
	}

	targetPath := mappedPaths[displayID].Path

	var setColorState DISPLAYCONFIG_SET_ADVANCED_COLOR_STATE
	setColorState.Header.Type = DISPLAYCONFIG_DEVICE_INFO_SET_ADVANCED_COLOR_STATE
	setColorState.Header.Size = uint32(unsafe.Sizeof(setColorState))
	setColorState.Header.AdapterId = targetPath.TargetInfo.AdapterId
	setColorState.Header.Id = targetPath.TargetInfo.Id

	if enable {
		setColorState.Value = 1
	} else {
		setColorState.Value = 0
	}

	ret, _, _ := displayConfigSetDeviceInfo.Call(uintptr(unsafe.Pointer(&setColorState)))
	if ret != 0 {
		fmt.Printf("[HDR Toggle Debug] SetDeviceInfo failed for display %d with status: 0x%X\n", displayID, ret)
	}
}

func fallbackMonitors() []MonitorInfo {
	return []MonitorInfo{
		{ID: 0, MonitorName: "Display 1 (Fallback)", DeviceName: "Device 1 (Fallback)", IsHDROn: false},
	}
}

func getActivePaths() ([]DISPLAYCONFIG_PATH_INFO, []DISPLAYCONFIG_MODE_INFO, error) {
	for attempts := 0; attempts < 5; attempts++ {
		var numPathArrayItems, numModeInfoArrayItems uint32

		ret, _, _ := getDisplayConfigBufferSizes.Call(
			uintptr(QDC_ONLY_ACTIVE_PATHS),
			uintptr(unsafe.Pointer(&numPathArrayItems)),
			uintptr(unsafe.Pointer(&numModeInfoArrayItems)),
		)
		if ret != 0 || numPathArrayItems == 0 {
			return nil, nil, fmt.Errorf("GetDisplayConfigBufferSizes failed with code 0x%X", ret)
		}

		pathInfoArray := make([]DISPLAYCONFIG_PATH_INFO, numPathArrayItems)
		modeInfoArray := make([]DISPLAYCONFIG_MODE_INFO, numModeInfoArrayItems)

		pathCount := numPathArrayItems
		modeCount := numModeInfoArrayItems

		ret, _, _ = queryDisplayConfig.Call(
			uintptr(QDC_ONLY_ACTIVE_PATHS),
			uintptr(unsafe.Pointer(&pathCount)),
			uintptr(unsafe.Pointer(&pathInfoArray[0])),
			uintptr(unsafe.Pointer(&modeCount)),
			uintptr(unsafe.Pointer(&modeInfoArray[0])),
			0,
		)

		if ret == ERROR_INSUFFICIENT_BUFFER {
			continue
		}

		if ret != 0 {
			return nil, nil, fmt.Errorf("QueryDisplayConfig failed with code 0x%X", ret)
		}

		return pathInfoArray[:pathCount], modeInfoArray[:modeCount], nil
	}

	return nil, nil, fmt.Errorf("QueryDisplayConfig buffer size unstable after retries")
}
