package main

var AndroidPermissions = map[string][]string{
	"network": {
		"android.permission.INTERNET",
	},
	"bluetooth": {
		"android.permission.BLUETOOTH",
		"android.permission.BLUETOOTH_ADMIN",
		"android.permission.ACCESS_FINE_LOCATION",
	},
	"bluetooth_le": {
		"android.permission.BLUETOOTH",
		"android.permission.BLUETOOTH_ADMIN",
		"android.permission.ACCESS_FINE_LOCATION",
	},
}

var AndroidFeatures = map[string][]string{
	"default":      {`glEsVersion="0x00030000"`},
	"bluetooth":    {`name="android.hardware.bluetooth"`},
	"bluetooth_le": {`name="android.hardware.bluetooth_le"`},
}
