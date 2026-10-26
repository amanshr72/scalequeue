FROM golang:1.27.3-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /worker ./cmd/worker
FROM alpine:3.22
RUN adduser -D worker
USER worker
COPY --from=build /worker /worker
EXPOSE 9090
ENTRYPOINT ["/worker"]
