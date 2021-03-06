package main

var AndroidPermissions = map[string][]string{
	"network": {
		"android.permission.INTERNET",
	},
	"networkstate": {
		"android.permission.ACCESS_NETWORK_STATE",
	},
	"bluetooth": {
		"android.permission.BLUETOOTH",
		"android.permission.BLUETOOTH_ADMIN",
		"android.permission.ACCESS_FINE_LOCATION",
	},
	"camera": {
		"android.permission.CAMERA",
	},
	"storage": {
		"android.permission.READ_EXTERNAL_STORAGE",
		"android.permission.WRITE_EXTERNAL_STORAGE",
	},
}

var AndroidFeatures = map[string][]string{
	"default": {`glEsVersion="0x00020000"`, `name="android.hardware.type.pc"`},
	"bluetooth": {
		`name="android.hardware.bluetooth"`,
		`name="android.hardware.bluetooth_le"`,
	},
	"camera": {
		`name="android.hardware.camera"`,
	},
}
