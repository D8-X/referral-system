FROM golang:1.22
ARG BUILD_VERSION
WORKDIR /referral
COPY . . 

RUN go mod download && go mod verify
RUN go build -ldflags "-X 'main.VERSION=${BUILD_VERSION}'" -o /bin/app ./cmd/main.go

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y ca-certificates
COPY --from=0 /bin/app /bin/app

CMD ["/bin/app"]