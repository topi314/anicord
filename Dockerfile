FROM --platform=$BUILDPLATFORM golang:1.21-alpine AS build

WORKDIR /build

COPY go.mod go.sum ./

RUN go mod download

COPY . .

ARG TARGETOS
ARG TARGETARCH

RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    CGO_ENABLED=0 \
    GOOS=$TARGETOS \
    GOARCH=$TARGETARCH \
    go build -o anicord-bot github.com/topi314/anicord

FROM alpine

COPY --from=build /build/anicord-bot /bin/anicord

ENTRYPOINT ["/bin/anicord"]

CMD ["-config", "/var/lib/anicord.yml"]
