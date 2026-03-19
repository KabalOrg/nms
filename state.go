package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

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
	ADMIN_TOKEN     = fmt.Sprintf("session_%x", time.Now().UnixNano())
)

// sendAlert відправляє повідомлення всім отримувачам вузла + глобальним
// ВАЖЛИВО: викликати тільки всередині state.Lock()
func sendAlert(node NodeConfig, msg string) {
	if !node.AlertsEnabled || bot == nil {
		return
	}
	targets := make(map[int64]bool)
	for _, id := range node.NotifyIDs {
		targets[id] = true
	}
	confLock.Lock()
	for _, id := range globalNotifyIDs {
		targets[id] = true
	}
	confLock.Unlock()
	for id := range targets {
		bot.Send(tgbotapi.NewMessage(id, msg))
	}
}

// appendHistory додає подію в початок History і зберігає у файл
// ВАЖЛИВО: викликати тільки всередині state.Lock()
func appendHistory(msg string) {
	now := time.Now()
	state.History = append([]Event{{Date: now.Format("02.01"), Time: now.Format("15:04"), Msg: msg}}, state.History...)
	if len(state.History) > 100 {
		state.History = state.History[:100]
	}
	d, _ := json.MarshalIndent(state.History, "", "  ")
	os.WriteFile("history.json", d, 0644)
}

func updateNodeState(node NodeConfig, data NodeData) {
	state.Lock()
	defer state.Unlock()

	existing, exists := state.Nodes[node.ID]

	// Запам'ятовуємо час першого зникнення мережі
	if !data.GridOnline && !data.NoPowerSensor {
		if exists && existing.GridOfflineSince > 0 {
			data.GridOfflineSince = existing.GridOfflineSince
		} else {
			data.GridOfflineSince = time.Now().Unix()
		}
	} else {
		data.GridOfflineSince = 0
	}

	// Подія: зміна стану живлення (мережа)
	if old, ok := lastGridState[node.ID]; ok && old != data.GridOnline && !data.NoPowerSensor {
		m := "✅ " + node.Name + ": Світло є"
		if !data.GridOnline {
			if node.PoweredByClients {
				if node.ClientPhone != "" {
					m = "👥 " + node.Name + ": Живлення від клієнтів (" + node.ClientPhone + ")"
				} else {
					m = "👥 " + node.Name + ": Живлення від клієнтів (Мережа відсутня)"
				}
			} else {
				m = "🔴 " + node.Name + ": Світло зникло"
			}
		}
		appendHistory(m)
		sendAlert(node, m)
		if node.CallEnabled && !data.GridOnline && !node.PoweredByClients {
			go makeHardwareCall(node.Name)
		}
	}
	lastGridState[node.ID] = data.GridOnline

	// Подія: зміна стану генератора (тільки для Deye)
	if node.Type == "deye" {
		if old, ok := lastGenState[node.ID]; ok && old != data.GenOnline {
			m := "🚀 " + node.Name + ": ГЕНЕРАТОР ЗАПУЩЕН"
			if !data.GenOnline {
				m = "⚙️ " + node.Name + ": Генератор зупинено"
			}
			appendHistory(m)
			sendAlert(node, m)
		}
		lastGenState[node.ID] = data.GenOnline
	}

	state.Nodes[node.ID] = data
}
