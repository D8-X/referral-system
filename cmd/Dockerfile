FROM golang:1.21
WORKDIR /referral
COPY . . 

#RUN apt update
RUN go mod download && go mod verify
RUN go build -o /bin/app ./cmd/main.go

FROM debian:bookworm-slim
COPY --from=0 /bin/app /bin/app

CMD ["/bin/app"]