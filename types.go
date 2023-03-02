package main

import (
	"net"
)

type Job struct {
	domain string
	ip     net.IP
}

type Prefix struct {
	IPPrefix           string `json:"ip_prefix"`
	Region             string `json:"region"`
	Service            string `json:"service"`
	NetworkBorderGroup string `json:"network_border_group"`
}

type Response struct {
	SyncToken  string   `json:"syncToken"`
	CreateDate string   `json:"createDate"`
	Prefixes   []Prefix `json:"prefixes"`
}
