package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
)

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

	initAPI()

	go startPolling()
	go startBot()

	appPrefix := os.Getenv("NMS_PREFIX")
	if appPrefix == "" { appPrefix = "KABAL" }
	log.Printf("🚀 %s NMS v1.0 Active (Port 8085).", strings.ToUpper(appPrefix))
	log.Fatal(http.ListenAndServe(":8085", nil))
}
