# Dockerfile
FROM golang:1.24

WORKDIR /app

# Скопируем файлы go.mod и go.sum, чтобы заранее скачать зависимости
COPY go.mod go.sum ./
RUN go mod download

# Скопируем остальной код
COPY . .

# Соберём исполняемый файл
RUN go build -o atsenergobot .

# Запуск
CMD ["./atsenergobot"]
