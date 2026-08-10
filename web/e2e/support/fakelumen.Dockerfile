# syntax=docker/dockerfile:1

FROM golang:1.25.12-trixie AS build
WORKDIR /src
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
RUN CGO_ENABLED=0 go build -trimpath -o /out/fakelumen ./tools/fakelumen

FROM scratch
COPY --from=build /out/fakelumen /fakelumen
USER 65532:65532
EXPOSE 50051
ENTRYPOINT ["/fakelumen"]
