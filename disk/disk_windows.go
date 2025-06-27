//go:build windows

package disk

import (
	"bytes"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32                    *windows.LazyDLL
	procGetLogicalDriveStringsW *windows.LazyProc
	procGetDriveType            *windows.LazyProc
	procGetVolumeInformation    *windows.LazyProc
)

func init() {
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")
	procGetLogicalDriveStringsW = kernel32.NewProc("GetLogicalDriveStringsW")
	procGetDriveType = kernel32.NewProc("GetDriveTypeW")
	procGetVolumeInformation = kernel32.NewProc("GetVolumeInformationW")
}

const (
	FileFileCompression = int64(16)     // 0x00000010
	FileReadOnlyVolume  = int64(524288) // 0x00080000
)

func partitions() ([]PartitionStat, error) {
	var ret []PartitionStat
	lpBuffer := make([]byte, 254)
	diskret, _, err := procGetLogicalDriveStringsW.Call(
		uintptr(len(lpBuffer)),
		uintptr(unsafe.Pointer(&lpBuffer[0])))
	if diskret == 0 {
		return ret, err
	}
	for _, v := range lpBuffer {
		if v >= 65 && v <= 90 {
			path := string(v) + ":"
			typepath, _ := windows.UTF16PtrFromString(path)
			typeret, _, _ := procGetDriveType.Call(uintptr(unsafe.Pointer(typepath)))
			if typeret == 0 {
				return ret, windows.GetLastError()
			}
			// 2: DRIVE_REMOVABLE 3: DRIVE_FIXED 4: DRIVE_REMOTE 5: DRIVE_CDROM

			if typeret == 2 || typeret == 3 || typeret == 4 || typeret == 5 {
				lpVolumeNameBuffer := make([]byte, 256)
				lpVolumeSerialNumber := int64(0)
				lpMaximumComponentLength := int64(0)
				lpFileSystemFlags := int64(0)
				lpFileSystemNameBuffer := make([]byte, 256)
				volpath, _ := windows.UTF16PtrFromString(string(v) + ":/")
				driveret, _, err := procGetVolumeInformation.Call(
					uintptr(unsafe.Pointer(volpath)),
					uintptr(unsafe.Pointer(&lpVolumeNameBuffer[0])),
					uintptr(len(lpVolumeNameBuffer)),
					uintptr(unsafe.Pointer(&lpVolumeSerialNumber)),
					uintptr(unsafe.Pointer(&lpMaximumComponentLength)),
					uintptr(unsafe.Pointer(&lpFileSystemFlags)),
					uintptr(unsafe.Pointer(&lpFileSystemNameBuffer[0])),
					uintptr(len(lpFileSystemNameBuffer)))
				if driveret == 0 {
					if typeret == 5 || typeret == 2 {
						continue //device is not ready will happen if there is no disk in the drive
					}
					return ret, err
				}
				opts := "rw"
				if lpFileSystemFlags&FileReadOnlyVolume != 0 {
					opts = "ro"
				}
				if lpFileSystemFlags&FileFileCompression != 0 {
					opts += ".compress"
				}

				d := PartitionStat{
					MountPoint: path,
					Device:     path,
					FsType:     string(bytes.Replace(lpFileSystemNameBuffer, []byte("\x00"), []byte(""), -1)),
					Opts:       opts,
				}
				ret = append(ret, d)
			}
		}
	}
	return ret, nil
}
