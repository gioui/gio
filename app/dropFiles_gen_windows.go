package app

import (
	"fmt"
	"syscall"
	"unsafe"

	"github.com/ddkwork/golibrary/mylog"

	gowindows "golang.org/x/sys/windows"
)

type DropFilesEventData struct {
	X, Y  int
	Files []string
}

func (d DropFilesEventData) ImplementsFilter() {
}

func (d DropFilesEventData) ImplementsEvent() {
}

const (
	WM_DROPFILES      = 0x233 // 563
	WS_EX_ACCEPTFILES = 0x00000010
)

func genDropFilesEventArg(hDrop uintptr) DropFilesEventData {
	var data DropFilesEventData
	_, fileCount := DragQueryFile(hDrop, 0xFFFFFFFF)
	data.Files = make([]string, fileCount)

	var i uint
	for i = 0; i < fileCount; i++ {
		data.Files[i], _ = DragQueryFile(hDrop, i)
	}

	data.X, data.Y, _ = DragQueryPoint(hDrop)
	DragFinish(hDrop)
	return data
}

var (
	modshell32 = syscall.NewLazyDLL("shell32.dll")

	procSHBrowseForFolder    = modshell32.NewProc("SHBrowseForFolderW")
	procSHGetPathFromIDList  = modshell32.NewProc("SHGetPathFromIDListW")
	procDragAcceptFiles      = modshell32.NewProc("DragAcceptFiles")
	procDragQueryFile        = modshell32.NewProc("DragQueryFileW")
	procDragQueryPoint       = modshell32.NewProc("DragQueryPoint")
	procDragFinish           = modshell32.NewProc("DragFinish")
	procShellExecute         = modshell32.NewProc("ShellExecuteW")
	procExtractIcon          = modshell32.NewProc("ExtractIconW")
	procGetSpecialFolderPath = modshell32.NewProc("SHGetSpecialFolderPathW")
)

func DragQueryFile(hDrop HDROP, iFile uint) (fileName string, fileCount uint) {
	ret, _ := mylog.Check3(procDragQueryFile.Call(uintptr(hDrop), uintptr(iFile), 0, 0))
	fileCount = uint(ret)
	if iFile != 0xFFFFFFFF {
		buf := make([]uint16, fileCount+1)
		ret, _ := mylog.Check3(procDragQueryFile.Call(
			uintptr(hDrop),
			uintptr(iFile),
			uintptr(unsafe.Pointer(&buf[0])),
			uintptr(fileCount+1)))

		if ret == 0 {
			panic("Invoke DragQueryFile error.")
		}
		fileName = syscall.UTF16ToString(buf)
	}
	return
}

func DragQueryPoint(hDrop HDROP) (x, y int, isClientArea bool) {
	var pt POINT
	ret, _ := mylog.Check3(procDragQueryPoint.Call(
		uintptr(hDrop),
		uintptr(unsafe.Pointer(&pt))))
	return int(pt.X), int(pt.Y), ret == 1
}

func DragFinish(hDrop HDROP) {
	procDragFinish.Call(hDrop)
}

type (
	HANDLE = uintptr
	HDROP  = HANDLE
)

// http://msdn.microsoft.com/en-us/library/windows/desktop/dd162805.aspx
type POINT struct {
	X, Y int32
}

// http://msdn.microsoft.com/en-us/library/windows/desktop/dd162897.aspx
type RECT struct {
	Left, Top, Right, Bottom int32
}

func (r *RECT) String() string {
	return fmt.Sprintf("RECT (%p): Left: %d, Top: %d, Right: %d, Bottom: %d", r, r.Left, r.Top, r.Right, r.Bottom)
}

func DragAcceptFiles(hwnd gowindows.Handle, accept bool) {
	procDragAcceptFiles.Call(uintptr(hwnd), uintptr(BoolToBOOL(accept)))
}

func BoolToBOOL(value bool) BOOL {
	if value {
		return 1
	}
	return 0
}

type BOOL = int32
