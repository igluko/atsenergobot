package main

import (
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

// knownMonths хранит value всех option, которые мы уже «видели».
var knownMonths = make(map[string]bool)

// TGUpdateResponse описывает структуру ответа на метод getUpdates Telegram API.
type TGUpdateResponse struct {
	OK     bool     `json:"ok"`
	Result []Update `json:"result"`
}

// Update описывает одно «событие» (полученное сообщение или callback).
type Update struct {
	UpdateID int64    `json:"update_id"`
	Message  *Message `json:"message,omitempty"`
}

// Message – структура с полями сообщения, которые нам нужны.
type Message struct {
	MessageID int64  `json:"message_id"`
	Chat      *Chat  `json:"chat"`
	Text      string `json:"text"`
}

// Chat описывает чат/пользователя.
type Chat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}

func main() {
	// Считываем переменные окружения из .env (или системных env).
	err := godotenv.Load()
	if err != nil {
		log.Println("Не удалось найти .env файл – пробуем переменные окружения системы")
	}

	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	chatID := os.Getenv("TELEGRAM_CHAT_ID")

	if botToken == "" {
		log.Fatal("Не задан TELEGRAM_BOT_TOKEN в .env или переменных окружения!")
	}
	if chatID == "" {
		log.Println("Не задан TELEGRAM_CHAT_ID. Авто-рассылка о новых месяцах работать не будет.")
	}

	// Запускаем фоновой мониторинг каждые 5 минут.
	go func() {
		for {
			checkForNewMonths(botToken, chatID)
			time.Sleep(5 * time.Minute)
		}
	}()

	// Запускаем цикл прослушивания команд (getUpdates).
	listenForCommands(botToken)
}

// checkForNewMonths заходит на страницу, парсит <select class="form-select periods"> и проверяет,
// не появился ли новый <option>. При появлении нового месяца шлёт уведомление в Telegram.
func checkForNewMonths(botToken, chatID string) {
	if chatID == "" {
		// Если не указан chat_id, оповещать некуда — просто выходим.
		return
	}

	resp, err := http.Get("https://www.atsenergo.ru/results/market/calcfacthour")
	if err != nil {
		log.Printf("Ошибка запроса к сайту: %v\n", err)
		return
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		log.Printf("Ошибка парсинга HTML: %v\n", err)
		return
	}

	// Ищем все <option> внутри <select class="form-select periods">
	doc.Find("select.form-select.periods option").Each(func(i int, s *goquery.Selection) {
		value, exists := s.Attr("value")
		if !exists {
			return // у option нет атрибута value — пропускаем
		}

		if !knownMonths[value] {
			// Встретили новый месяц, шлём уведомление
			knownMonths[value] = true
			monthText := s.Text()
			message := fmt.Sprintf("Обнаружен новый месяц в списке: %s (value=%s)", monthText, value)
			sendTelegramMessage(botToken, chatID, message)
		}
	})
}

// listenForCommands постоянно опрашивает Telegram (getUpdates) и обрабатывает сообщения.
// Если приходит команда "/current", бот сообщает текущий (верхний) месяц.
func listenForCommands(botToken string) {
	// offset нужен, чтобы Telegram не прислал заново старые сообщения.
	var offset int64

	for {
		updates, err := getUpdates(botToken, offset)
		if err != nil {
			log.Printf("Ошибка getUpdates: %v\n", err)
			time.Sleep(5 * time.Second)
			continue
		}

		for _, update := range updates.Result {
			// update.UpdateID – int64, значит offset тоже делаем int64.
			offset = update.UpdateID + 1

			if update.Message == nil {
				continue
			}
			chatID := update.Message.Chat.ID
			text := update.Message.Text

			if text == "/current" {
				current := getCurrentMonth()
				if current == "" {
					sendTelegramMessage(botToken, strconv.FormatInt(chatID, 10),
						"Не удалось определить текущий месяц (пустой список?).")
				} else {
					sendTelegramMessage(botToken, strconv.FormatInt(chatID, 10),
						"Текущий (верхний) месяц в списке: "+current)
				}
			}
		}

		// Небольшая пауза, чтобы не нагружать Telegram.
		time.Sleep(2 * time.Second)
	}
}

// getUpdates обращается к Telegram API getUpdates для получения входящих сообщений/обновлений.
func getUpdates(botToken string, offset int64) (*TGUpdateResponse, error) {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates?timeout=30", botToken)
	if offset > 0 {
		apiURL += fmt.Sprintf("&offset=%d", offset)
	}

	resp, err := http.Get(apiURL)
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

// sendTelegramMessage посылает простое текстовое сообщение в указанный chat_id.
func sendTelegramMessage(botToken, chatID, text string) {
	encodedText := url.QueryEscape(text)
	reqURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage?chat_id=%s&text=%s",
		botToken, chatID, encodedText)

	resp, err := http.Get(reqURL)
	if err != nil {
		log.Printf("Ошибка при отправке сообщения в Telegram: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("sendMessage вернул статус %d\n", resp.StatusCode)
	}
}

// getCurrentMonth возвращает текст (Text) самого верхнего <option> из <select>.
func getCurrentMonth() string {
	resp, err := http.Get("https://www.atsenergo.ru/results/market/calcfacthour")
	if err != nil {
		log.Printf("Ошибка при получении текущего месяца: %v\n", err)
		return ""
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		log.Printf("Ошибка парсинга HTML: %v\n", err)
		return ""
	}

	// Берём первый option внутри select.form-select.periods
	firstOption := doc.Find("select.form-select.periods option").First()
	if firstOption == nil {
		return ""
	}
	return strings.TrimSpace(firstOption.Text())
}
