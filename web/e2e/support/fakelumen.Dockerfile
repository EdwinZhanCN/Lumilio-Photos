# syntax=docker/dockerfile:1

FROM golang:1.25.0-trixie AS build
WORKDIR /src
COPY server/go.mod server/go.sum ./
COPY server/third_party/river/go.mod ./third_party/river/go.mod
COPY third_party/river-rivershared/go.mod ../third_party/river-rivershared/go.mod
RUN go mod download
COPY server/ ./
COPY third_party/river-rivershared ../third_party/river-rivershared
RUN CGO_ENABLED=0 go build -trimpath -o /out/fakelumen ./tools/fakelumen

FROM scratch
COPY --from=build /out/fakelumen /fakelumen
USER 65532:65532
EXPOSE 50051
ENTRYPOINT ["/fakelumen"]
