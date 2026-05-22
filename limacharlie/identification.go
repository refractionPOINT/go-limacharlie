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
	Text                     uint32
	JSON                     uint32
	GCP                      uint32
	AWS                      uint32
	CarbonBlack              uint32
	OnePassword              uint32
	Office365                uint32
	Sophos                   uint32
	HubSpot                  uint32
	Mimecast                 uint32
	FalconCloud              uint32
	Zendesk                  uint32
	PandaDoc                 uint32
	MacUnifiedLogging        uint32
	EntraID                  uint32
	Crowdstrike              uint32
	Xml                      uint32
	Wel                      uint32
	MsDefender               uint32
	Duo                      uint32
	Okta                     uint32
	SentinelOne              uint32
	GitHub                   uint32
	Slack                    uint32
	CEF                      uint32
	LCEvent                  uint32
	AzureAD                  uint32
	AzureMonitor             uint32
	CanaryToken              uint32
	GuardDuty                uint32
	ITGlue                   uint32
	K8sPods                  uint32
	Zeek                     uint32
	AzureEventHubNamespace   uint32
	AzureKeyVault            uint32
	AzureKubernetesService   uint32
	AzureNetwokSecurityGroup uint32
	AzureSqlAudit            uint32
	Email                    uint32
	Fortigate                uint32
	TrendWorryFree           uint32
	Netscaler                uint32
	PaloAltoFW               uint32
	IISLogs                  uint32
	Sublime                  uint32
	Box                      uint32
	Cylance                  uint32
	Proofpoint               uint32
	Wiz                      uint32
	Bitwarden                uint32
	TrendMicro               uint32
	Otel                     uint32
	CortexXDR                uint32
	Harmony                  uint32
	ThreatLocker             uint32
	HaloPSA                  uint32
}{
	Windows:  0x10000000,
	Linux:    0x20000000,
	MacOS:    0x30000000,
	IOS:      0x40000000,
	Android:  0x50000000,
	ChromeOS: 0x60000000,
	VPN:      0x70000000,

	// USP Formats
	Text:                     0x80000000,
	JSON:                     0x90000000,
	GCP:                      0xA0000000,
	AWS:                      0xB0000000,
	CarbonBlack:              0xC0000000,
	OnePassword:              0xD0000000,
	Office365:                0xE0000000,
	Sophos:                   0xF0000000,
	Crowdstrike:              0x01000000,
	Xml:                      0x02000000,
	Wel:                      0x03000000,
	MsDefender:               0x04000000,
	Duo:                      0x05000000,
	Okta:                     0x06000000,
	SentinelOne:              0x07000000,
	GitHub:                   0x08000000,
	Slack:                    0x09000000,
	CEF:                      0x0A000000,
	LCEvent:                  0x0B000000,
	AzureAD:                  0x0C000000,
	AzureMonitor:             0x0D000000,
	CanaryToken:              0x0E000000,
	GuardDuty:                0x0F000000,
	ITGlue:                   0x11000000,
	K8sPods:                  0x12000000,
	Zeek:                     0x13000000,
	MacUnifiedLogging:        0x14000000,
	AzureEventHubNamespace:   0x15000000,
	AzureKeyVault:            0x16000000,
	AzureKubernetesService:   0x17000000,
	AzureNetwokSecurityGroup: 0x18000000,
	AzureSqlAudit:            0x19000000,
	Email:                    0x1A000000,
	Fortigate:                0x1B000000,
	TrendWorryFree:           0x1C000000,
	Netscaler:                0x1D000000,
	PaloAltoFW:               0x1E000000,
	IISLogs:                  0x1F000000,
	HubSpot:                  0x21000000,
	Zendesk:                  0x22000000,
	PandaDoc:                 0x23000000,
	FalconCloud:              0x24000000,
	Mimecast:                 0x25000000,
	Sublime:                  0x26000000,
	Box:                      0x27000000,
	Cylance:                  0x28000000,
	Proofpoint:               0x29000000,
	EntraID:                  0x2A000000,
	Wiz:                      0x2B000000,
	Bitwarden:                0x2C000000,
	TrendMicro:               0x2D000000,
	Otel:                     0x2E000000,
	CortexXDR:                0x2F000000,
	Harmony:                  0x31000000,
	ThreatLocker:             0x32000000,
	HaloPSA:                  0x33000000,
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
	Platforms.Text:                     "text",
	Platforms.JSON:                     "json",
	Platforms.GCP:                      "gcp",
	Platforms.AWS:                      "aws",
	Platforms.CarbonBlack:              "carbon_black",
	Platforms.OnePassword:              "1password",
	Platforms.Office365:                "office365",
	Platforms.Sophos:                   "sophos",
	Platforms.HubSpot:                  "hubspot",
	Platforms.Mimecast:                 "mimecast",
	Platforms.FalconCloud:              "falconcloud",
	Platforms.Zendesk:                  "zendesk",
	Platforms.PandaDoc:                 "pandadoc",
	Platforms.MacUnifiedLogging:        "mac_unified_logging",
	Platforms.Crowdstrike:              "crowdstrike",
	Platforms.Xml:                      "xml",
	Platforms.Wel:                      "wel",
	Platforms.MsDefender:               "msdefender",
	Platforms.Duo:                      "duo",
	Platforms.Okta:                     "okta",
	Platforms.SentinelOne:              "sentinel_one",
	Platforms.GitHub:                   "github",
	Platforms.Slack:                    "slack",
	Platforms.CEF:                      "cef",
	Platforms.LCEvent:                  "lc_event",
	Platforms.AzureAD:                  "azure_ad",
	Platforms.AzureMonitor:             "azure_monitor",
	Platforms.CanaryToken:              "canary_token",
	Platforms.GuardDuty:                "guard_duty",
	Platforms.ITGlue:                   "itglue",
	Platforms.K8sPods:                  "k8s_pods",
	Platforms.Zeek:                     "zeek",
	Platforms.AzureEventHubNamespace:   "azure_event_hub_namespace",
	Platforms.AzureKeyVault:            "azure_key_vault",
	Platforms.AzureKubernetesService:   "azure_kubernetes_service",
	Platforms.AzureNetwokSecurityGroup: "azure_network_security_group",
	Platforms.AzureSqlAudit:            "azure_sql_audit",
	Platforms.Email:                    "email",
	Platforms.Fortigate:                "fortigate",
	Platforms.TrendWorryFree:           "trend_worryfree",
	Platforms.Netscaler:                "netscaler",
	Platforms.PaloAltoFW:               "paloalto_fw",
	Platforms.IISLogs:                  "iis",
	Platforms.Sublime:                  "sublime",
	Platforms.Box:                      "box",
	Platforms.Cylance:                  "cylance",
	Platforms.Proofpoint:               "proofpoint",
	Platforms.EntraID:                  "entraid",
	Platforms.Wiz:                      "wiz",
	Platforms.Bitwarden:                "bitwarden",
	Platforms.TrendMicro:               "trend_micro",
	Platforms.Otel:                     "otel",
	Platforms.CortexXDR:                "cortex_xdr",
	Platforms.Harmony:                  "harmony",
	Platforms.ThreatLocker:             "threatlocker",
	Platforms.HaloPSA:                  "halopsa",
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
	"text":                         Platforms.Text,
	"json":                         Platforms.JSON,
	"gcp":                          Platforms.GCP,
	"aws":                          Platforms.AWS,
	"carbon_black":                 Platforms.CarbonBlack,
	"1password":                    Platforms.OnePassword,
	"office365":                    Platforms.Office365,
	"sophos":                       Platforms.Sophos,
	"hubspot":                      Platforms.HubSpot,
	"mimecast":                     Platforms.Mimecast,
	"falconcloud":                  Platforms.FalconCloud,
	"zendesk":                      Platforms.Zendesk,
	"pandadoc":                     Platforms.PandaDoc,
	"mac_unified_logging":          Platforms.MacUnifiedLogging,
	"crowdstrike":                  Platforms.Crowdstrike,
	"xml":                          Platforms.Xml,
	"wel":                          Platforms.Wel,
	"msdefender":                   Platforms.MsDefender,
	"duo":                          Platforms.Duo,
	"okta":                         Platforms.Okta,
	"sentinel_one":                 Platforms.SentinelOne,
	"github":                       Platforms.GitHub,
	"slack":                        Platforms.Slack,
	"cef":                          Platforms.CEF,
	"lc_event":                     Platforms.LCEvent,
	"azure_ad":                     Platforms.AzureAD,
	"azure_monitor":                Platforms.AzureMonitor,
	"canary_token":                 Platforms.CanaryToken,
	"guard_duty":                   Platforms.GuardDuty,
	"itglue":                       Platforms.ITGlue,
	"k8s_pods":                     Platforms.K8sPods,
	"zeek":                         Platforms.Zeek,
	"azure_event_hub_namespace":    Platforms.AzureEventHubNamespace,
	"azure_key_vault":              Platforms.AzureKeyVault,
	"azure_kubernetes_service":     Platforms.AzureKubernetesService,
	"azure_network_security_group": Platforms.AzureNetwokSecurityGroup,
	"azure_sql_audit":              Platforms.AzureSqlAudit,
	"email":                        Platforms.Email,
	"fortigate":                    Platforms.Fortigate,
	"trend_worryfree":              Platforms.TrendWorryFree,
	"netscaler":                    Platforms.Netscaler,
	"paloalto_fw":                  Platforms.PaloAltoFW,
	"iis":                          Platforms.IISLogs,
	"sublime":                      Platforms.Sublime,
	"box":                          Platforms.Box,
	"cylance":                      Platforms.Cylance,
	"proofpoint":                   Platforms.Proofpoint,
	"entraid":                      Platforms.EntraID,
	"wiz":                          Platforms.Wiz,
	"bitwarden":                    Platforms.Bitwarden,
	"trend_micro":                  Platforms.TrendMicro,
	"otel":                         Platforms.Otel,
	"cortex_xdr":                   Platforms.CortexXDR,
	"harmony":                      Platforms.Harmony,
	"threatlocker":                 Platforms.ThreatLocker,
	"halopsa":                      Platforms.HaloPSA,
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
