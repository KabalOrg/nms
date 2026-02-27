package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/gosnmp/gosnmp"
	"github.com/joho/godotenv"
)

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

var (
	state           = GlobalState{Nodes: make(map[string]NodeData), History: make([]Event, 0)}
	nodesConf       []NodeConfig
	globalNotifyIDs []int64
	allowedUsers    = make(map[int64]bool)
	lastGridState   = make(map[string]bool)
	lastGenState    = make(map[string]bool)
	confLock        sync.Mutex
	bot             *tgbotapi.BotAPI
	deyeToken       string
	tokenLock       sync.Mutex
	ADMIN_TOKEN     = fmt.Sprintf("session_%x", time.Now().UnixNano()) // Унікальний токен при кожному старті
)

func makeHardwareCall(nodeName string) {
	phone := os.Getenv("ADMIN_PHONE")
	if phone == "" { return }
	cmd := exec.Command("ssh", "-T", "-i", "/home/kabal/nms/id_rsa", "root@office-pbx", "perl /etc/nagios/bin/notify-by-phone.pl", phone, nodeName)
	err := cmd.Run()
	if err != nil { log.Printf("☎️ Помилка SSH-дзвінка: %v", err) }
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	var creds struct{ Password string `json:"password"` }
	json.NewDecoder(r.Body).Decode(&creds)
	if strings.TrimSpace(creds.Password) == os.Getenv("ADMIN_PASSWORD") {
		http.SetCookie(w, &http.Cookie{Name: "auth_token", Value: ADMIN_TOKEN, Path: "/", MaxAge: 86400})
		w.WriteHeader(200)
	} else { w.WriteHeader(401) }
}

func checkAuth(r *http.Request) bool {
	cookie, err := r.Cookie("auth_token")
	return err == nil && cookie.Value == ADMIN_TOKEN
}

