// ВАЖНО: используйте `go get` чтобы убедиться, что все библиотеки установлены

package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/joho/godotenv"
)

var (
	lastMonth  string
	siteError  bool
	firstCheck = true
)

var insecureTransport = &http.Transport{
	TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
}
var insecureClient = &http.Client{Transport: insecureTransport}

type TGUpdateResponse struct {
	OK     bool     `json:"ok"`
	Result []Update `json:"result"`
}

type Update struct {
	UpdateID int64    `json:"update_id"`
	Message  *Message `json:"message,omitempty"`
}

type Message struct {
	MessageID int64  `json:"message_id"`
	Chat      *Chat  `json:"chat"`
	Text      string `json:"text"`
}

type Chat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}

func main() {
	_ = godotenv.Load()
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	chatID := os.Getenv("TELEGRAM_CHAT_ID")
	adminChatID := os.Getenv("ADMIN_CHAT_ID")

	go func() {
		for {
			checkMonth(botToken, chatID, adminChatID)
			time.Sleep(5 * time.Minute)
		}
	}()

	listenForMessages(botToken)
}

func checkMonth(botToken, chatID, adminChatID string) {
	resp, err := insecureClient.Get("https://www.atsenergo.ru/results/market/calcfacthour")
	if err != nil {
		if !siteError && adminChatID != "" {
			siteError = true
			sendTelegramMessage(botToken, adminChatID, fmt.Sprintf("Ошибка при запросе сайта: %v", err))
		}
		return
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		if !siteError && adminChatID != "" {
			siteError = true
			sendTelegramMessage(botToken, adminChatID, fmt.Sprintf("Ошибка при парсинге HTML: %v", err))
		}
		return
	}

	firstOption := doc.Find("select.form-select.periods option").First()
	if firstOption == nil {
		if !siteError && adminChatID != "" {
			siteError = true
			sendTelegramMessage(botToken, adminChatID, "Не удалось найти текущий месяц на сайте.")
		}
		return
	}

	newMonth := strings.TrimSpace(firstOption.Text())
	if newMonth == "" {
		if !siteError && adminChatID != "" {
			siteError = true
			sendTelegramMessage(botToken, adminChatID, "Не удалось определить значение первого месяца.")
		}
		return
	}

	if siteError && adminChatID != "" {
		siteError = false
		msg := fmt.Sprintf("Сайт восстановился, текущий месяц: %s", newMonth)
		if newMonth != lastMonth {
			lastMonth = newMonth
			msg = fmt.Sprintf("Сайт восстановился, текущий месяц обновился до: %s", newMonth)
		}
		sendTelegramMessage(botToken, adminChatID, msg)
	} else if newMonth != lastMonth {
		lastMonth = newMonth
		if firstCheck {
			firstCheck = false
			if adminChatID != "" {
				sendTelegramMessage(botToken, adminChatID, "Текущий месяц при старте: "+newMonth)
			}
		} else {
			if chatID != "" {
				sendTelegramMessage(botToken, chatID, "Обнаружен новый месяц: "+newMonth)
			}
		}
	}
}

func listenForMessages(botToken string) {
	var offset int64
	for {
		resp, err := getUpdates(botToken, offset)
		if err != nil {
			log.Println(err)
			time.Sleep(5 * time.Second)
			continue
		}

		for _, update := range resp.Result {
			offset = update.UpdateID + 1
			if update.Message == nil {
				continue
			}
			chatID := update.Message.Chat.ID
			sendTelegramMessage(
				botToken,
				strconv.FormatInt(chatID, 10),
				fmt.Sprintf("User ID: %d, Chat ID: %d", chatID, chatID),
			)
		}

		time.Sleep(2 * time.Second)
	}
}

func getUpdates(botToken string, offset int64) (*TGUpdateResponse, error) {
	u := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates?timeout=30", botToken)
	if offset > 0 {
		u += fmt.Sprintf("&offset=%d", offset)
	}
	resp, err := http.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var updates TGUpdateResponse
	if err := json.NewDecoder(resp.Body).Decode(&updates); err != nil {
		return nil, err
	}

	return &updates, nil
}

func sendTelegramMessage(botToken, chatID, text string) {
	msg := url.QueryEscape(text)
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage?chat_id=%s&text=%s", botToken, chatID, msg)
	_, _ = http.Get(url)
}
