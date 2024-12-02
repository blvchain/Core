FROM golang:1.23.3-bookworm

WORKDIR /usr/src/app/blvchain/core_v1

COPY go.mod go.sum ./

RUN go mod download

COPY *.go ./

RUN CGO_ENABLED=0 GOOS=linux go build -o /blvchain-core_v1

EXPOSE 50051

CMD ["/blvchain-core"]