package limacharlie

var Platforms = struct {
	Windows  uint32
	Linux    uint32
	MacOS    uint32
	IOS      uint32
	Android  uint32
	ChromeOS uint32
	Net      uint32
}{
	Windows:  0x10000000,
	Linux:    0x20000000,
	MacOS:    0x30000000,
	IOS:      0x40000000,
	Android:  0x50000000,
	ChromeOS: 0x60000000,
	Net:      0x70000000,
}

var Architectures = struct {
	X86       uint32
	X64       uint32
	ARM       uint32
	ARM64     uint32
	Alpine64  uint32
	Chrome    uint32
	WireGuard uint32
	ARML      uint32
}{
	X86:       0x00000001,
	X64:       0x00000002,
	ARM:       0x00000003,
	ARM64:     0x00000004,
	Alpine64:  0x00000005,
	Chrome:    0x00000006,
	WireGuard: 0x00000007,
	ARML:      0x00000008,
}

var PlatformsString = map[uint32]string{
	Platforms.Windows:  "windows",
	Platforms.Linux:    "linux",
	Platforms.MacOS:    "macos",
	Platforms.IOS:      "ios",
	Platforms.Android:  "android",
	Platforms.ChromeOS: "chromeos",
	Platforms.Net:      "net",
}

var ArchitecturesString = map[uint32]string{
	Architectures.X86:       "x86",
	Architectures.X64:       "x64",
	Architectures.ARM:       "arm",
	Architectures.ARM64:     "arm64",
	Architectures.Alpine64:  "alpine64",
	Architectures.Chrome:    "chrome",
	Architectures.WireGuard: "wireguard",
	Architectures.ARML:      "arml",
}
