FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build

WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY . .
ARG TARGETOS
ARG TARGETARCH
RUN go run ./cmd/preprocess -input data/references.json.gz -output /out/data/references.bin -format kmeans-ivf-int16 -nlist 2048 -kmeans-iter 8 -kmeans-sample 20000
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

FROM alpine:3.21

RUN adduser -D -H -u 10001 app && mkdir -p /sockets && chown app:app /sockets

WORKDIR /app
COPY --from=build /out/api /app/api
COPY --from=build /out/data/ /app/data/

USER app

ENTRYPOINT ["/app/api"]
