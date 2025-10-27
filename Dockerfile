FROM golang:1.25.3 as builder

WORKDIR /src

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /stackver cmd/stackver/*.go

FROM alpine:latest as app

RUN apk --no-cache add ca-certificates git

COPY --from=builder /stackver /bin/stackver

WORKDIR /stack

ENTRYPOINT ["/bin/stackver"]