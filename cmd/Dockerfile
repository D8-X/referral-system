FROM golang:1.21
WORKDIR /referral
COPY . . 

RUN apt-get update && apt-get install -y ca-certificates
RUN go mod download && go mod verify
RUN go build -o /bin/app ./cmd/main.go

FROM debian:bookworm-slim

COPY --from=0 /bin/app /bin/app

CMD ["/bin/app"]