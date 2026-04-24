FROM golang:1.24 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/codex-pool .

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/codex-pool /usr/local/bin/codex-pool
EXPOSE 8989
ENTRYPOINT ["/usr/local/bin/codex-pool"]
