# ATS Energo Telegram Bot

Этот бот каждые 5 минут заходит на страницу [https://www.atsenergo.ru/results/market/calcfacthour](https://www.atsenergo.ru/results/market/calcfacthour) и:

1. Получает самый верхний (последний доступный) месяц в выпадающем списке.
2. Отправляет сообщение в рабочий Telegram-чат, если обнаружен новый месяц по сравнению с предыдущим сохранённым.
3. Если сайт недоступен или возникает ошибка:
   - бот **однократно** уведомляет **администратора**;
   - больше не тревожит, пока сайт снова не заработает;
   - после восстановления сообщает об этом **только администратору**.
4. При первом запуске бот определяет текущий месяц и отправляет его **только администратору**.
5. Если пользователю написать боту в Telegram, он в ответ отправит ID пользователя и чата.

## Содержимое проекта

- `main.go` – исходный код бота на Go.
- `Dockerfile` – инструкции сборки контейнера с Go 1.24.
- `docker-compose.yml` – настройки Docker Compose, в том числе проверка обязательных переменных окружения.

## Как настроить

1. **Создайте файл `.env`** (в той же директории, где лежат `Dockerfile` и `docker-compose.yml`).  
   Укажите в нём значения для следующих переменных:
   ```env
   TELEGRAM_BOT_TOKEN=1234567:ABCDEF...
   TELEGRAM_CHAT_ID=-1001234567890      # рабочий групповой чат (получает только смену месяца)
   ADMIN_CHAT_ID=1122334455             # ваш личный ID (получает ошибки и стартовое сообщение)
