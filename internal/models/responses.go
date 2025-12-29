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
	AddressCount int16  `json:"allowedIpCount,omitempty"`
}

// used to communicate with github
type GithubTokenResponse struct {
	AccessToken string `json:"access_token"`
	Scope       string `json:"scope"`
	TokenType   string `json:"token_type"`
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
	PlayerNumber int16    `json:"numPlayers"`
	Players      []string `json:"players"`
	GameType     string   `json:"gameType"`
	MaxPlayers   int16    `json:"maxPlayers"`
	HostPort     int32    `json:"hostPort"`
	Version      string   `json:"version"`
	Map          string   `json:"map"`
	GameId       string   `json:"gameId"`
}

type CommonResponse struct {
	Message string `json:"message"`
}
