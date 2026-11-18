# Online build of Ragnarok of Building.
#
# The game catalog is generated locally from your own RO LATAM client
# (go run ./cmd/fetchrathena && go run ./cmd/import) and copied into the
# image: it is not in the git repository.
#
#   docker build -t ragnarok-of-building .
#   docker run -p 8080:8080 ragnarok-of-building

FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/server ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/server /app/server
COPY data/gen/catalog.json.gz /app/data/gen/catalog.json.gz
ENV PORT=8080
EXPOSE 8080
USER nonroot
ENTRYPOINT ["/app/server"]
