//go:build windows

package operatingsystem

import (
	"fmt"

	"github.com/ddkwork/golibrary/mylog"

	"golang.org/x/sys/windows/registry"
)

func platformInfo() (*OS, error) {
	// Default value
	var result OS
	result.ID = "Unknown"
	result.Name = "Windows"
	result.Version = "Unknown"

	// Credit: https://stackoverflow.com/a/33288328
	// Ignore errors as it isn't a showstopper
	key := mylog.Check2(registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows NT\CurrentVersion`, registry.QUERY_VALUE))

	productName, _ := mylog.Check3(key.GetStringValue("ProductName"))
	currentBuild, _ := mylog.Check3(key.GetStringValue("CurrentBuildNumber"))
	displayVersion, _ := mylog.Check3(key.GetStringValue("DisplayVersion"))
	releaseId, _ := mylog.Check3(key.GetStringValue("ReleaseId"))

	result.Name = productName
	result.Version = fmt.Sprintf("%s (Build: %s)", releaseId, currentBuild)
	result.ID = displayVersion

	return &result, key.Close()
}
