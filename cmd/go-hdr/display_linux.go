//go:build !windows

package main

import "fmt"

type MonitorInfo struct {
	ID          uint32
	MonitorName string
	DeviceName  string
	IsHDROn     bool
}

func GetMonitors() []MonitorInfo {
	return []MonitorInfo{
		{ID: 0, MonitorName: "Linux Virtual Display (Mock)", DeviceName: "NA", IsHDROn: false},
	}
}

func CheckHDRStatus(displayID uint32) bool {
	return false
}

func ToggleHDR(displayID uint32, enable bool) {
	fmt.Printf("[Linux Mock] Toggle HDR for display %d -> %t\n", displayID, enable)
}
