package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gosnmp/gosnmp"
)

func pollSnmp(node NodeConfig) {
	comm := node.Community
	if comm == "" {
		comm = os.Getenv("SNMP_COMMUNITY")
	}
	sn := &gosnmp.GoSNMP{Target: node.Host, Port: 161, Community: comm, Version: gosnmp.Version2c, Timeout: 3 * time.Second}
	if err := sn.Connect(); err != nil {
		return
	}
	defer sn.Conn.Close()

	voltsOID := os.Getenv("VOLTAGE_OID")
	if voltsOID == "" {
		voltsOID = ".1.3.6.1.4.1.35160.1.16.1.13.4"
	}
	powerOID := os.Getenv("POWER_OID")

	oids := []string{voltsOID}
	if !node.NoPowerSensor && powerOID != "" {
		oids = append(oids, powerOID)
	}

	res, err := sn.Get(oids)
	if err != nil || len(res.Variables) == 0 {
		return
	}

	vRaw := int64(0)
	if res.Variables[0].Value != nil {
		vRaw = gosnmp.ToBigInt(res.Variables[0].Value).Int64()
	}
	voltage := float64(vRaw) / 10.0

	gridOnline := true
	if !node.NoPowerSensor && len(res.Variables) > 1 {
		gridOnline = gosnmp.ToBigInt(res.Variables[1].Value).Int64() == 1
	}

	soc := 0
	if node.IsLiFePO4 {
		if voltage >= 13.3 {
			soc = 100
		} else if voltage >= 13.2 {
			soc = 80
		} else if voltage >= 13.1 {
			soc = 60
		} else if voltage >= 13.0 {
			soc = 40
		} else if voltage >= 12.8 {
			soc = 20
		} else if voltage >= 12.0 {
			soc = 10
		} else {
			soc = 0
		}
	} else {
		soc = int(((voltage - 11.5) / (12.8 - 11.5)) * 100)
		if soc > 100 {
			soc = 100
		} else if soc < 0 {
			soc = 0
		}
	}

	updateNodeState(node, NodeData{
		ID: node.ID, Type: "snmp", Name: node.Name, City: node.City, Model: node.Model,
		BatterySOC: soc, TotalValue: voltage, GridOnline: gridOnline,
		Updated: time.Now().Format("15:04:05"), IsLiFePO4: node.IsLiFePO4,
		PoweredByClients: node.PoweredByClients, ClientPhone: node.ClientPhone, NoPowerSensor: node.NoPowerSensor,
	})
}

func pollDeye(node NodeConfig, token string) {
	payload := map[string][]string{"deviceList": {node.DeviceSN}}
	b, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "https://eu1-developer.deyecloud.com/v1.0/device/latest", bytes.NewBuffer(b))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	var res struct {
		DeviceDataList []struct {
			CollectionTime int64 `json:"collectionTime"`
			DataList       []struct {
				Key   string `json:"key"`
				Value string `json:"value"`
			} `json:"dataList"`
		} `json:"deviceDataList"`
	}
	json.NewDecoder(resp.Body).Decode(&res)
	if len(res.DeviceDataList) > 0 {
		d := res.DeviceDataList[0]
		data := NodeData{
			ID: node.ID, Type: "deye", Name: node.Name, City: node.City, Model: node.Model, DeviceSN: node.DeviceSN,
			Updated: time.Unix(d.CollectionTime, 0).Format("15:04:05"), IsLiFePO4: node.IsLiFePO4,
			PoweredByClients: node.PoweredByClients, ClientPhone: node.ClientPhone, NoPowerSensor: node.NoPowerSensor,
		}
		for _, item := range d.DataList {
			v, _ := strconv.ParseFloat(item.Value, 64)
			k := strings.ToUpper(item.Key)
			switch k {
			case "SOC":
				data.BatterySOC = int(v)
			case "UPSLOADPOWER":
				data.LoadPower = int(v)
			case "BATTERYPOWER":
				data.TotalValue = math.Abs(v)
			}
			if strings.Contains(k, "GRID") && strings.Contains(k, "VOLTAGE") && v > 180 {
				data.GridOnline = true
			}
			if strings.Contains(k, "GEN") && strings.Contains(k, "POWER") && v > 50 {
				data.GenOnline = true
			}
		}
		if data.NoPowerSensor {
			data.GridOnline = true
		}
		updateNodeState(node, data)
	}
}

func startPolling() {
	for {
		url := fmt.Sprintf("https://eu1-developer.deyecloud.com/v1.0/account/token?appId=%s", os.Getenv("DEYE_APPLD"))
		p := map[string]string{"appSecret": os.Getenv("DEYE_APPSECRET"), "email": os.Getenv("DEYE_LOGIN"), "password": os.Getenv("DEYE_PASS")}
		b, _ := json.Marshal(p)
		resp, err := http.Post(url, "application/json", bytes.NewBuffer(b))
		if err == nil {
			var r map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&r)
			if t, ok := r["accessToken"].(string); ok {
				tokenLock.Lock()
				deyeToken = t
				tokenLock.Unlock()
			}
			resp.Body.Close()
		}

		confLock.Lock()
		nodes := make([]NodeConfig, len(nodesConf))
		copy(nodes, nodesConf)
		confLock.Unlock()

		tokenLock.Lock()
		t := deyeToken
		tokenLock.Unlock()

		for _, n := range nodes {
			if n.Type == "deye" {
				go pollDeye(n, t)
			} else {
				go pollSnmp(n)
			}
		}
		time.Sleep(30 * time.Second)
	}
}
