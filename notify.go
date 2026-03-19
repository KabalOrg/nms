package main

import (
	"log"
	"os"
	"os/exec"
)

// makeHardwareCall — SSH-дзвінок через Asterisk/Nagios при зникненні живлення.
// ⚠️ ФУНКЦІЯ В РОЗРОБЦІ: потребує налаштованого SSH-ключа та PBX-сервера.
func makeHardwareCall(nodeName string) {
	phone := os.Getenv("ADMIN_PHONE")
	if phone == "" {
		return
	}
	cmd := exec.Command(
		"ssh", "-T",
		"-i", "/home/kabal/nms/id_rsa",
		"root@office-pbx",
		"perl /etc/nagios/bin/notify-by-phone.pl",
		phone, nodeName,
	)
	if err := cmd.Run(); err != nil {
		log.Printf("☎️ [В РОЗРОБЦІ] Помилка SSH-дзвінка: %v", err)
	}
}