func pollSnmp(node NodeConfig) {
	comm := node.Community
	if comm == "" { comm = os.Getenv("SNMP_COMMUNITY") }
	sn := &gosnmp.GoSNMP{ Target: node.Host, Port: 161, Community: comm, Version: gosnmp.Version2c, Timeout: 3 * time.Second }
	if err := sn.Connect(); err != nil { return }
	defer sn.Conn.Close()

	voltsOID := os.Getenv("VOLTAGE_OID")
	if voltsOID == "" { voltsOID = ".1.3.6.1.4.1.35160.1.16.1.13.4" }
	powerOID := os.Getenv("POWER_OID")

	oids := []string{voltsOID}
	if !node.NoPowerSensor && powerOID != "" { oids = append(oids, powerOID) }

	res, err := sn.Get(oids)
	if err != nil || len(res.Variables) == 0 { return }

	vRaw := int64(0)
	if res.Variables[0].Value != nil { vRaw = gosnmp.ToBigInt(res.Variables[0].Value).Int64() }
	voltage := float64(vRaw) / 10.0

	gridOnline := true
	if !node.NoPowerSensor && len(res.Variables) > 1 { gridOnline = gosnmp.ToBigInt(res.Variables[1].Value).Int64() == 1 }

	soc := 0
	if node.IsLiFePO4 {
		if voltage >= 13.3 { soc = 100 } else if voltage >= 13.2 { soc = 80 } else if voltage >= 13.1 { soc = 60 } else if voltage >= 13.0 { soc = 40 } else if voltage >= 12.8 { soc = 20 } else if voltage >= 12.0 { soc = 10 } else { soc = 0 }
	} else {
		soc = int(((voltage - 11.5) / (12.8 - 11.5)) * 100)
		if soc > 100 { soc = 100 } else if soc < 0 { soc = 0 }
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
	if err != nil { return }
	defer resp.Body.Close()

	var res struct {
		DeviceDataList []struct {
			CollectionTime int64 `json:"collectionTime"`
			DataList []struct { Key string `json:"key"`; Value string `json:"value"` } `json:"dataList"`
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
			v, _ := strconv.ParseFloat(item.Value, 64); k := strings.ToUpper(item.Key)
			switch k {
			case "SOC": data.BatterySOC = int(v)
			case "UPSLOADPOWER": data.LoadPower = int(v)
			case "BATTERYPOWER": data.TotalValue = math.Abs(v)
			}
			if strings.Contains(k, "GRID") && strings.Contains(k, "VOLTAGE") && v > 180 { data.GridOnline = true }
			if strings.Contains(k, "GEN") && strings.Contains(k, "POWER") && v > 50 { data.GenOnline = true }
		}
		if data.NoPowerSensor { data.GridOnline = true }
		updateNodeState(node, data)
	}
}

func updateNodeState(node NodeConfig, data NodeData) {
	state.Lock()
	defer state.Unlock()
	existing, exists := state.Nodes[node.ID]
	
	if !data.GridOnline && !data.NoPowerSensor {
		if exists && existing.GridOfflineSince > 0 { data.GridOfflineSince = existing.GridOfflineSince } else { data.GridOfflineSince = time.Now().Unix() }
	} else { data.GridOfflineSince = 0 }

	if old, ok := lastGridState[node.ID]; ok && old != data.GridOnline && !data.NoPowerSensor {
		m := "✅ " + node.Name + ": Світло є"
		if !data.GridOnline {
			if node.PoweredByClients {
				if node.ClientPhone != "" { m = "👥 " + node.Name + ": Живлення від клієнтів (" + node.ClientPhone + ")" } else { m = "👥 " + node.Name + ": Живлення від клієнтів (Мережа відсутня)" }
			} else { m = "🔴 " + node.Name + ": Світло зникло" }
		}
		now := time.Now(); state.History = append([]Event{{Date: now.Format("02.01"), Time: now.Format("15:04"), Msg: m}}, state.History...)
		if len(state.History) > 100 { state.History = state.History[:100] }
		d, _ := json.MarshalIndent(state.History, "", "  "); os.WriteFile("history.json", d, 0644)

		if node.AlertsEnabled {
			targets := make(map[int64]bool); for _, id := range node.NotifyIDs { targets[id] = true }
			confLock.Lock(); for _, id := range globalNotifyIDs { targets[id] = true }; confLock.Unlock()
			for id := range targets { bot.Send(tgbotapi.NewMessage(id, m)) }
			if node.CallEnabled && !data.GridOnline && !node.PoweredByClients { go makeHardwareCall(node.Name) }
		}
	}
	lastGridState[node.ID] = data.GridOnline
	
	if node.Type == "deye" {
		if old, ok := lastGenState[node.ID]; ok && old != data.GenOnline {
			m := "🚀 " + node.Name + ": ГЕНЕРАТОР ЗАПУЩЕН"; if !data.GenOnline { m = "⚙️ " + node.Name + ": Генератор зупинено" }
			now := time.Now(); state.History = append([]Event{{Date: now.Format("02.01"), Time: now.Format("15:04"), Msg: m}}, state.History...)
			if len(state.History) > 100 { state.History = state.History[:100] }
			d, _ := json.MarshalIndent(state.History, "", "  "); os.WriteFile("history.json", d, 0644)

			if node.AlertsEnabled {
				targets := make(map[int64]bool); for _, id := range node.NotifyIDs { targets[id] = true }
				confLock.Lock(); for _, id := range globalNotifyIDs { targets[id] = true }; confLock.Unlock()
				for id := range targets { bot.Send(tgbotapi.NewMessage(id, m)) }
			}
		}
		lastGenState[node.ID] = data.GenOnline
	}
	state.Nodes[node.ID] = data
}

func main() {
	godotenv.Load()
	fN, _ := os.ReadFile("nodes.json"); json.Unmarshal(fN, &nodesConf)
	fG, _ := os.ReadFile("global_notify.json"); json.Unmarshal(fG, &globalNotifyIDs)
	fH, err := os.ReadFile("history.json"); if err == nil { json.Unmarshal(fH, &state.History) }
	
	raw := os.Getenv("ALLOWED_USERS")
	for _, s := range strings.Split(raw, ",") {
		id, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64); if id != 0 { allowedUsers[id] = true }
	}
	
	bot, _ = tgbotapi.NewBotAPI(os.Getenv("BOT_TOKEN"))

	http.Handle("/", http.FileServer(http.Dir("./")))
	
	// НОВЕ API ДЛЯ FRONTEND (Отримання префіксу)
	http.HandleFunc("/api/info", func(w http.ResponseWriter, r *http.Request) {
		prefix := os.Getenv("NMS_PREFIX")
		if prefix == "" { prefix = "KABAL" }
		json.NewEncoder(w).Encode(map[string]string{"prefix": prefix})
	})

	http.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		state.RLock(); json.NewEncoder(w).Encode(state); state.RUnlock()
	})
	http.HandleFunc("/api/login", handleLogin)

	http.HandleFunc("/api/admin/nodes", func(w http.ResponseWriter, r *http.Request) {
		if !checkAuth(r) { w.WriteHeader(403); return }
		if r.Method == "GET" { json.NewEncoder(w).Encode(nodesConf) }
		if r.Method == "PUT" {
			var u NodeConfig; json.NewDecoder(r.Body).Decode(&u)
			if u.ID == "" { u.ID = fmt.Sprintf("%d", time.Now().UnixNano()) }
			confLock.Lock()
			found := false
			for i, n := range nodesConf { if n.ID == u.ID { nodesConf[i] = u; found = true; break } }
			if !found { nodesConf = append(nodesConf, u) }
			confLock.Unlock()
			d, _ := json.MarshalIndent(nodesConf, "", "  "); os.WriteFile("nodes.json", d, 0644)
			w.WriteHeader(200)
		}
		if r.Method == "DELETE" {
			id := r.URL.Query().Get("id")
			confLock.Lock()
			newConf := []NodeConfig{}
			for _, n := range nodesConf { if n.ID != id { newConf = append(newConf, n) } }
			nodesConf = newConf
			confLock.Unlock()
			
			state.Lock()
			delete(state.Nodes, id)
			state.Unlock()
			
			d, _ := json.MarshalIndent(nodesConf, "", "  "); os.WriteFile("nodes.json", d, 0644)
			w.WriteHeader(200)
		}
	})

	http.HandleFunc("/api/admin/global_notify", func(w http.ResponseWriter, r *http.Request) {
		if !checkAuth(r) { w.WriteHeader(403); return }
		if r.Method == "GET" { json.NewEncoder(w).Encode(globalNotifyIDs) }
		if r.Method == "POST" {
			confLock.Lock(); json.NewDecoder(r.Body).Decode(&globalNotifyIDs)
			d, _ := json.MarshalIndent(globalNotifyIDs, "", "  "); os.WriteFile("global_notify.json", d, 0644); confLock.Unlock()
			w.WriteHeader(200)
		}
	})

	go func() {
		for {
			url := fmt.Sprintf("https://eu1-developer.deyecloud.com/v1.0/account/token?appId=%s", os.Getenv("DEYE_APPLD"))
			p := map[string]string{"appSecret": os.Getenv("DEYE_APPSECRET"), "email": os.Getenv("DEYE_LOGIN"), "password": os.Getenv("DEYE_PASS")}
			b, _ := json.Marshal(p); resp, err := http.Post(url, "application/json", bytes.NewBuffer(b))
			if err == nil {
				var r map[string]interface{}; json.NewDecoder(resp.Body).Decode(&r)
				if t, ok := r["accessToken"].(string); ok { deyeToken = t }
				resp.Body.Close()
			}
			confLock.Lock(); nodes := make([]NodeConfig, len(nodesConf)); copy(nodes, nodesConf); confLock.Unlock()
			for _, n := range nodes { if n.Type == "deye" { go pollDeye(n, deyeToken) } else { go pollSnmp(n) } }
			time.Sleep(30 * time.Second)
		}
	}()

	go func() {
		u := tgbotapi.NewUpdate(0)
		updates := bot.GetUpdatesChan(u)
		
		keyboard := tgbotapi.NewReplyKeyboard(
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("📊 СТАТУС"),
				tgbotapi.NewKeyboardButton("📜 ОСТАННІ ПОДІЇ"),
			),
		)

		for update := range updates {
			if update.Message == nil { continue }
			if !allowedUsers[update.Message.From.ID] { continue }

			text := update.Message.Text
			
			if text == "/reload_env" {
				godotenv.Overload() 
				raw := os.Getenv("ALLOWED_USERS")
				newAllowed := make(map[int64]bool)
				for _, s := range strings.Split(raw, ",") {
					id, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
					if id != 0 { newAllowed[id] = true }
				}
				allowedUsers = newAllowed 
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "✅ Конфіг .env успішно перечитано!\nНових користувачів додано без перезавантаження сервера.")
				bot.Send(msg)
				continue
			}

			if text == "/start" || (text != "📊 СТАТУС" && text != "📜 ОСТАННІ ПОДІЇ") {
				appPrefix := os.Getenv("NMS_PREFIX")
				if appPrefix == "" { appPrefix = "KABAL" }
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("👋 Вітаю в системі моніторингу %s NMS!\nОберіть дію на клавіатурі нижче 👇", strings.ToUpper(appPrefix)))
				msg.ReplyMarkup = keyboard
				bot.Send(msg)
				continue
			}

			if text == "📊 СТАТУС" {
				state.RLock()
				cityGroups := make(map[string][]NodeData)
				off, crit := 0, 0; nowTs := time.Now().Unix()
				for _, n := range state.Nodes {
					cityGroups[n.City] = append(cityGroups[n.City], n)
					if !n.GridOnline && !n.PoweredByClients && !n.NoPowerSensor { 
						off++; if n.GridOfflineSince > 0 && (nowTs-n.GridOfflineSince) > 21600 { crit++ } 
					}
				}
				state.RUnlock()
				
				var cities []string; for c := range cityGroups { cities = append(cities, c) }; sort.Strings(cities)
				var sb strings.Builder
				sb.WriteString(fmt.Sprintf("📊 *СТАТУС МЕРЕЖІ*\n⚠️ Без світла: %d\n", off))
				if crit > 0 { sb.WriteString(fmt.Sprintf("🆘 *КРИТИЧНО (6г+): %d*\n", crit)) }
				
				for _, cityName := range cities {
					sb.WriteString(fmt.Sprintf("\n📍 *%s*\n", strings.ToUpper(cityName)))
					nodes := cityGroups[cityName]
					sort.Slice(nodes, func(i, j int) bool { if nodes[i].Type != nodes[j].Type { return nodes[i].Type == "deye" }; return nodes[i].Name < nodes[j].Name })
					
					for _, n := range nodes {
						icon := "✅"
						if n.NoPowerSensor {
							icon = "⚪"
						} else if n.GenOnline { 
							icon = "🚀" 
						} else if !n.GridOnline { 
							if n.PoweredByClients { icon = "👥" } else {
								icon = "🔋"; if n.GridOfflineSince > 0 && (nowTs-n.GridOfflineSince) > 21600 { icon = "🆘" }
							}
						}
						if n.Type == "deye" { sb.WriteString(fmt.Sprintf("  %s *%s*: %d%% (%dW)\n", icon, n.Name, n.BatterySOC, n.LoadPower)) } else { sb.WriteString(fmt.Sprintf("  %s %s: %d%% (%.1fV)\n", icon, n.Name, n.BatterySOC, n.TotalValue)) }
					}
				}
				m := tgbotapi.NewMessage(update.Message.Chat.ID, sb.String())
				m.ParseMode = "Markdown"
				m.ReplyMarkup = keyboard
				bot.Send(m)

			} else if text == "📜 ОСТАННІ ПОДІЇ" {
				state.RLock()
				limit := 15
				if len(state.History) < limit { limit = len(state.History) }

				var sb strings.Builder
				sb.WriteString("📜 *ОСТАННІ ПОДІЇ (Live Log)*\n━━━━━━━━━━━━━━━\n")

				if limit == 0 {
					sb.WriteString("Історія поки порожня...")
				} else {
					for i := 0; i < limit; i++ {
						e := state.History[i]
						icon := "🔹"
						if strings.Contains(e.Msg, "Світло зникло") { icon = "🔴" } else if strings.Contains(e.Msg, "Світло є") { icon = "✅" } else if strings.Contains(e.Msg, "ГЕНЕРАТОР ЗАПУЩЕН") { icon = "🚀" } else if strings.Contains(e.Msg, "Генератор зупинено") { icon = "⚙️" } else if strings.Contains(e.Msg, "Живлення від клієнтів") { icon = "👥" }
						
						cleanMsg := e.Msg
						prefixes := []string{"✅ ", "🔴 ", "🚀 ", "⚙️ ", "👥 "}
						for _, p := range prefixes { cleanMsg = strings.Replace(cleanMsg, p, "", 1) }

						sb.WriteString(fmt.Sprintf("%s *%s %s*\n↳ %s\n\n", icon, e.Date, e.Time, cleanMsg))
					}
				}
				state.RUnlock()

				msg := tgbotapi.NewMessage(update.Message.Chat.ID, sb.String())
				msg.ParseMode = "Markdown"
				msg.ReplyMarkup = keyboard
				bot.Send(msg)
			}
		}
	}()

	appPrefix := os.Getenv("NMS_PREFIX")
	if appPrefix == "" { appPrefix = "KABAL" }
	log.Printf("🚀 %s NMS v1.0 Active (Port 8085).", strings.ToUpper(appPrefix))
	http.ListenAndServe(":8085", nil)
}
