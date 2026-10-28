FROM golang:1.27.3-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /server ./cmd/server \
    && mkdir /classroom-data && chown 65532:65532 /classroom-data

FROM scratch AS runtime-core
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /server /server
COPY --from=build --chown=65532:65532 /classroom-data /data
USER 65532:65532
ENV DATA_DIR=/data HTTP_ADDR=:8080
EXPOSE 8080
ENTRYPOINT ["/server"]

FROM alpine:3.22 AS runtime-pdf
RUN apk add --no-cache poppler-utils ca-certificates
COPY --from=build /server /server
COPY --from=build --chown=65532:65532 /classroom-data /data
USER 65532:65532
ENV DATA_DIR=/data HTTP_ADDR=:8080
EXPOSE 8080
ENTRYPOINT ["/server"]

FROM runtime-core AS final
