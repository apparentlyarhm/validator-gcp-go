package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

/*
	There are several directions I can take the router configuration, which is synonymous with controllers
	in spring boot. in my original app, i made a single controller so here making a single handler + router
	makes sense.

	following were the original whitelisted routes -> no auth required

*/

/*
/api/v2/auth/** -> generate url and return tokens:

	`GET /login`
	`GET /callback`

GET /api/v2/ping,
GET /api/v2/machine,
GET /api/v2/firewall,
PATCH /api/v2/firewall/add-ip,
GET /api/v2/firewall/check-ip,
GET /api/v2/server-info,

/api/v2/mods/** -> download and list all mods (connects to gcs):

	GET /mods
	GET /mods/download/{filename}

needs admin role:
PATCH /api/v2/firewall/purge
PATCH /api/v2/firewall/make-public

needs admin or user role:
POST /api/v2/execute
*/
func GlobalRouter(h *GlobalHandler) http.Handler {
	r := chi.NewRouter()

	// there isnt much going on globally
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Route("/api/v2", func(r chi.Router) {

		r.Get("/ping", h.Pong)
		r.Get("/machine", h.GetMachineDetails)
		r.Get("/server-info", h.GetServerInfo)

		r.Route("/mods", func(r chi.Router) {
			r.Get("", h.GetMods)
			r.Get("/download/{filename}", h.DownloadMod)
		})

		r.Route("/firewall", func(r chi.Router) {
			r.Get("", h.GetFirewallDetails)
			r.Get("/check-ip", h.CheckIpInFirewall)
			r.Patch("/add-ip", h.AddUserIp)

			r.Group(func(r chi.Router) {
				// TODO: auth middleware - admin
				r.Patch("/purge", h.PurgeFirewall)
				r.Patch("/make-public", h.MakePublic)
			})

		})

		r.Route("/auth", func(r chi.Router) {
			r.Get("/login", h.GetGitHubLoginUrl)
			r.Get("/callback", h.IssueJwtToken)
		})

	})

	return nil
}
