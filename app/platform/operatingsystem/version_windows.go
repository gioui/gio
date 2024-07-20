//go:build windows

package operatingsystem

import (
	"strconv"

	"github.com/ddkwork/golibrary/mylog"
	"golang.org/x/sys/windows/registry"
)

type WindowsVersionInfo struct {
	Major          int
	Minor          int
	Build          int
	DisplayVersion string
}

func (w *WindowsVersionInfo) IsWindowsVersionAtLeast(major, minor, buildNumber int) bool {
	return w.Major >= major && w.Minor >= minor && w.Build >= buildNumber
}

func GetWindowsVersionInfo() (*WindowsVersionInfo, error) {
	key := mylog.Check2(registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows NT\CurrentVersion`, registry.QUERY_VALUE))
	return &WindowsVersionInfo{
		Major:          regDWORDKeyAsInt(key, "CurrentMajorVersionNumber"),
		Minor:          regDWORDKeyAsInt(key, "CurrentMinorVersionNumber"),
		Build:          regStringKeyAsInt(key, "CurrentBuildNumber"),
		DisplayVersion: regKeyAsString(key, "DisplayVersion"),
	}, nil
}

func regDWORDKeyAsInt(key registry.Key, name string) int {
	result, _ := mylog.Check3(key.GetIntegerValue(name))
	return int(result)
}

func regStringKeyAsInt(key registry.Key, name string) int {
	resultStr, _ := mylog.Check3(key.GetStringValue(name))
	result := mylog.Check2(strconv.Atoi(resultStr))
	return result
}

func regKeyAsString(key registry.Key, name string) string {
	resultStr, _ := mylog.Check3(key.GetStringValue(name))
	return resultStr
}
