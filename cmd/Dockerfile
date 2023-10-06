FROM golang:1.21
WORKDIR /referral
COPY . . 

#RUN apt update
RUN go mod download && go mod verify
RUN go build -o /usr/local/bin/app ./cmd/main.go

CMD ["app"]