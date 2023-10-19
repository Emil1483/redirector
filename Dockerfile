FROM golang:1.18

WORKDIR /app
COPY go.mod .
COPY go.sum .

RUN go mod download

COPY schema.prisma .
RUN go run github.com/steebchen/prisma-client-go generate

COPY . .

RUN go build -tags netgo -ldflags '-s -w' -o app

CMD ["./app"]
