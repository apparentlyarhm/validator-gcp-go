package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"path"

	"github.com/validator-gcp/v2/internal/apperror"
	serv "github.com/validator-gcp/v2/internal/service"
)

/*
the global handler, needs the main validator service and auth service.
*/
type GlobalHandler struct {
	Validator *serv.ValidatorService
}

func (h *GlobalHandler) Pong(w http.ResponseWriter, r *http.Request) {
	// TODO: Return CommonResponse via service.DoPong()

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("pong"))
}

// ---------------- AUTH ----------------

func (h *GlobalHandler) GetGitHubLoginUrl(w http.ResponseWriter, r *http.Request) {
	// TODO: Return CommonResponse with URL
}

/*
if a user logged in successfuly, we give them a token and a role. could be ANON, USER, ADMIN
*/
func (h *GlobalHandler) IssueJwtToken(w http.ResponseWriter, r *http.Request) {
	// TODO: Decode JSON body -> Call service.IssueJwtToken(code) -> Return LoginResponse
}

// ---------------- GCP / FIREWALL ----------------

func (h *GlobalHandler) GetMachineDetails(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	machine, err := h.Validator.GetMachineDetails(ctx)
	if err != nil {
		h.handleError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(machine); err != nil {
		h.handleError(w, r, err)
		return
	}
}
func (h *GlobalHandler) GetFirewallDetails(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	fw, err := h.Validator.GetFirewallDetails(ctx)
	if err != nil {
		h.handleError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(fw); err != nil {
		h.handleError(w, r, err)
		return
	}
}

func (h *GlobalHandler) AddUserIp(w http.ResponseWriter, r *http.Request) {
	// TODO: Decode JSON Body -> Call service.AddIpToFirewall(req)
}

func (h *GlobalHandler) CheckIpInFirewall(w http.ResponseWriter, r *http.Request) {
	ip := r.URL.Query().Get("ip")
	if ip == "" {
		h.handleError(w, r, apperror.ErrBadRequest)
	}

	ctx := r.Context()
	res, err := h.Validator.IsIpPresent(ctx, ip)
	if err != nil {
		h.handleError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(res); err != nil {
		h.handleError(w, r, err)
		return
	}
}

func (h *GlobalHandler) PurgeFirewall(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	res, err := h.Validator.PurgeFirewall(ctx)
	if err != nil {
		h.handleError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(res); err != nil {
		h.handleError(w, r, err)
		return
	}
}

func (h *GlobalHandler) MakePublic(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	res, err := h.Validator.AllowPublicAccess(ctx)
	if err != nil {
		h.handleError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(res); err != nil {
		h.handleError(w, r, err)
		return
	}
}

func (h *GlobalHandler) GetMods(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	mods, err := h.Validator.GetModList(ctx)
	if err != nil {
		h.handleError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(mods); err != nil {
		h.handleError(w, r, err)
		return
	}
}

func (h *GlobalHandler) DownloadMod(w http.ResponseWriter, r *http.Request) {
	fileName := path.Base(r.URL.Path)
	ctx := r.Context()

	res, err := h.Validator.Download(ctx, fileName)
	if err != nil {
		h.handleError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(res); err != nil {
		h.handleError(w, r, err)
		return
	}
}

// ---------------- MINECRAFT / UTILS ----------------

func (h *GlobalHandler) GetServerInfo(w http.ResponseWriter, r *http.Request) {
	address := r.URL.Query().Get("address")
	ctx := r.Context()

	res, err := h.Validator.GetServerInfo(ctx, address)
	if err != nil {
		h.handleError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(res); err != nil {
		h.handleError(w, r, err)
		return
	}
}

func (h *GlobalHandler) ExecuteRcon(w http.ResponseWriter, r *http.Request) {
	// address := r.URL.Query().Get("address")
	// TODO: Decode JSON Body -> Call service.ExecuteRcon(address, req)
}

// private helper that sends an error response.
func (h *GlobalHandler) handleError(w http.ResponseWriter, r *http.Request, err error) {
	var message string
	var status int

	switch {
	case errors.Is(err, apperror.ErrNotFound):
		status = http.StatusNotFound
		message = err.Error()

	case errors.Is(err, apperror.ErrConflict):
		status = http.StatusConflict
		message = err.Error()

	case errors.Is(err, apperror.ErrForbidden):
		status = http.StatusForbidden
		message = err.Error()

	case errors.Is(err, apperror.ErrBadRequest):
		status = http.StatusBadRequest
		message = err.Error()

	case errors.Is(err, apperror.ErrInternal):
		status = http.StatusInternalServerError
		message = apperror.INTERNAL_MESSAGE

	default:
		/*
			this section is for mostly 500 errors. i havent written the service layer so i'm not sure what all
			errors the GCP libraries can throw. In java it was a lot of times, IOExceptions
		*/
		message = apperror.INTERNAL_MESSAGE

		log.Printf("%v : %v", r.URL.Path, err)
		status = http.StatusInternalServerError

	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	json.NewEncoder(w).Encode(apperror.ErrorResponse{
		Message: message,
		Code:    int16(status), // bad practice but have to maintain response structures
	})
}
