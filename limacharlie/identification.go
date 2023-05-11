package limacharlie

var Platforms = struct {
	Windows  uint32
	Linux    uint32
	MacOS    uint32
	IOS      uint32
	Android  uint32
	ChromeOS uint32
	VPN      uint32

	// USP Formats
	Text         uint32
	JSON         uint32
	GCP          uint32
	AWS          uint32
	CarbonBlack  uint32
	OnePassword  uint32
	Office365    uint32
	Sophos       uint32
	Crowdstrike  uint32
	Xml          uint32
	Wel          uint32
	MsDefender   uint32
	Duo          uint32
	Okta         uint32
	SentinelOne  uint32
	GitHub       uint32
	Slack        uint32
	CEF          uint32
	LCEvent      uint32
	AzureAD      uint32
	AzureMonitor uint32
	CanaryToken  uint32
	GuardDuty    uint32
}{
	Windows:  0x10000000,
	Linux:    0x20000000,
	MacOS:    0x30000000,
	IOS:      0x40000000,
	Android:  0x50000000,
	ChromeOS: 0x60000000,
	VPN:      0x70000000,

	// USP Formats
	Text:         0x80000000,
	JSON:         0x90000000,
	GCP:          0xA0000000,
	AWS:          0xB0000000,
	CarbonBlack:  0xC0000000,
	OnePassword:  0xD0000000,
	Office365:    0xE0000000,
	Sophos:       0xF0000000,
	Crowdstrike:  0x01000000,
	Xml:          0x02000000,
	Wel:          0x03000000,
	MsDefender:   0x04000000,
	Duo:          0x05000000,
	Okta:         0x06000000,
	SentinelOne:  0x07000000,
	GitHub:       0x08000000,
	Slack:        0x09000000,
	CEF:          0x0A000000,
	LCEvent:      0x0B000000,
	AzureAD:      0x0C000000,
	AzureMonitor: 0x0D000000,
	CanaryToken:  0x0E000000,
	GuardDuty:    0x0F000000,
}

var Architectures = struct {
	X86        uint32
	X64        uint32
	ARM        uint32
	ARM64      uint32
	Alpine64   uint32
	Chrome     uint32
	WireGuard  uint32
	ARML       uint32
	USPAdapter uint32
}{
	X86:       0x00000001,
	X64:       0x00000002,
	ARM:       0x00000003,
	ARM64:     0x00000004,
	Alpine64:  0x00000005,
	Chrome:    0x00000006,
	WireGuard: 0x00000007,
	ARML:      0x00000008,

	// USP Formats
	USPAdapter: 0x00000009,
}

var PlatformStrings = map[uint32]string{
	Platforms.Windows:  "windows",
	Platforms.Linux:    "linux",
	Platforms.MacOS:    "macos",
	Platforms.IOS:      "ios",
	Platforms.Android:  "android",
	Platforms.ChromeOS: "chrome",
	Platforms.VPN:      "vpn",

	// USP Formats
	Platforms.Text:         "text",
	Platforms.JSON:         "json",
	Platforms.GCP:          "gcp",
	Platforms.AWS:          "aws",
	Platforms.CarbonBlack:  "carbon_black",
	Platforms.OnePassword:  "1password",
	Platforms.Office365:    "office365",
	Platforms.Sophos:       "sophos",
	Platforms.Crowdstrike:  "crowdstrike",
	Platforms.Xml:          "xml",
	Platforms.Wel:          "wel",
	Platforms.MsDefender:   "msdefender",
	Platforms.Duo:          "duo",
	Platforms.Okta:         "okta",
	Platforms.SentinelOne:  "sentinel_one",
	Platforms.GitHub:       "github",
	Platforms.Slack:        "slack",
	Platforms.CEF:          "cef",
	Platforms.LCEvent:      "lc_event",
	Platforms.AzureAD:      "azure_ad",
	Platforms.AzureMonitor: "azure_monitor",
	Platforms.CanaryToken:  "canary_token",
	Platforms.GuardDuty:    "guard_duty",
}

var ArchitectureStrings = map[uint32]string{
	Architectures.X86:       "x86",
	Architectures.X64:       "x64",
	Architectures.ARM:       "arm",
	Architectures.ARM64:     "arm64",
	Architectures.Alpine64:  "alpine64",
	Architectures.Chrome:    "chromium",
	Architectures.WireGuard: "wireguard",
	Architectures.ARML:      "arml",

	// USP Formats
	Architectures.USPAdapter: "usp_adapter",
}

var StringToPlatform = map[string]uint32{
	"windows": Platforms.Windows,
	"linux":   Platforms.Linux,
	"macos":   Platforms.MacOS,
	"ios":     Platforms.IOS,
	"android": Platforms.Android,
	"chrome":  Platforms.ChromeOS,
	"vpn":     Platforms.VPN,

	// USP Formats
	"text":          Platforms.Text,
	"json":          Platforms.JSON,
	"gcp":           Platforms.GCP,
	"aws":           Platforms.AWS,
	"carbon_black":  Platforms.CarbonBlack,
	"1password":     Platforms.OnePassword,
	"office365":     Platforms.Office365,
	"sophos":        Platforms.Sophos,
	"crowdstrike":   Platforms.Crowdstrike,
	"xml":           Platforms.Xml,
	"wel":           Platforms.Wel,
	"msdefender":    Platforms.MsDefender,
	"duo":           Platforms.Duo,
	"okta":          Platforms.Okta,
	"sentinel_one":  Platforms.SentinelOne,
	"github":        Platforms.GitHub,
	"slack":         Platforms.Slack,
	"cef":           Platforms.CEF,
	"lc_event":      Platforms.LCEvent,
	"azure_ad":      Platforms.AzureAD,
	"azure_monitor": Platforms.AzureMonitor,
	"canary_token":  Platforms.CanaryToken,
	"guard_duty":    Platforms.GuardDuty,
}

var StringToArchitecture = map[string]uint32{
	"x86":       Architectures.X86,
	"x64":       Architectures.X64,
	"arm":       Architectures.ARM,
	"arm64":     Architectures.ARM64,
	"alpine64":  Architectures.Alpine64,
	"chromium":  Architectures.Chrome,
	"wireguard": Architectures.WireGuard,
	"arml":      Architectures.ARML,

	// USP Formats
	"usp_adapter": Architectures.USPAdapter,
}
