package main

import "sync"

type NodeConfig struct {
	ID               string  `json:"id"`
	Type             string  `json:"type"`
	City             string  `json:"city"`
	Name             string  `json:"name"`
	DeviceSN         string  `json:"device_sn,omitempty"`
	Host             string  `json:"host,omitempty"`
	Community        string  `json:"community,omitempty"`
	OID              string  `json:"oid,omitempty"`
	Model            string  `json:"model"`
	NotifyIDs        []int64 `json:"notify_ids"`
	AlertsEnabled    bool    `json:"alerts_enabled"`
	IsLiFePO4        bool    `json:"is_lifepo4"`
	CallEnabled      bool    `json:"call_enabled"`
	PoweredByClients bool    `json:"powered_by_clients"`
	ClientPhone      string  `json:"client_phone"`
	NoPowerSensor    bool    `json:"no_power_sensor"`
}

type NodeData struct {
	ID               string  `json:"id"`
	Type             string  `json:"type"`
	Name             string  `json:"name"`
	City             string  `json:"city"`
	Model            string  `json:"model"`
	DeviceSN         string  `json:"device_sn,omitempty"`
	BatterySOC       int     `json:"soc"`
	LoadPower        int     `json:"load_p"`
	TotalValue       float64 `json:"total_val"`
	GridOnline       bool    `json:"grid_on"`
	GenOnline        bool    `json:"gen_on"`
	Updated          string  `json:"updated"`
	GridOfflineSince int64   `json:"grid_offline_since"`
	IsLiFePO4        bool    `json:"is_lifepo4"`
	PoweredByClients bool    `json:"powered_by_clients"`
	ClientPhone      string  `json:"client_phone"`
	NoPowerSensor    bool    `json:"no_power_sensor"`
}

type Event struct {
	Date string `json:"date"`
	Time string `json:"time"`
	Msg  string `json:"msg"`
}

type GlobalState struct {
	Nodes   map[string]NodeData `json:"nodes"`
	History []Event             `json:"history"`
	sync.RWMutex
}
