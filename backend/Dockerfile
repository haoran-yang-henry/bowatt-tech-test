# ---- build stage ----
FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
# Statically-linked binary with no external dependencies.
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/server .

# ---- run stage ----
FROM alpine:3.20
# Root certificates are required for HTTPS calls to the LLM and search APIs.
RUN apk add --no-cache ca-certificates
COPY --from=build /out/server /server
EXPOSE 8787
ENTRYPOINT ["/server"]
