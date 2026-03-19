package main

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
)

// nodeIcon повертає іконку стану вузла для Telegram (пріоритет: генератор > датчик > мережа)
func nodeIcon(n NodeData, nowTs int64) string {
	switch {
	case n.GenOnline:
		return "🚀"
	case n.NoPowerSensor:
		return "⚪"
	case !n.GridOnline && n.PoweredByClients:
		return "👥"
	case !n.GridOnline && n.GridOfflineSince > 0 && (nowTs-n.GridOfflineSince) > 21600:
		return "🆘"
	case !n.GridOnline:
		return "🔋"
	default:
		return "✅"
	}
}

// eventIcon повертає іконку за текстом події в логу
func eventIcon(msg string) string {
	switch {
	case strings.Contains(msg, "Світло зникло"):
		return "🔴"
	case strings.Contains(msg, "Світло є"):
		return "✅"
	case strings.Contains(msg, "ГЕНЕРАТОР ЗАПУЩЕН"):
		return "🚀"
	case strings.Contains(msg, "Генератор зупинено"):
		return "⚙️"
	case strings.Contains(msg, "Живлення від клієнтів"):
		return "👥"
	default:
		return "🔹"
	}
}

// cleanMsg прибирає іконки з початку рядка повідомлення
func cleanMsg(msg string) string {
	for _, p := range []string{"✅ ", "🔴 ", "🚀 ", "⚙️ ", "👥 "} {
		msg = strings.Replace(msg, p, "", 1)
	}
	return msg
}

func startBot() {
	if bot == nil {
		return
	}

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	keyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("📊 СТАТУС"),
			tgbotapi.NewKeyboardButton("📜 ОСТАННІ ПОДІЇ"),
		),
	)

	for update := range updates {
		if update.Message == nil {
			continue
		}
		if !allowedUsers[update.Message.From.ID] {
			continue
		}

		chatID := update.Message.Chat.ID
		text := update.Message.Text

		switch text {
		case "/reload_env":
			godotenv.Overload()
			newAllowed := make(map[int64]bool)
			for _, s := range strings.Split(os.Getenv("ALLOWED_USERS"), ",") {
				if id, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64); err == nil && id != 0 {
					newAllowed[id] = true
				}
			}
			allowedUsers = newAllowed
			bot.Send(tgbotapi.NewMessage(chatID, "✅ Конфіг .env успішно перечитано!"))

		case "📊 СТАТУС":
			state.RLock()
			cityGroups := make(map[string][]NodeData)
			off, crit := 0, 0
			nowTs := time.Now().Unix()
			for _, n := range state.Nodes {
				cityGroups[n.City] = append(cityGroups[n.City], n)
				if !n.GridOnline && !n.PoweredByClients && !n.NoPowerSensor {
					off++
					if n.GridOfflineSince > 0 && (nowTs-n.GridOfflineSince) > 21600 {
						crit++
					}
				}
			}
			state.RUnlock()

			cities := make([]string, 0, len(cityGroups))
			for c := range cityGroups {
				cities = append(cities, c)
			}
			sort.Strings(cities)

			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("📊 *СТАТУС МЕРЕЖІ*\n⚠️ Без світла: %d\n", off))
			if crit > 0 {
				sb.WriteString(fmt.Sprintf("🆘 *КРИТИЧНО (6г+): %d*\n", crit))
			}

			for _, cityName := range cities {
				sb.WriteString(fmt.Sprintf("\n📍 *%s*\n", strings.ToUpper(cityName)))
				nodes := cityGroups[cityName]
				sort.Slice(nodes, func(i, j int) bool {
					if nodes[i].Type != nodes[j].Type {
						return nodes[i].Type == "deye"
					}
					return nodes[i].Name < nodes[j].Name
				})
				for _, n := range nodes {
					icon := nodeIcon(n, nowTs)
					if n.Type == "deye" {
						sb.WriteString(fmt.Sprintf("  %s *%s*: %d%% (%dW)\n", icon, n.Name, n.BatterySOC, n.LoadPower))
					} else {
						sb.WriteString(fmt.Sprintf("  %s %s: %d%% (%.1fV)\n", icon, n.Name, n.BatterySOC, n.TotalValue))
					}
				}
			}

			m := tgbotapi.NewMessage(chatID, sb.String())
			m.ParseMode = "Markdown"
			m.ReplyMarkup = keyboard
			bot.Send(m)

		case "📜 ОСТАННІ ПОДІЇ":
			state.RLock()
			limit := 15
			if len(state.History) < limit {
				limit = len(state.History)
			}
			var sb strings.Builder
			sb.WriteString("📜 *ОСТАННІ ПОДІЇ (Live Log)*\n━━━━━━━━━━━━━━━\n")
			if limit == 0 {
				sb.WriteString("Історія поки порожня...")
			} else {
				for i := 0; i < limit; i++ {
					e := state.History[i]
					sb.WriteString(fmt.Sprintf("%s *%s %s*\n↳ %s\n\n", eventIcon(e.Msg), e.Date, e.Time, cleanMsg(e.Msg)))
				}
			}
			state.RUnlock()

			msg := tgbotapi.NewMessage(chatID, sb.String())
			msg.ParseMode = "Markdown"
			msg.ReplyMarkup = keyboard
			bot.Send(msg)

		default:
			appPrefix := os.Getenv("NMS_PREFIX")
			if appPrefix == "" {
				appPrefix = "KABAL"
			}
			msg := tgbotapi.NewMessage(chatID, fmt.Sprintf(
				"👋 Вітаю в системі моніторингу %s NMS!\nОберіть дію на клавіатурі нижче 👇",
				strings.ToUpper(appPrefix),
			))
			msg.ReplyMarkup = keyboard
			bot.Send(msg)
		}
	}
}
