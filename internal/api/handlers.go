package api

import (
	"net/http"

	serv "github.com/validator-gcp/v2/internal/service"
)

// There are several directions I can take the router configuration, which is synonymous with
// controllers in spring boot. in my original app, i made a single controller so here making a single
// handler makes sense.

type Handler struct {
	validator *serv.ValidatorService
}

func (h *Handler) Pong(w http.ResponseWriter, r *http.Request) {
	// TODO: Return CommonResponse via service.DoPong()

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("pong"))
}

// ---------------- AUTH ----------------

// GET /auth/github/url
func (h *Handler) GetGitHubLoginUrl(w http.ResponseWriter, r *http.Request) {
	// TODO: Return CommonResponse with URL
}

// POST /auth/github/login
func (h *Handler) IssueJwtToken(w http.ResponseWriter, r *http.Request) {
	// TODO: Decode JSON body -> Call service.IssueJwtToken(code) -> Return LoginResponse
}

// ---------------- GCP / FIREWALL ----------------

// GET /machine
func (h *Handler) GetMachineDetails(w http.ResponseWriter, r *http.Request) {
	// TODO: Call service.GetMachineDetails() -> Return InstanceDetailResponse
}

// GET /firewall
func (h *Handler) GetFirewallDetails(w http.ResponseWriter, r *http.Request) {
	// TODO: Call service.GetFirewallDetails() -> Return FirewallRuleResponse
}

// PATCH /firewall/add-ip
// Body: AddressAddRequest
func (h *Handler) AddUserIp(w http.ResponseWriter, r *http.Request) {
	// TODO: Decode JSON Body -> Call service.AddIpToFirewall(req)
}

// GET /firewall/check-ip?ip=1.2.3.4
func (h *Handler) CheckIpInFirewall(w http.ResponseWriter, r *http.Request) {
	// ip := r.URL.Query().Get("ip")
	// TODO: Call service.IsIpPresent(ip)
}

// PATCH /firewall/purge (ADMIN)
func (h *Handler) PurgeFirewall(w http.ResponseWriter, r *http.Request) {
	// TODO: Call service.PurgeFirewall()
}

// PATCH /firewall/make-public (ADMIN)
func (h *Handler) MakePublic(w http.ResponseWriter, r *http.Request) {
	// TODO: Call service.AllowPublicAccess()
}

// ---------------- MINECRAFT / UTILS ----------------

// GET /server-info?address=...
func (h *Handler) GetServerInfo(w http.ResponseWriter, r *http.Request) {
	// address := r.URL.Query().Get("address")
	// TODO: Call service.GetServerInfo(address) -> Return MOTDResponse
}

// GET /mods
func (h *Handler) GetMods(w http.ResponseWriter, r *http.Request) {
	// TODO: Call service.GetModList() -> Return ModListResponse
}

// GET /mods/download/{fileName}
func (h *Handler) DownloadMod(w http.ResponseWriter, r *http.Request) {
	// fileName := chi.URLParam(r, "fileName")
	// TODO: Call service.Download(fileName)
}

// POST /execute?address=...
// Body: RconRequest
func (h *Handler) ExecuteRcon(w http.ResponseWriter, r *http.Request) {
	// address := r.URL.Query().Get("address")
	// TODO: Decode JSON Body -> Call service.ExecuteRcon(address, req)
}
