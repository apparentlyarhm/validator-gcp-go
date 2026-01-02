package models

type InstanceDetailResponse struct {
	InstanceName      string            `json:"instanceName,omitempty"`
	InstanceZone      string            `json:"instanceZone,omitempty"`
	MachineType       string            `json:"machineType,omitempty"`
	InstanceId        string            `json:"instanceId,omitempty"`
	Status            string            `json:"status,omitempty"`
	CreationTimestamp string            `json:"creationTimestamp,omitempty"`
	PublicIp          string            `json:"publicIp,omitempty"`
	CpuPlatform       string            `json:"cpuPlatform,omitempty"`
	CpuCores          int               `json:"cpuCores,omitempty"`
	MemoryMb          int               `json:"memoryMb,omitempty"`
	DiskGb            int32             `json:"diskGb,omitempty"`
	Metadata          map[string]string `json:"metadata,omitempty"`
}

type FirwallRuleResponse struct {
	Name         string `json:"name,omitempty"`
	Status       string `json:"status,omitempty"`
	Direction    string `json:"direction,omitempty"`
	AddressCount int    `json:"allowedIpCount,omitempty"`
}

// used to communicate with github
type GithubTokenResponse struct {
	AccessToken string `json:"access_token"`
	Scope       string `json:"scope"`
	TokenType   string `json:"token_type"`
}

// comes from github when we lookup user in the login flow
type GithubUserResponse struct {
	Login string `json:"login"`
	ID    int64  `json:"id"`
	Email string `json:"email"`
}

type ModListResponse struct {
	UpdatedAt string   `json:"updatedAt"`
	Mods      []string `json:"mods"`
}

type LoginResponse struct {
	Id    string `json:"id"`
	Email string `json:"email"`
	Token string `json:"token"`
}

type MOTDResponse struct {
	Hostname     string   `json:"hostname"`
	PlayerNumber int      `json:"numPlayers"`
	Players      []string `json:"players"`
	GameType     string   `json:"gameType"`
	MaxPlayers   int      `json:"maxPlayers"`
	HostPort     int      `json:"hostPort"`
	Version      string   `json:"version"`
	Map          string   `json:"map"`
	GameId       string   `json:"gameId"`
}

type CommonResponse struct {
	Message string `json:"message"`
}

/*
The struct below is very specific to how the server at the time of writing this was behaving.
Server was: Modded + NeoForge on 1.21.1

The log lines have been parsed to remove stack traces and truncate long messages.

some example log lines:
[02Jan2026 11:50:06.134] [Server thread/INFO] [net.minecraft.server.MinecraftServer/]: ligmahbulls has made the advancement [Cobweb Entanglement]

[02Jan2026 10:49:11.971] [Server thread/INFO] [net.minecraft.server.MinecraftServer/]: Karma0o7 fell from a high place

[02Jan2026 10:49:11.972] [Server thread/INFO] [gravestone/]: The death ID of player Karma0o7 is 10a665a3-f0ce-4273-8868-17f3c6f7e2e1

[02Jan2026 10:45:11.160] [Server thread/INFO] [pingwheel/]: Channel update: ligmahbulls -> default

we observe the presence of 4 groups, which can be captured via regex
*/
type LogItem struct {
	Timestamp string `json:"timestamp"` // weird date format
	Level     string `json:"level"`     // pretty much the severity
	Src       string `json:"src"`       // server thread?
	Message   string `json:"message"`   // The actual content (truncated)
}

type LogResponse struct {
	Items []LogItem `json:"items"`
}
