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
	"storage": {
		"android.permission.READ_EXTERNAL_STORAGE",
		"android.permission.WRITE_EXTERNAL_STORAGE",
	},
}

var AndroidFeatures = map[string][]string{
	"default": {`glEsVersion="0x00030000"`},
	"bluetooth": {
		`name="android.hardware.bluetooth"`,
		`name="android.hardware.bluetooth_le"`,
	},
}
