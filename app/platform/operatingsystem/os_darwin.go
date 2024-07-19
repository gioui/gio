package operatingsystem

import "github.com/ddkwork/golibrary/mylog"

func getSysctlValue(key string) (string, error) {
	return "", nil
	//stdout, _, err := shell.RunCommand(".", "sysctl", key)
	//if err != nil {
	//	return "", err
	//}
	//version := strings.TrimPrefix(stdout, key+": ")
	//return strings.TrimSpace(version), nil
}

func platformInfo() (*OS, error) {
	// Default value
	var result OS
	result.ID = "Unknown"
	result.Name = "MacOS"
	result.Version = "Unknown"

	version := mylog.Check2(getSysctlValue("kern.osproductversion"))

	result.Version = version
	ID := mylog.Check2(getSysctlValue("kern.osversion"))

	result.ID = ID

	// 		cmd := CreateCommand(directory, command, args...)
	// 		var stdo, stde bytes.Buffer
	// 		cmd.Stdout = &stdo
	// 		cmd.Stderr = &stde
	// 		err := stream.RunCommand()
	// 		return stdo.String(), stde.String(), err
	// 	}
	// 	sysctl := shell.NewCommand("sysctl")
	// 	kern.ostype: Darwin
	// kern.osrelease: 20.1.0
	// kern.osrevision: 199506

	return &result, nil
}
