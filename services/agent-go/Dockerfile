# syntax=docker/dockerfile:1
# ── J.A.R.V.I.S. Go Agent ──
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -trimpath -o /out/server ./cmd/server

FROM alpine:3.21
RUN apk --no-cache add ca-certificates tzdata
COPY --from=build /out/server /server
COPY --from=build /src/skills /skills
EXPOSE 3002
ENTRYPOINT ["/server"]
