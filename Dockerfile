#
# lobster-world-core Dockerfile (v0.2)
#
# Build:  docker build -t lobster-world-core .
# Run:    docker run --rm -p 8080:8080 -e PORT=8080 lobster-world-core
#

FROM golang:1.22 AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o /out/server ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot

ENV PORT=8080
EXPOSE 8080

WORKDIR /
COPY --from=builder /out/server /server
USER nonroot:nonroot
CMD ["/server"]

