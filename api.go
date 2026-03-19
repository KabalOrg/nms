package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

func handleLogin(w http.ResponseWriter, r *http.Request) {
	var creds struct {
		Password string `json:"password"`
	}
	json.NewDecoder(r.Body).Decode(&creds)
	if strings.TrimSpace(creds.Password) == os.Getenv("ADMIN_PASSWORD") {
		http.SetCookie(w, &http.Cookie{Name: "auth_token", Value: ADMIN_TOKEN, Path: "/", MaxAge: 86400})
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusUnauthorized)
	}
}

func checkAuth(r *http.Request) bool {
	cookie, err := r.Cookie("auth_token")
	return err == nil && cookie.Value == ADMIN_TOKEN
}

func initAPI() {
	http.Handle("/", http.FileServer(http.Dir("./")))

	// Інфо для frontend: повертає prefix NMS
	http.HandleFunc("/api/info", func(w http.ResponseWriter, r *http.Request) {
		prefix := os.Getenv("NMS_PREFIX")
		if prefix == "" {
			prefix = "KABAL"
		}
		json.NewEncoder(w).Encode(map[string]string{"prefix": prefix})
	})

	// Поточний стан всіх вузлів та история
	http.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		state.RLock()
		json.NewEncoder(w).Encode(&state)
		state.RUnlock()
	})

	http.HandleFunc("/api/login", handleLogin)

	// CRUD для вузлів (GET / PUT / DELETE)
	http.HandleFunc("/api/admin/nodes", func(w http.ResponseWriter, r *http.Request) {
		if !checkAuth(r) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		switch r.Method {
		case http.MethodGet:
			json.NewEncoder(w).Encode(nodesConf)
		case http.MethodPut:
			var u NodeConfig
			json.NewDecoder(r.Body).Decode(&u)
			if u.ID == "" {
				u.ID = fmt.Sprintf("%d", time.Now().UnixNano())
			}
			confLock.Lock()
			found := false
			for i, n := range nodesConf {
				if n.ID == u.ID {
					nodesConf[i] = u
					found = true
					break
				}
			}
			if !found {
				nodesConf = append(nodesConf, u)
			}
			confLock.Unlock()
			d, _ := json.MarshalIndent(nodesConf, "", "  ")
			os.WriteFile("nodes.json", d, 0644)
			w.WriteHeader(http.StatusOK)
		case http.MethodDelete:
			id := r.URL.Query().Get("id")
			confLock.Lock()
			newConf := make([]NodeConfig, 0, len(nodesConf))
			for _, n := range nodesConf {
				if n.ID != id {
					newConf = append(newConf, n)
				}
			}
			nodesConf = newConf
			confLock.Unlock()
			state.Lock()
			delete(state.Nodes, id)
			state.Unlock()
			d, _ := json.MarshalIndent(nodesConf, "", "  ")
			os.WriteFile("nodes.json", d, 0644)
			w.WriteHeader(http.StatusOK)
		}
	})

	// Управління глобальними Telegram ID для повідомлень
	http.HandleFunc("/api/admin/global_notify", func(w http.ResponseWriter, r *http.Request) {
		if !checkAuth(r) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		switch r.Method {
		case http.MethodGet:
			json.NewEncoder(w).Encode(globalNotifyIDs)
		case http.MethodPost:
			confLock.Lock()
			json.NewDecoder(r.Body).Decode(&globalNotifyIDs)
			d, _ := json.MarshalIndent(globalNotifyIDs, "", "  ")
			os.WriteFile("global_notify.json", d, 0644)
			confLock.Unlock()
			w.WriteHeader(http.StatusOK)
		}
	})
}
