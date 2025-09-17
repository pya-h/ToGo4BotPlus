FROM golang:1.22

WORKDIR /app

COPY go.mod go.sum .env togos.db ./
COPY Togo/ ./Togo/

RUN go mod download

COPY . .

RUN go build -o tg .

# EXPOSE 8080

# update
CMD ["./tg"]
