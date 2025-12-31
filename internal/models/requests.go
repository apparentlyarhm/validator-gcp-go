package models

type RconRequest struct {
	Command   string   `json:"command"`
	Arguments []string `json:"arguments"`
}

type AddressAddRequest struct {
	Address string `json:"address"`
}
